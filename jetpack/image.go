package jetpack

import "bytes"
import "encoding/json"
import "fmt"
import "io/ioutil"
import "os"
import "path"
import "strings"
import "time"

import "code.google.com/p/go-uuid/uuid"
import "github.com/appc/spec/schema"
import "github.com/appc/spec/schema/types"
import "github.com/juju/errors"

type Image struct {
	Dataset  *Dataset             `json:"-"`
	Manifest schema.ImageManifest `json:"-"`

	UUID uuid.UUID `json:"-"`

	Hash      *types.Hash `json:",omitempty"`
	Origin    string
	Timestamp time.Time
}

func NewImage(ds *Dataset) (*Image, error) {
	img := &Image{
		Dataset: ds,
		UUID:    uuid.Parse(path.Base(ds.Name)),
	}
	if img.UUID == nil {
		return nil, errors.New("Invalid UUID")
	}
	return img, nil
}

func GetImage(ds *Dataset) (img *Image, err error) {
	img, err = NewImage(ds)
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

	if err := img.readManifest(""); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (img *Image) Import(uri string) error {
	if !img.IsEmpty() {
		return errors.New("Image is not empty")
	}

	img.Origin = uri
	img.Timestamp = time.Now()

	if hash, err := UnpackImage(uri, img.Dataset.Mountpoint); err != nil {
		return errors.Trace(err)
	} else {
		img.Hash = &hash
	}

	if err := img.readManifest(""); err != nil {
		return errors.Trace(err)
	}

	return errors.Trace(img.seal())
}

func (img *Image) Commit() error {
	// Serialize metadata
	if manifestJSON, err := json.Marshal(img.Manifest); err != nil {
		return errors.Trace(err)
	} else {
		if err := ioutil.WriteFile(img.Dataset.Path("manifest"), manifestJSON, 0400); err != nil {
			return errors.Trace(err)
		}
	}
	return img.seal()
}

func (img *Image) seal() error {
	// Serialize metadata
	if metadataJSON, err := json.Marshal(img); err != nil {
		return errors.Trace(err)
	} else {
		if err := ioutil.WriteFile(img.Dataset.Path("metadata"), metadataJSON, 0400); err != nil {
			return errors.Trace(err)
		}
	}

	if _, err := img.Dataset.Snapshot("aci", false); err != nil {
		return errors.Trace(err)
	}

	if err := img.Dataset.SetProperty("readonly", "on"); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (img *Image) readManifest(path string) error {
	if path == "" {
		path = img.Dataset.Path("manifest")
	}
	manifestJSON, err := ioutil.ReadFile(path)
	if err != nil {
		return errors.Trace(err)
	}

	if err = json.Unmarshal(manifestJSON, &img.Manifest); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (img *Image) PrettyLabels() string {
	labels := make([]string, len(img.Manifest.Labels))
	for i, l := range img.Manifest.Labels {
		labels[i] = fmt.Sprintf("%v=%#v", l.Name, l.Value)
	}
	return strings.Join(labels, " ")
}

func (img *Image) Clone(snapshot, dest string) (*Dataset, error) {
	snap, err := img.Dataset.GetSnapshot(snapshot)
	if err != nil {
		return nil, errors.Trace(err)
	}

	ds, err := snap.Clone(dest, nil)
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
	if img.Hash != nil {
		app.ImageID = *img.Hash
	}
	return app
}

func (img *Image) Summary() string {
	return fmt.Sprintf("%v %v %v",
		img.UUID, img.Manifest.Name, img.PrettyLabels())
}

// For sorting
type ImageSlice []*Image

func (ii ImageSlice) Len() int           { return len(ii) }
func (ii ImageSlice) Less(i, j int) bool { return bytes.Compare(ii[i].UUID, ii[j].UUID) < 0 }
func (ii ImageSlice) Swap(i, j int)      { ii[i], ii[j] = ii[j], ii[i] }
