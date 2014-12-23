package jetpack

import "strings"
import "net/url"

import "code.google.com/p/go-uuid/uuid"
import "github.com/appc/spec/schema/types"
import "github.com/juju/errors"

type ImageManager struct {
	Dataset *Dataset `json:"-"`
	Host    *Host    `json:"-"`
}

func (imgr *ImageManager) All() (ImageSlice, error) {
	if dss, err := imgr.Dataset.Children(1); err != nil {
		return nil, errors.Trace(err)
	} else {
		rv := make([]*Image, 0, len(dss))
		for _, ds := range dss {
			if ds.Type != "filesystem" {
				continue
			}
			if img, err := GetImage(ds, imgr); err != nil {
				// TODO: warn
				return nil, errors.Trace(err)
			} else {
				rv = append(rv, img)
			}
		}
		return rv, nil
	}
}

func (imgr *ImageManager) Find(query string) (ImageSlice, error) {
	// Empty query means all images
	if query == "" {
		if imgs, err := imgr.All(); err == nil && len(imgs) == 0 {
			return nil, ErrNotFound
		} else {
			return imgs, err
		}
	}

	// Try UUID
	if uuid := uuid.Parse(query); uuid != nil {
		if img, err := imgr.Get(uuid.String()); err != nil {
			return nil, errors.Trace(err)
		} else {
			return ImageSlice{img}, nil
		}
	}

	// We'll search for images, let's prepare the list now
	imgs, err := imgr.All()
	if err != nil {
		return nil, errors.Trace(err)
	}

	// Try hash
	if hash, err := types.NewHash(query); err == nil {
		for _, img := range imgs {
			if img.Hash != nil && *img.Hash == *hash {
				return ImageSlice{img}, nil
			}
		}
		return nil, ErrNotFound
	}

	// Bad luck, we have a query. Let's transform it into a query string and parse it this way…
	query = strings.Replace(query, ":", ",version=", 1)
	query = strings.Replace(query, ",", "&", -1)
	query = "name=" + query
	v, err := url.ParseQuery(query)
	if err != nil {
		return nil, err
	}

	name := types.ACName(v["name"][0])
	delete(v, "name")

	rv := ImageSlice{}
images:
	for _, img := range imgs {
		if img.Manifest.Name == name {
		labels:
			for label, values := range v {
				if imgvalue, ok := img.Manifest.GetLabel(label); ok {
					for _, value := range values {
						if imgvalue == value {
							// We got a good value, next label
							continue labels
						}
					}
					// No good values were found, next image
					continue images
				} else {
					continue images
				}
			}
			// If we got here, image was not rejected, so it's a good one.
			rv = append(rv, img)
		}
	}

	if len(rv) == 0 {
		return nil, ErrNotFound
	} else {
		return rv, nil
	}
}

func (imgr *ImageManager) Get(spec string) (*Image, error) {
	// TODO: by uuid.UUID only
	if ds, err := imgr.Dataset.GetDataset(spec); err == nil {
		return GetImage(ds, imgr)
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
	return nil, ErrNotFound
}

func (imgr *ImageManager) Create() (*Image, error) {
	if ds, err := imgr.Dataset.CreateFilesystem(uuid.NewRandom().String(), nil); err != nil {
		return nil, errors.Trace(err)
	} else {
		return NewImage(ds, imgr)
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
