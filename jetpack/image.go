package jetpack

// FIXME: refactor references to Host.imagesDS

import "bytes"
import "crypto/sha512"
import "encoding/json"
import "fmt"
import "io"
import "io/ioutil"
import "os"
import "path"
import "path/filepath"
import "sort"
import "strings"
import "time"

import "code.google.com/p/go-uuid/uuid"
import "github.com/appc/spec/schema"
import "github.com/appc/spec/schema/types"
import "github.com/juju/errors"

import "../run"
import "../zfs"

const imageSnapshotName = "seal"

type Image struct {
	Dataset  *zfs.Dataset         `json:"-"`
	Host     *Host                `json:"-"`
	Manifest schema.ImageManifest `json:"-"`

	UUID uuid.UUID `json:"-"`

	Hash      *types.Hash `json:",omitempty"`
	Origin    string
	Timestamp time.Time
}

func NewImage(ds *zfs.Dataset, h *Host) *Image {
	basename := path.Base(ds.Name)
	img := &Image{
		Dataset:  ds,
		Host:     h,
		UUID:     uuid.Parse(basename),
		Manifest: *schema.BlankImageManifest(),
	}
	if img.UUID == nil {
		panic(fmt.Sprintf("Invalid UUID: %#v", basename))
	}
	return img
}

func (img *Image) IsEmpty() bool {
	_, err := os.Stat(img.Dataset.Path("manifest"))
	return os.IsNotExist(err)
}

func (img *Image) Load() error {
	if img.IsEmpty() {
		return errors.New("Image is empty")
	}

	metadataJSON, err := ioutil.ReadFile(img.Dataset.Path("metadata"))
	if err != nil {
		return errors.Trace(err)
	}

	if err = json.Unmarshal(metadataJSON, img); err != nil {
		return errors.Trace(err)
	}

	if err := img.loadManifest(); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (img *Image) Seal() error {
	// Make sure that manifest exists and validates correctly
	if err := img.loadManifest(); err != nil {
		return errors.Trace(err)
	}

	if img.Hash == nil {
		amiPath := ""
		if img.Host.Properties.GetBool("images.ami.store", false) {
			amiPath = img.Dataset.Path("ami")
		}

		if hash, err := img.SaveAMI(amiPath, 0400); err != nil {
			return errors.Trace(err)
		} else {
			img.Hash = hash
		}
	}

	// Serialize metadata
	if metadataJSON, err := json.Marshal(img); err != nil {
		return errors.Trace(err)
	} else {
		if err := ioutil.WriteFile(img.Dataset.Path("metadata"), metadataJSON, 0400); err != nil {
			return errors.Trace(err)
		}
	}

	if err := os.Symlink(path.Base(img.Dataset.Name), img.Host.imagesDS.Path(img.Hash.String())); err != nil {
		return errors.Trace(err)
	}

	if _, err := img.Dataset.Snapshot(imageSnapshotName); err != nil {
		return errors.Trace(err)
	}

	if err := img.Dataset.Zfs("set", "readonly=on"); err != nil {
		return errors.Trace(err)
	}

	return nil
}

// Save image to an AMI file, return its hash.
//
// If path is an empty string, don't save the image, just return the
// hash. If path is "-", print image to stdout.
func (img *Image) SaveAMI(path string, perm os.FileMode) (*types.Hash, error) {
	archiver := run.Command("tar", "-C", img.Dataset.Mountpoint, "-c", "-f", "-", "manifest", "rootfs")
	if archive, err := archiver.StdoutPipe(); err != nil {
		return nil, errors.Trace(err)
	} else {
		hash := sha512.New()
		faucet := io.TeeReader(archive, hash)
		sink := ioutil.Discard
		var compressor *run.Cmd = nil

		if path != "" {
			if path == "-" {
				sink = os.Stdout
			} else {
				if f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, perm); err != nil {
					return nil, errors.Trace(err)
				} else {
					defer f.Close()
					sink = f
				}
			}

			if compression := img.Host.Properties.GetString("images.ami.compression", "no"); compression != "none" {
				switch compression {
				case "xz":
					compressor = run.Command("xz", "-z", "-c")
				case "bzip2":
					compressor = run.Command("bzip2", "-z", "-c")
				case "gz":
				case "gzip":
					compressor = run.Command("gzip", "-c")
				default:
					return nil, errors.Errorf("Invalid setting images.ami.compression=%#v (allowed values: xz, bzip2, gzip, none)", compression)
				}

				compressor.Cmd.Stdout = sink
				if cin, err := compressor.StdinPipe(); err != nil {
					return nil, errors.Trace(err)
				} else {
					sink = cin
				}
			}
		}

		if err := archiver.Start(); err != nil {
			return nil, errors.Trace(err)
		}

		if compressor != nil {
			if err := compressor.Start(); err != nil {
				archiver.Cmd.Process.Kill()
				return nil, errors.Trace(err)
			}
		}

		if _, err := io.Copy(sink, faucet); err != nil {
			return nil, errors.Trace(err)
		}

		if hash, err := types.NewHash(fmt.Sprintf("sha512-%x", hash.Sum(nil))); err != nil {
			// CAN'T HAPPEN, srsly
			return nil, errors.Trace(err)
		} else {
			return hash, nil
		}
	}
}

func (img *Image) loadManifest() error {
	manifestJSON, err := ioutil.ReadFile(img.Dataset.Path("manifest"))
	if err != nil {
		return errors.Trace(err)
	}

	if err = json.Unmarshal(manifestJSON, &img.Manifest); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (img *Image) Containers() (children ContainerSlice, _ error) {
	if containers, err := img.Host.Containers(); err != nil {
		return nil, errors.Trace(err)
	} else {
		snap := img.Dataset.SnapshotName(imageSnapshotName)
		for _, container := range containers {
			if container.Dataset.Origin == snap {
				children = append(children, container)
			}
		}
		return
	}
}

func (img *Image) Destroy() (err error) {
	err = errors.Trace(img.Dataset.Destroy("-r"))
	if img.Hash != nil {
		if err2 := os.Remove(img.Host.imagesDS.Path(img.Hash.String())); err2 != nil && err == nil {
			err = errors.Trace(err2)
		}
	}
	return
}

func (img *Image) Clone(dest string) (*zfs.Dataset, error) {
	snap, err := img.Dataset.GetSnapshot(imageSnapshotName)
	if err != nil {
		return nil, errors.Trace(err)
	}

	ds, err := snap.Clone(dest)
	if err != nil {
		return nil, errors.Trace(err)
	}

	// FIXME: maybe not? (hint: multi-app containers)
	for _, filename := range []string{"manifest", "metadata"} {
		if err := os.Remove(ds.Path(filename)); err != nil && !os.IsNotExist(err) {
			return nil, errors.Trace(err)
		}
	}
	return ds, nil
}

func (img *Image) RuntimeApp() schema.RuntimeApp {
	app := schema.RuntimeApp{Name: img.Manifest.Name}
	app.Annotations.Set("jetpack/image-uuid", img.UUID.String())
	if img.Hash != nil {
		app.ImageID = *img.Hash
	} else {
		// TODO: do we really need to store ACI tarballs to have an image ID on built images?
		app.ImageID.Set(fmt.Sprintf(
			"sha512-000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000%032x",
			[]byte(img.UUID),
		))
	}
	return app
}

func (img *Image) GetApp() *types.App {
	if img.Manifest.App != nil {
		return img.Manifest.App
	} else {
		return ConsoleApp("root")
	}
}

func (img *Image) Build(buildDir string, addFiles []string, buildExec []string) (*Image, error) {
	var buildContainer *Container
	if c, err := img.Host.CloneContainer(img); err != nil {
		return nil, errors.Trace(err)
	} else {
		buildContainer = c
	}

	// This is needed by freebsd-update at least, should be okay to
	// allow this in builders.
	buildContainer.Manifest.Annotations.Set("jetpack/jail.conf/allow.chflags", "true")

	destroot := buildContainer.Dataset.Path("rootfs")

	workDir, err := ioutil.TempDir(destroot, ".jetpack.build.")
	if err != nil {
		return nil, errors.Trace(err)
	}

	if buildDir[len(buildDir)-1] != '/' {
		buildDir += "/"
	}

	cpArgs := []string{"-R", buildDir}
	if addFiles != nil {
		cpArgs = append(cpArgs, addFiles...)
	}
	cpArgs = append(cpArgs, workDir)

	if err := run.Command("cp", cpArgs...).Run(); err != nil {
		return nil, errors.Trace(err)
	}

	cWorkDir := filepath.Base(workDir)
	buildApp := &types.App{
		Exec: append([]string{
			"/bin/sh", "-c",
			fmt.Sprintf("cd '%s' && exec \"${@}\"", cWorkDir),
			"jetpack-build@" + cWorkDir,
		}, buildExec...),
	}

	if err := buildContainer.Run(buildApp); err != nil {
		return nil, errors.Trace(err)
	}

	if err := os.Rename(
		filepath.Join(workDir, "manifest.json"),
		buildContainer.Dataset.Path("build.manifest"),
	); err != nil {
		return nil, errors.Trace(err)
	}

	if err := os.RemoveAll(workDir); err != nil {
		return nil, errors.Trace(err)
	}

	if err := os.Remove(filepath.Join(destroot, "/etc/resolv.conf")); err != nil {
		return nil, errors.Trace(err)
	}

	// Pivot container into an image
	uuid := path.Base(buildContainer.Dataset.Name)
	if err := buildContainer.Dataset.Rename(img.Host.imagesDS.ChildName(uuid)); err != nil {
		return nil, errors.Trace(err)
	}

	ds, err := img.Host.imagesDS.GetDataset(uuid)
	if err != nil {
		return nil, errors.Trace(err)
	}

	// Clean up container's runtime stuff, leave only rootfs and new manifest
	if entries, err := ioutil.ReadDir(ds.Mountpoint); err != nil {
		return nil, errors.Trace(err)
	} else {
		for _, entry := range entries {
			filename := entry.Name()
			if filename == "rootfs" || filename == "build.manifest" {
				continue
			}
			if err := os.RemoveAll(ds.Path(filename)); err != nil {
				return nil, errors.Trace(err)
			}
		}
	}

	childImage := NewImage(ds, img.Host)

	// Construct the child image's manifest

	if manifestBytes, err := ioutil.ReadFile(childImage.Dataset.Path("build.manifest")); err != nil {
		return nil, errors.Trace(err)
	} else {
		if err := json.Unmarshal(manifestBytes, &childImage.Manifest); err != nil {
			return nil, errors.Trace(err)
		}
	}

	if _, ok := childImage.Manifest.Annotations.Get("timestamp"); !ok {
		if ts, err := time.Now().MarshalText(); err != nil {
			return nil, errors.Trace(err)
		} else {
			childImage.Manifest.Annotations.Set("timestamp", string(ts))
		}
	}

	for _, label := range []string{"os", "arch"} {
		if childValue, ok := childImage.Manifest.GetLabel(label); !ok {
			// if child has no os/arch, copy from parent
			if parentValue, ok := img.Manifest.GetLabel(label); ok {
				childImage.Manifest.Labels = append(childImage.Manifest.Labels,
					types.Label{Name: types.ACName(label), Value: parentValue})
			}
		} else if childValue == "" {
			// if child explicitly set to nil or empty string, remove the
			// label
			for i, l := range childImage.Manifest.Labels {
				if string(l.Name) == label {
					childImage.Manifest.Labels = append(
						childImage.Manifest.Labels[:i],
						childImage.Manifest.Labels[i+1:]...)
					break
				}
			}
		}
	}

	// TODO: should we merge app from parent image?

	childImage.Origin = img.UUID.String()

	if manifestBytes, err := json.Marshal(childImage.Manifest); err != nil {
		return nil, errors.Trace(err)
	} else {
		if err := ioutil.WriteFile(childImage.Dataset.Path("manifest"), manifestBytes, 0400); err != nil {
			return nil, errors.Trace(err)
		}
	}

	if err := os.Remove(childImage.Dataset.Path("build.manifest")); err != nil {
		return nil, errors.Trace(err)
	}

	childImage.Timestamp = time.Now()

	if err := childImage.Seal(); err != nil {
		return nil, errors.Trace(err)
	}

	return childImage, nil
}

// For sorting
type ImageSlice []*Image

func (imgs ImageSlice) Len() int           { return len(imgs) }
func (imgs ImageSlice) Less(i, j int) bool { return bytes.Compare(imgs[i].UUID, imgs[j].UUID) < 0 }
func (imgs ImageSlice) Swap(i, j int)      { imgs[i], imgs[j] = imgs[j], imgs[i] }

func (imgs ImageSlice) Table() [][]string {
	rows := make([][]string, len(imgs)+1)
	rows[0] = []string{"UUID", "NAME", "LABELS"}
	for i, img := range imgs {
		labels := make([]string, len(img.Manifest.Labels))
		for j, l := range img.Manifest.Labels {
			labels[j] = fmt.Sprintf("%v=%#v", l.Name, l.Value)
		}
		sort.Strings(labels)
		rows[i+1] = append([]string{img.UUID.String(), string(img.Manifest.Name)}, strings.Join(labels, ","))
	}
	return rows
}
