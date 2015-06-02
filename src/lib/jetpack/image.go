package jetpack

import "crypto/sha512"
import "encoding/json"
import "fmt"
import "io"
import "io/ioutil"
import "os"
import "path"
import "path/filepath"
import "time"

import "code.google.com/p/go-uuid/uuid"
import "github.com/appc/spec/schema"
import "github.com/appc/spec/schema/types"
import "github.com/juju/errors"

import "lib/run"
import "lib/zfs"

const imageSnapshotName = "seal"

type Image struct {
	UUID     uuid.UUID            `json:"-"`
	Host     *Host                `json:"-"`
	Manifest schema.ImageManifest `json:"-"`

	Hash      *types.Hash `json:",omitempty"`
	Timestamp time.Time

	rootfs *zfs.Dataset
}

func NewImage(h *Host, id uuid.UUID) *Image {
	if id == nil {
		id = uuid.NewRandom()
	}
	return &Image{Host: h, UUID: id, Manifest: *schema.BlankImageManifest()}
}

func LoadImage(h *Host, id uuid.UUID) (*Image, error) {
	if id == nil {
		return nil, errors.New("No UUID given")
	}
	img := NewImage(h, id)
	if img.IsEmpty() {
		return nil, ErrNotFound
	}
	if err := img.Load(); err != nil {
		return nil, err
	}
	return img, nil
}

func (img *Image) Path(elem ...string) string {
	return img.Host.Path(append(
		[]string{"images", img.UUID.String()},
		elem...,
	)...)
}

func (img *Image) getRootfs() *zfs.Dataset {
	if img.rootfs == nil {
		ds, err := img.Host.Dataset.GetDataset(path.Join("images", img.UUID.String()))
		if err != nil {
			panic(err)
		}
		img.rootfs = ds
	}
	return img.rootfs
}

func (img *Image) IsEmpty() bool {
	_, err := os.Stat(img.Path("manifest"))
	return os.IsNotExist(err)
}

func (img *Image) Load() error {
	if img.IsEmpty() {
		return errors.New("Image is empty")
	}

	metadataJSON, err := ioutil.ReadFile(img.Path("metadata"))
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

	// Set access mode for the metadata server
	_, mdsGID := img.Host.GetMDSUGID()
	if err := os.Chown(img.Path(), 0, mdsGID); err != nil {
		return errors.Trace(err)
	}

	if err := os.Chown(img.Path("manifest"), 0, mdsGID); err != nil {
		return errors.Trace(err)
	}

	if err := os.Chmod(img.Path(), 0750); err != nil {
		return errors.Trace(err)
	}

	if err := os.Chmod(img.Path("manifest"), 0440); err != nil {
		return errors.Trace(err)
	}

	if img.Hash == nil {
		amiPath := ""
		if img.Host.Properties.GetBool("images.ami.store", false) {
			amiPath = img.Path("ami")
		}

		if hash, err := img.SaveAMI(amiPath, 0440); err != nil {
			return errors.Trace(err)
		} else {
			img.Hash = hash
		}
	}

	// Serialize metadata
	if metadataJSON, err := json.Marshal(img); err != nil {
		return errors.Trace(err)
	} else {
		if err := ioutil.WriteFile(img.Path("metadata"), metadataJSON, 0440); err != nil {
			return errors.Trace(err)
		}
	}

	if err := os.Symlink(img.UUID.String(), img.Path("..", img.Hash.String())); err != nil {
		return errors.Trace(err)
	}

	if _, err := img.getRootfs().Snapshot(imageSnapshotName); err != nil {
		return errors.Trace(err)
	}

	if err := img.getRootfs().Zfs("set", "readonly=on"); err != nil {
		return errors.Trace(err)
	}

	return nil
}

// Save image to an AMI file, return its hash.
//
// If path is an empty string, don't save the image, just return the
// hash. If path is "-", print image to stdout.
func (img *Image) SaveAMI(path string, perm os.FileMode) (*types.Hash, error) {
	archiver := run.Command("tar", "-C", img.Path(), "-c", "-f", "-", "manifest", "rootfs")
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
	manifestJSON, err := ioutil.ReadFile(img.Path("manifest"))
	if err != nil {
		return errors.Trace(err)
	}

	if err = json.Unmarshal(manifestJSON, &img.Manifest); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (img *Image) Destroy() (err error) {
	err = errors.Trace(img.getRootfs().Destroy("-r"))
	if img.Hash != nil {
		if err2 := os.Remove(img.Path("..", img.Hash.String())); err2 != nil && err == nil {
			err = errors.Trace(err2)
		}
		if err2 := os.RemoveAll(img.Path()); err2 != nil && err == nil {
			err = errors.Trace(err2)
		}
	}
	return
}

func (img *Image) Clone(dest, mountpoint string) (*zfs.Dataset, error) {
	snap, err := img.getRootfs().GetSnapshot(imageSnapshotName)
	if err != nil {
		return nil, errors.Trace(err)
	}

	ds, err := snap.Clone(dest, "-o", "mountpoint="+mountpoint)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return ds, nil
}

func (img *Image) RuntimeApp() schema.RuntimeApp {
	app := schema.RuntimeApp{
		Name:  img.Manifest.Name,
		Image: schema.RuntimeImage{Name: &img.Manifest.Name},
	}
	app.Annotations.Set("jetpack/image-uuid", img.UUID.String())
	if img.Hash != nil {
		app.Image.ID = *img.Hash
	} else {
		// TODO: do we really need to store ACI tarballs to have an image ID on built images?
		app.Image.ID.Set(fmt.Sprintf(
			"sha512-000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000%032x",
			img.UUID,
		))
	}
	return app
}

func (img *Image) buildPodManifest(exec []string) *schema.PodManifest {
	bpm := schema.BlankPodManifest()

	// Figure out working path that doesn't exist in the image's rootfs
	workDir := ".jetpack.build."
	for {
		if _, err := os.Stat(img.getRootfs().Path(workDir)); err != nil {
			if os.IsNotExist(err) {
				break
			}
			panic(err)
		}
		workDir = fmt.Sprintf(".jetpack.build.%v", uuid.NewRandom())
	}

	bprta := img.RuntimeApp()
	bprta.Name.Set("jetpack/build")
	bprta.App = &types.App{
		Exec:             exec,
		WorkingDirectory: "/" + workDir,
		User:             "0",
		Group:            "0",
	}
	bpm.Apps = append(bpm.Apps, bprta)

	// This is needed by freebsd-update at least, should be okay to
	// allow this in builders.
	bpm.Annotations.Set("jetpack/jail.conf/allow.chflags", "true")

	return bpm
}

func (img *Image) Build(buildDir string, addFiles []string, buildExec []string) (*Image, error) {
	buildPod, err := img.Host.CreatePod(img.buildPodManifest(buildExec))
	if err != nil {
		return nil, errors.Trace(err)
	}

	fullWorkDir := buildPod.Path("rootfs/0", buildPod.Manifest.Apps[0].App.WorkingDirectory)
	if err := os.Mkdir(fullWorkDir, 0700); err != nil {
		return nil, errors.Trace(err)
	}

	if buildDir[len(buildDir)-1] != '/' {
		buildDir += "/"
	}

	cpArgs := []string{"-R", buildDir}
	if addFiles != nil {
		cpArgs = append(cpArgs, addFiles...)
	}
	cpArgs = append(cpArgs, fullWorkDir)

	if err := run.Command("cp", cpArgs...).Run(); err != nil {
		return nil, errors.Trace(err)
	}

	if err := buildPod.RunApp(buildPod.Manifest.Apps[0].Name); err != nil {
		return nil, errors.Trace(err)
	}

	if err := buildPod.Kill(); err != nil {
		return nil, errors.Trace(err)
	}

	manifestBytes, err := ioutil.ReadFile(filepath.Join(fullWorkDir, "manifest.json"))
	if err != nil {
		return nil, errors.Trace(err)
	}

	if err := os.RemoveAll(fullWorkDir); err != nil {
		return nil, errors.Trace(err)
	}

	if err := os.Remove(buildPod.Path("rootfs/0/etc/resolv.conf")); err != nil && !os.IsNotExist(err) {
		return nil, errors.Trace(err)
	}

	// Pivot pod into an image
	childImage := NewImage(img.Host, buildPod.UUID)

	ds, err := img.Host.Dataset.GetDataset(path.Join("pods", buildPod.UUID.String(), "rootfs.0"))
	if err != nil {
		return nil, errors.Trace(err)
	}

	if err := ds.Set("mountpoint", childImage.Path("rootfs")); err != nil {
		return nil, errors.Trace(err)
	}

	if err := ds.Rename(img.Host.Dataset.ChildName(path.Join("images", childImage.UUID.String()))); err != nil {
		return nil, errors.Trace(err)
	}

	// Construct the child image's manifest

	if err := json.Unmarshal(manifestBytes, &childImage.Manifest); err != nil {
		return nil, errors.Trace(err)
	}

	// We don't need build pod anymore
	if err := buildPod.Destroy(); err != nil {
		return nil, errors.Trace(err)
	}
	buildPod = nil

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

	if manifestBytes, err := json.Marshal(childImage.Manifest); err != nil {
		return nil, errors.Trace(err)
	} else {
		if err := ioutil.WriteFile(childImage.Path("manifest"), manifestBytes, 0400); err != nil {
			return nil, errors.Trace(err)
		}
	}

	childImage.Timestamp = time.Now()

	if err := childImage.Seal(); err != nil {
		return nil, errors.Trace(err)
	}

	return childImage, nil
}
