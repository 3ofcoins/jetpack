package jetpack

import "bytes"
import "crypto/sha512"
import "encoding/json"
import "fmt"
import "io"
import "io/ioutil"
import "os"
import "path"
import "path/filepath"
import "strings"
import "time"

import "code.google.com/p/go-uuid/uuid"
import "github.com/appc/spec/schema"
import "github.com/appc/spec/schema/types"
import "github.com/juju/errors"

import "../run"
import "../zfs"

type Image struct {
	Dataset  *zfs.Dataset         `json:"-"`
	Manager  *ImageManager        `json:"-"`
	Manifest schema.ImageManifest `json:"-"`

	UUID uuid.UUID `json:"-"`

	Hash      *types.Hash `json:",omitempty"`
	Origin    string
	Timestamp time.Time
}

func NewImage(ds *zfs.Dataset, mgr *ImageManager) (*Image, error) {
	basename := path.Base(ds.Name)
	img := &Image{
		Dataset: ds,
		Manager: mgr,
		UUID:    uuid.Parse(basename),
	}
	if img.UUID == nil {
		return nil, errors.Errorf("Invalid UUID: %#v", basename)
	}
	return img, nil
}

func GetImage(ds *zfs.Dataset, mgr *ImageManager) (img *Image, err error) {
	img, err = NewImage(ds, mgr)
	if err != nil {
		return
	}
	err = img.Load()
	return
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

	if err := img.LoadManifest(); err != nil {
		return errors.Trace(err)
	}

	if app := img.Manifest.App; app != nil {
		if len(app.EventHandlers) != 0 {
			return errors.New("TODO: event handlers are not supported")
		}
		if len(app.Ports) != 0 {
			return errors.New("TODO: ports are not supported")
		}
		if len(app.Isolators) != 0 {
			return errors.New("TODO: isolators are not supported")
		}
	}

	return nil
}

func (img *Image) Seal() error {
	// Make sure that manifest exists and validates correctly
	if err := img.LoadManifest(); err != nil {
		return errors.Trace(err)
	}

	if img.Hash == nil {
		// Save AMI, acquire hash.
		// TODO: separate dir/volume for AMIs, only symlink from image dir?
		tarCmd := run.Command("tar", "-C", img.Dataset.Mountpoint, "-c", "-f", "-", "manifest", "rootfs")
		if tarOut, err := tarCmd.StdoutPipe(); err != nil {
			return errors.Trace(err)
		} else {
			hash := sha512.New()
			if err := tarCmd.Start(); err != nil {
				return errors.Trace(err)
			}
			if img.Manager.Host.Properties.GetBool("images.ami.store", false) {
				if amiFile, err := os.OpenFile(img.Dataset.Path("ami"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0400); err != nil {
					return errors.Trace(err)
				} else {
					defer amiFile.Close()
					xzCmd := run.Command("xz", "-z")
					xzCmd.Cmd.Stdout = amiFile
					if xzIn, err := xzCmd.StdinPipe(); err != nil {
						return errors.Trace(err)
					} else {
						if err := xzCmd.Start(); err != nil {
							return errors.Trace(err)
						}
						if _, err := io.Copy(xzIn, io.TeeReader(tarOut, hash)); err != nil {
							return errors.Trace(err)
						}
					}
				}
			} else {
				if _, err := io.Copy(hash, tarOut); err != nil {
					return errors.Trace(err)
				}
			}

			if hash, err := types.NewHash(fmt.Sprintf("sha512-%x", hash.Sum(nil))); err != nil {
				// CAN'T HAPPEN, srsly
				return errors.Trace(err)
			} else {
				img.Hash = hash
			}
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

	if err := os.Symlink(path.Base(img.Dataset.Name), img.Manager.Dataset.Path(img.Hash.String())); err != nil {
		return errors.Trace(err)
	}

	if _, err := img.Dataset.Snapshot("seal"); err != nil {
		return errors.Trace(err)
	}

	if err := img.Dataset.Zfs("set", "readonly=on"); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (img *Image) LoadManifest() error {
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
	if containers, err := img.Manager.Host.Containers.All(); err != nil {
		return nil, errors.Trace(err)
	} else {
		snap := img.Dataset.SnapshotName("seal")
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
		if err2 := os.Remove(img.Manager.Dataset.Path(img.Hash.String())); err2 != nil && err == nil {
			err = errors.Trace(err2)
		}
	}
	return
}

type imageLabels []string

func (lb imageLabels) String() string {
	return strings.Join(lb, " ")
}

func (img *Image) PrettyLabels() imageLabels {
	labels := make(imageLabels, len(img.Manifest.Labels))
	for i, l := range img.Manifest.Labels {
		labels[i] = fmt.Sprintf("%v=%#v", l.Name, l.Value)
	}
	return labels
}

func (img *Image) Clone(snapshot, dest string) (*zfs.Dataset, error) {
	snap, err := img.Dataset.GetSnapshot(snapshot)
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
	app := schema.RuntimeApp{
		Name: img.Manifest.Name,
		Annotations: map[types.ACName]string{
			"jetpack/image-uuid": img.UUID.String(),
		},
	}
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

func (img *Image) Run(app *types.App, keep bool) (err1 error) {
	c, err := img.Manager.Host.Containers.Clone(img)
	if err != nil {
		return errors.Trace(err)
	}
	if !keep {
		defer func() {
			if err := c.Destroy(); err != nil {
				err = errors.Trace(err)
				if err1 != nil {
					err1 = errors.Wrap(err1, err)
				} else {
					err1 = err
				}
			}
		}()
	}
	return c.Run(app)
}

func (img *Image) Build(buildDir string, addFiles []string, buildExec []string) (*Image, error) {
	var buildContainer *Container
	if c, err := img.Manager.Host.Containers.Clone(img); err != nil {
		return nil, errors.Trace(err)
	} else {
		buildContainer = c
	}

	// This is needed by freebsd-update at least, should be okay to
	// allow this in builders.
	buildContainer.Manifest.Annotations["jetpack/jail.conf/allow.chflags"] = "true"

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
	if err := buildContainer.Dataset.Rename(img.Manager.Dataset.ChildName(uuid)); err != nil {
		return nil, errors.Trace(err)
	}

	ds, err := img.Manager.Dataset.GetDataset(uuid)
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

	childImage, err := NewImage(ds, img.Manager)
	if err != nil {
		return nil, errors.Trace(err)
	}

	// Construct the final manifest

	// defaults that are always present
	manifest := map[string]interface{}{
		"acKind":    "ImageManifest",
		"acVersion": schema.AppContainerVersion,
	}

	// Merge what the build directory has left for us
	if manifestBytes, err := ioutil.ReadFile(childImage.Dataset.Path("build.manifest")); err != nil {
		return nil, errors.Trace(err)
	} else {
		if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
			return nil, errors.Trace(err)
		}
	}

	// FIXME: the below is UGLY. Can we make it prettier?

	// Annotate with timestamp if there is no such annotation yet
	if manifest["annotations"] == nil {
		manifest["annotations"] = make(map[string]interface{})
	}

	annotations := manifest["annotations"].(map[string]interface{})
	if _, ok := annotations["timestamp"]; !ok {
		annotations["timestamp"] = time.Now()
	}

	// Merge OS and arch from parent image if it's not set
	// TODO: can we prevent this?
	hasLabels := map[string]bool{}
	for _, labelI := range manifest["labels"].([]interface{}) {
		label := labelI.(map[string]interface{})
		hasLabels[label["name"].(string)] = true
	}

	if val, ok := img.Manifest.GetLabel("os"); ok && !hasLabels["os"] {
		manifest["labels"] = append(manifest["labels"].([]interface{}),
			map[string]interface{}{"name": "os", "val": val})
	}

	if val, ok := img.Manifest.GetLabel("arch"); ok && !hasLabels["arch"] {
		manifest["labels"] = append(manifest["labels"].([]interface{}),
			map[string]interface{}{"name": "arch", "val": val})
	}

	// TODO: merge app from parent image?

	childImage.Origin = img.UUID.String()

	if manifestBytes, err := json.Marshal(manifest); err != nil {
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

func (ii ImageSlice) Len() int           { return len(ii) }
func (ii ImageSlice) Less(i, j int) bool { return bytes.Compare(ii[i].UUID, ii[j].UUID) < 0 }
func (ii ImageSlice) Swap(i, j int)      { ii[i], ii[j] = ii[j], ii[i] }

func (ii ImageSlice) Table() [][]string {
	rows := make([][]string, len(ii)+1)
	rows[0] = []string{"UUID", "NAME", "LABELS"}
	for i, img := range ii {
		rows[i+1] = append([]string{img.UUID.String(), string(img.Manifest.Name)}, img.PrettyLabels()...)
	}
	return rows
}
