package jetpack

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"code.google.com/p/go-uuid/uuid"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/juju/errors"

	"lib/ui"
	"lib/zfs"
)

const imageSnapshotName = "seal"

type Image struct {
	UUID     uuid.UUID            `json:"-"`
	Host     *Host                `json:"-"`
	Manifest schema.ImageManifest `json:"-"`

	Hash      *types.Hash `json:",omitempty"`
	Timestamp time.Time

	rootfs *zfs.Dataset
	ui     *ui.UI
}

func NewImage(h *Host, id uuid.UUID) *Image {
	if id == nil {
		id = uuid.NewRandom()
	}
	return &Image{
		Host:     h,
		UUID:     id,
		Manifest: *schema.BlankImageManifest(),
		ui:       ui.NewUI("blue", "image", id.String()),
	}
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

func (img *Image) ID() string {
	if img.Hash == nil {
		return fmt.Sprintf("uuid-%v", img.UUID)
	}
	return img.Hash.String()
}

func (img *Image) String() string {
	labels := make([]string, len(img.Manifest.Labels))
	for i, label := range img.Manifest.Labels {
		if label.Name == "version" {
			// HACK: we want version to `sort.Strings()` before all other
			// labels that will start with a comma. A colon we'll want to
			// use to separate version from name is asciibetically after a
			// comma, so we use `+` prefix here, and change it to `:` after
			// the sort.
			labels[i] = fmt.Sprintf("+%v", label.Value)
		} else {
			labels[i] = fmt.Sprintf(",%v=%#v", label.Name, label.Value)
		}
	}
	sort.Strings(labels)
	if labels[0][0] == '+' {
		labels[0] = ":" + labels[0][1:]
	}

	return string(img.Manifest.Name) + strings.Join(labels, "")
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
	_, er1 := os.Stat(img.Path("manifest"))
	_, er2 := os.Stat(img.Path("metadata"))
	return os.IsNotExist(er1) || os.IsNotExist(er2)
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
	img.ui.Println("Destroying")
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
	img.ui.Debugf("Cloning rootfs as %v at %v", dest, mountpoint)
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

func (img *Image) saveManifest() error {
	img.ui.Debug("Saving manifest")
	if manifestBytes, err := json.Marshal(img.Manifest); err != nil {
		return errors.Trace(err)
	} else {
		if err := ioutil.WriteFile(img.Path("manifest"), manifestBytes, 0444); err != nil {
			return errors.Trace(err)
		}
	}

	// Make sure that manifest exists and validates correctly
	if err := img.loadManifest(); err != nil {
		return errors.Trace(err)
	}

	return nil
}

// Finalize unpacked/built image
func (img *Image) sealImage() error {
	img.ui.Debug("Sealing")
	img.Timestamp = time.Now()

	// Set access mode for the metadata server
	_, mdsGID := img.Host.GetMDSUGID()
	if err := os.Chown(img.Path(), 0, mdsGID); err != nil {
		return errors.Trace(err)
	}

	if err := os.Chmod(img.Path(), 0750); err != nil {
		return errors.Trace(err)
	}

	if err := os.Chown(img.Path("manifest"), 0, mdsGID); err != nil {
		return errors.Trace(err)
	}

	if err := os.Chmod(img.Path("manifest"), 0440); err != nil {
		return errors.Trace(err)
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
