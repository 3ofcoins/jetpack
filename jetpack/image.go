package jetpack

import "archive/tar"
import "crypto/sha256"
import "encoding/base64"
import "fmt"
import "io"
import "io/ioutil"
import "os"
import "path"
import "path/filepath"
import "reflect"
import "strings"

import "github.com/appc/spec/schema"
import "github.com/appc/spec/schema/types"
import "github.com/juju/errors"

import "github.com/3ofcoins/go-zfs"

type Image struct {
	schema.ImageManifest
	DS     *zfs.Dataset
	Sha256 []byte
}

func GetImage(ds *zfs.Dataset) (*Image, error) {
	img := &Image{DS: ds}

	b64str := path.Base(ds.Name)
	if n := len(b64str) % 4; n != 0 {
		b64str += strings.Repeat("=", 4-n)
	}

	sha256, err := base64.URLEncoding.DecodeString(b64str)
	if err != nil {
		return nil, errors.Trace(err)
	}
	img.Sha256 = sha256

	manifestJSON, err := ioutil.ReadFile(filepath.Join(img.Basedir(), "manifest"))
	if err != nil {
		return nil, errors.Trace(err)
	}

	err = img.ImageManifest.UnmarshalJSON(manifestJSON)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return img, nil
}

func ImportImage(h *Host, uri string) (*Image, error) {
	ff, err := ioutil.TempFile(h.imagesFS.Mountpoint, ".image.fetch.")
	if err != nil {
		return nil, errors.Trace(err)
	}
	ff.Close()
	fetchPath := ff.Name() + ".data"
	defer os.Remove(ff.Name())
	defer os.Remove(fetchPath)

	err = runCommand("fetch", "-l", "-o", fetchPath, uri)
	if err != nil {
		return nil, errors.Trace(err)
	}

	f, err := os.Open(fetchPath)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer f.Close()

	img := &Image{}

	// Save uncompressed tar file to unpack later on
	cf, err := ioutil.TempFile(h.imagesFS.Mountpoint, ".image.read.")
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer os.Remove(cf.Name())

	dr, err := DecompressingReader(f)
	if err != nil {
		return nil, errors.Trace(err)
	}
	hash := sha256.New()

	tee := io.TeeReader(io.TeeReader(dr, hash), cf)
	tr := tar.NewReader(tee)

	var manifestJSON []byte

TarLoop:
	for {
		switch hdr, err := tr.Next(); err {
		case nil:
			if hdr.Name == "manifest" {
				manifestJSON, err = ioutil.ReadAll(tr)
				if err != nil {
					return nil, errors.Trace(err)
				}
				break TarLoop
			}
		case io.EOF:
			break TarLoop
		default:
			return nil, errors.Trace(err)
		}
	}

	// Finish reading file, we stop parsing tar once we got manifest
	if _, err := io.Copy(ioutil.Discard, tee); err != nil {
		return nil, errors.Trace(err)
	}

	img.Sha256 = hash.Sum(nil)

	if manifestJSON != nil {
		// FIXME: is no manifest legal now?
		err = img.ImageManifest.UnmarshalJSON(manifestJSON)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}

	if err := cf.Close(); err != nil {
		return nil, errors.Trace(err)
	}

	ds, err := h.CreateFilesystem(
		map[string]string{
			"mountpoint": filepath.Join(
				h.imagesFS.Mountpoint,
				img.Checksum64(),
				"rootfs",
			),
		},
		"images", img.Checksum64())
	if err != nil {
		return nil, errors.Trace(err)
	}
	img.DS = ds

	props := map[string]string{
		"jetpack:checksum":      img.Checksum(),
		"jetpack:image":         string(img.Name),
		"jetpack:app":           bool2zfs(img.IsApp()),
		"jetpack:imported_from": uri,
	}
	for _, label := range img.Labels {
		props[fmt.Sprintf("jetpack:label:%v", label.Name)] = label.Value
	}
	for annName, annValue := range img.Annotations {
		props[fmt.Sprintf("jetpack:annotation:%v", annName)] = annValue
	}

	for propName, propValue := range props {
		if err := img.DS.SetProperty(propName, propValue); err != nil {
			return nil, errors.Trace(err)
		}
	}

	err = runCommand("tar", "-C", img.Basedir(), "-xf", cf.Name())
	if err != nil {
		return nil, errors.Trace(err)
	}

	_, err = ds.Snapshot("aci", false)
	if err != nil {
		return nil, errors.Trace(err)
	}

	for propName, propValue := range map[string]string{"canmount": "off", "readonly": "on"} {
		if err := ds.SetProperty(propName, propValue); err != nil {
			return nil, errors.Trace(err)
		}
	}

	return img, nil
}

func (img *Image) Checksum() string {
	return fmt.Sprintf("sha256-%x", img.Sha256)
}

func (img *Image) Checksum64() string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(img.Sha256), "=")
}

func (img *Image) Basedir() string {
	return filepath.Dir(img.DS.Mountpoint)
}

func (img *Image) String() string {
	return fmt.Sprintf("ACI:%v(%v)", img.Checksum(), img.Name)
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
