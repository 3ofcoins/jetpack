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

import "github.com/3ofcoins/go-zfs"

type Image struct {
	Dataset  *Dataset             `json:"-"`
	Manager  *ImageManager        `json:"-"`
	Manifest schema.ImageManifest `json:"-"`

	UUID uuid.UUID `json:"-"`

	Hash      *types.Hash `json:",omitempty"`
	Origin    string
	Timestamp time.Time
}

func NewImage(ds *Dataset, mgr *ImageManager) (*Image, error) {
	img := &Image{
		Dataset: ds,
		Manager: mgr,
		UUID:    uuid.Parse(path.Base(ds.Name)),
	}
	if img.UUID == nil {
		return nil, errors.New("Invalid UUID")
	}
	return img, nil
}

func GetImage(ds *Dataset, mgr *ImageManager) (img *Image, err error) {
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

	return errors.Trace(img.Seal())
}

func (img *Image) Seal() error {
	// Make sure that manifest exists and validates correctly
	if err := img.LoadManifest(); err != nil {
		return errors.Trace(err)
	}

	// Serialize metadata
	if metadataJSON, err := json.Marshal(img); err != nil {
		return errors.Trace(err)
	} else {
		if err := ioutil.WriteFile(img.Dataset.Path("metadata"), metadataJSON, 0400); err != nil {
			return errors.Trace(err)
		}
	}

	if _, err := img.Dataset.Snapshot("seal", false); err != nil {
		return errors.Trace(err)
	}

	if err := img.Dataset.SetProperty("readonly", "on"); err != nil {
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
		snap := img.Dataset.Name + "@seal"
		for _, container := range containers {
			if container.Dataset.Origin == snap {
				children = append(children, container)
			}
		}
		return
	}
}

func (img *Image) Destroy() error {
	return img.Dataset.Destroy(zfs.DestroyRecursive)
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
