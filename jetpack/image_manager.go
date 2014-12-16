package jetpack

import "path"

import "code.google.com/p/go-uuid/uuid"
import "github.com/juju/errors"

import "github.com/3ofcoins/go-zfs"

type ImageManager struct {
	*zfs.Dataset `json:"-"`
}

func (imgr *ImageManager) All() (ImageSlice, error) {
	if dss, err := imgr.Children(1); err != nil {
		return nil, errors.Trace(err)
	} else {
		rv := make([]*Image, len(dss))
		for i, ds := range dss {
			if img, err := GetImage(ds); err != nil {
				return nil, errors.Trace(err)
			} else {
				rv[i] = img
			}
		}
		return rv, nil
	}
}

func (imgr *ImageManager) Get(spec string) (*Image, error) {
	// TODO: cache image list?
	imgs, err := imgr.All()
	if err != nil {
		return nil, err
	}
	for _, img := range imgs {
		// TODO: more sophisticated spec (as in ACI/discovery, maybe)
		if string(img.Name) == spec {
			return img, nil
		}
	}
	return nil, nil
}

func (imgr *ImageManager) Import(uri string) (*Image, error) {
	if ds, err := zfs.CreateFilesystem(
		path.Join(imgr.Name, uuid.NewRandom().String()),
		nil,
	); err != nil {
		return nil, errors.Trace(err)
	} else {
		return ImportImage(ds, uri)
	}
}
