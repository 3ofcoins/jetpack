package jetpack

import "archive/tar"
import "crypto/sha256"
import "encoding/base64"
import "fmt"
import "io"
import "io/ioutil"
import "os"
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

func ImportImage(h *Host, rs io.ReadSeeker) (*Image, error) {
	img := &Image{}

	// Save uncompressed tar file to unpack later on
	cf, err := ioutil.TempFile(h.imagesFS.Mountpoint, ".image.read.")
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer os.Remove(cf.Name())

	dr, err := DecompressingReader(rs)
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
		"jetpack:image":    img.String(),
		"jetpack:checksum": img.Checksum(),
		"jetpack:name":     string(img.Name),
		"jetpack:app":      bool2zfs(img.IsApp()),
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

	err = runCommand("tar", "-C", filepath.Dir(ds.Mountpoint), "-xf", cf.Name())
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

func ImportImageFromFile(h *Host, path string) (*Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer f.Close()
	return ImportImage(h, f)
}

func (img *Image) Checksum() string {
	return fmt.Sprintf("sha256-%x", img.Sha256)
}

func (img *Image) Checksum64() string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(img.Sha256), "=")
}

func (img *Image) String() string {
	version, _ := img.Get("version")
	os, _ := img.Get("os")
	arch, _ := img.Get("arch")
	return fmt.Sprintf("%v-%v-%v-%v.aci[%v]",
		img.Name, version, os, arch,
		img.Checksum())
}

func (img *Image) IsApp() bool {
	return !reflect.DeepEqual(img.App, types.App{})
}
