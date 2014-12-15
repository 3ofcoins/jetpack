package jetpack

import "fmt"
import "log"
import "path"

import "github.com/3ofcoins/go-zfs"
import "github.com/juju/errors"

const DefaultMountpoint = "/srv/jetpack"

type Host struct {
	*zfs.Dataset
	imagesFS, containersFS *zfs.Dataset
}

func GetHost(rootDataset string) (*Host, error) {
	if ds, err := zfs.GetDataset(rootDataset); err != nil {
		return nil, errors.Trace(err)
	} else {
		h := &Host{Dataset: ds}
		h.imagesFS, err = h.getDataset("images")
		if err != nil {
			return nil, errors.Trace(err)
		}
		h.containersFS, err = h.getDataset("containers")
		if err != nil {
			return nil, errors.Trace(err)
		}
		return h, nil
	}
}

var storageZFSProperties = map[string]string{
	"atime":    "off",
	"compress": "lz4",
	"dedup":    "on",
}

func CreateHost(rootDataset, rootMountpoint string) (h *Host, err error) {
	h = &Host{}

	// Create root dataset
	if rootMountpoint == "" {
		rootMountpoint = DefaultMountpoint
	}

	log.Printf("Creating root ZFS dataset %#v at %v\n", rootDataset, rootMountpoint)
	if ds, err := zfs.CreateFilesystem(
		rootDataset,
		map[string]string{"mountpoint": rootMountpoint},
	); err != nil {
		return nil, errors.Trace(err)
	} else {
		h.Dataset = ds
	}

	log.Println("Creating images dataset")
	if ds, err := h.CreateFilesystem(storageZFSProperties, "images"); err != nil {
		return nil, errors.Trace(err)
	} else {
		h.imagesFS = ds
	}

	log.Println("Creating containers dataset")
	if ds, err := h.CreateFilesystem(nil, "containers"); err != nil {
		return nil, errors.Trace(err)
	} else {
		h.containersFS = ds
	}

	return
}

func (h *Host) dsName(name ...string) string {
	return path.Join(append([]string{h.Name}, name...)...)
}

func (h *Host) getDataset(name ...string) (*zfs.Dataset, error) {
	return zfs.GetDataset(h.dsName(name...))
}

func (h *Host) CreateFilesystem(properties map[string]string, name ...string) (*zfs.Dataset, error) {
	return zfs.CreateFilesystem(h.dsName(name...), properties)
}

func (h *Host) String() string {
	return fmt.Sprintf("Jetpack[%v]", h.Name)
}

func (h *Host) Images() (Images, error) {
	if dss, err := h.imagesFS.Children(1); err != nil {
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
