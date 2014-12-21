package jetpack

import "code.google.com/p/go-uuid/uuid"
import "github.com/juju/errors"

type ImageManager struct {
	Dataset *Dataset `json:"-"`
}

func (imgr *ImageManager) All() (ImageSlice, error) {
	if dss, err := imgr.Dataset.Children(1); err != nil {
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
	if ds, err := imgr.Dataset.GetDataset(spec); err == nil {
		return GetImage(ds)
	}

	// TODO: cache image list?
	imgs, err := imgr.All()
	if err != nil {
		return nil, err
	}
	for _, img := range imgs {
		// TODO: more sophisticated spec (as in ACI/discovery, maybe)
		if string(img.Manifest.Name) == spec || (img.Hash != nil && img.Hash.String() == spec) {
			return img, nil
		}
	}
	return nil, nil
}

func (imgr *ImageManager) Create() (*Image, error) {
	if ds, err := imgr.Dataset.CreateFilesystem(uuid.NewRandom().String(), nil); err != nil {
		return nil, errors.Trace(err)
	} else {
		return NewImage(ds)
	}
}

func (imgr *ImageManager) Import(uri string) (*Image, error) {
	if img, err := imgr.Create(); err != nil {
		return nil, errors.Trace(err)
	} else {
		if err := img.Import(uri); err != nil {
			return nil, errors.Trace(err)
		} else {
			return img, nil
		}
	}
}
