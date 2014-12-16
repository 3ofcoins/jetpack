package jetpack

import "encoding/json"
import "fmt"
import "io/ioutil"
import "path"
import "path/filepath"
import "reflect"
import "strings"
import "time"

import "code.google.com/p/go-uuid/uuid"
import "github.com/appc/spec/schema"
import "github.com/appc/spec/schema/types"
import "github.com/juju/errors"

import "github.com/3ofcoins/go-zfs"

type ImageMetadata struct {
	Hash      types.Hash
	Origin    string
	Timestamp time.Time
}

type Image struct {
	schema.ImageManifest
	ImageMetadata
	UUID uuid.UUID
	DS   *zfs.Dataset
}

func GetImage(ds *zfs.Dataset) (*Image, error) {
	img := &Image{
		DS:   ds,
		UUID: uuid.Parse(path.Base(ds.Name)),
	}

	if img.UUID == nil {
		return nil, errors.New("Invalid UUID")
	}

	metadataJSON, err := ioutil.ReadFile(img.Path("metadata"))
	if err != nil {
		// FIXME: if metadata does not exist (yet), this means that image
		// is being created (imported/built), this should not be treated
		// as error, maybe as input to separate cleanup task.
		return nil, errors.Trace(err)
	}

	if err = json.Unmarshal(metadataJSON, &img.ImageMetadata); err != nil {
		return nil, errors.Trace(err)
	}

	if err := img.readManifest(); err != nil {
		return nil, errors.Trace(err)
	}

	return img, nil
}

func ImportImage(h *Host, uri string) (*Image, error) {
	img := &Image{
		UUID: uuid.NewRandom(),
		ImageMetadata: ImageMetadata{
			Origin:    uri,
			Timestamp: time.Now(),
		},
	}

	if ds, err := h.CreateFilesystem(nil, "images", img.UUID.String()); err != nil {
		return nil, errors.Trace(err)
	} else {
		img.DS = ds
	}

	if hash, err := UnpackImage(uri, img.Path()); err != nil {
		// TODO: cleanup
		return nil, errors.Trace(err)
	} else {
		img.Hash = hash
	}

	if err := img.readManifest(); err != nil {
		// TODO: cleanup
		return nil, errors.Trace(err)
	}

	// Serialize metadata
	if metadataJSON, err := json.Marshal(img.ImageMetadata); err != nil {
		// TODO: cleanup
		return nil, errors.Trace(err)
	} else {
		if err := ioutil.WriteFile(img.Path("metadata"), metadataJSON, 0400); err != nil {
			// TODO: cleanup
			return nil, errors.Trace(err)
		}
	}

	if _, err := img.DS.Snapshot("aci", false); err != nil {
		// TODO: cleanup
		return nil, errors.Trace(err)
	}

	if err := img.DS.SetProperty("readonly", "on"); err != nil {
		// TODO: cleanup
		return nil, errors.Trace(err)
	}

	return img, nil
}

func (img *Image) Path(pieces ...string) string {
	return filepath.Join(append([]string{img.DS.Mountpoint}, pieces...)...)
}

func (img *Image) readManifest() error {
	manifestJSON, err := ioutil.ReadFile(img.Path("manifest"))
	if err != nil {
		return errors.Trace(err)
	}

	if err = json.Unmarshal(manifestJSON, &img.ImageManifest); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (img *Image) String() string {
	return fmt.Sprintf("ACI:%v(%v)", img.Name, img.Hash)
}

func (img *Image) PrettyLabels() string {
	labels := make([]string, len(img.Labels))
	for i, l := range img.Labels {
		labels[i] = fmt.Sprintf("%v=%#v", l.Name, l.Value)
	}
	return strings.Join(labels, " ")
}

func (img *Image) IsApp() bool {
	return !reflect.DeepEqual(img.App, types.App{})
}

// For sorting
type Images []*Image

func (ii Images) Len() int           { return len(ii) }
func (ii Images) Less(i, j int) bool { return ii[i].Name < ii[j].Name }
func (ii Images) Swap(i, j int)      { ii[i], ii[j] = ii[j], ii[i] }
