package jetpack

import "encoding/json"
import "io/ioutil"
import "net/url"
import "os"
import "strings"
import "time"

import "code.google.com/p/go-uuid/uuid"
import "github.com/appc/spec/schema"
import "github.com/appc/spec/schema/types"
import "github.com/juju/errors"

import "github.com/3ofcoins/jetpack/run"
import "github.com/3ofcoins/jetpack/zfs"

type ImageManager struct {
	Dataset *zfs.Dataset `json:"-"`
	Host    *Host        `json:"-"`
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

func (imgr *ImageManager) Find1(query string) (*Image, error) {
	if imgs, err := imgr.Find(query); err != nil {
		return nil, err
	} else {
		if len(imgs) == 1 {
			return imgs[0], nil
		} else {
			return nil, ErrManyFound
		}
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
	if ds, err := imgr.Dataset.CreateDataset(uuid.NewRandom().String()); err != nil {
		return nil, errors.Trace(err)
	} else {
		return NewImage(ds, imgr)
	}
}

func (imgr *ImageManager) Import(imageUri, manifestUri string) (*Image, error) {
	img, err := imgr.Create()
	if err != nil {
		return nil, errors.Trace(err)
	}
	img.Origin = imageUri
	img.Timestamp = time.Now()

	if manifestUri == "" {
		if hash, err := UnpackImage(imageUri, img.Dataset.Mountpoint, img.Dataset.Path("ami")); err != nil {
			return nil, errors.Trace(err)
		} else {
			img.Hash = &hash
		}
	} else {
		// FIXME: does this really belong here, or rather in an Image method?

		rootfsPath := img.Dataset.Path("rootfs")
		if err := os.Mkdir(rootfsPath, 0755); err != nil {
			return nil, errors.Trace(err)
		}
		if hash, err := UnpackImage(imageUri, rootfsPath, ""); err != nil {
			return nil, errors.Trace(err)
		} else {
			img.Hash = &hash
		}

		manifestBytes, err := run.Command("fetch", "-o", "-", manifestUri).Output()
		if err != nil {
			return nil, errors.Trace(err)
		}

		// Construct final manifest
		// FIXME: this may be somehow merged with build, and final manifest should be validated
		manifest := map[string]interface{}{
			"acKind":    "ImageManifest",
			"acVersion": schema.AppContainerVersion,
		}

		if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
			return nil, errors.Trace(err)
		}

		if manifest["annotations"] == nil {
			manifest["annotations"] = make(map[string]interface{})
		}

		annotations := manifest["annotations"].(map[string]interface{})
		if _, ok := annotations["timestamp"]; !ok {
			annotations["timestamp"] = time.Now()
		}

		if manifestBytes, err := json.Marshal(manifest); err != nil {
			return nil, errors.Trace(err)
		} else {
			if err := ioutil.WriteFile(img.Dataset.Path("manifest"), manifestBytes, 0400); err != nil {
				return nil, errors.Trace(err)
			}
		}
	}

	if err := img.Seal(); err != nil {
		return nil, errors.Trace(err)
	}

	return img, nil
}
