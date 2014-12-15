package jetpack

import "fmt"
import "log"
import "path"
import "path/filepath"

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
	if ds, err := h.createFilesystem(storageZFSProperties, "images"); err != nil {
		return nil, errors.Trace(err)
	} else {
		h.imagesFS = ds
	}

	log.Println("Creating containers dataset")
	if ds, err := h.createFilesystem(nil, "containers"); err != nil {
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

func (h *Host) createFilesystem(properties map[string]string, name ...string) (*zfs.Dataset, error) {
	return zfs.CreateFilesystem(h.dsName(name...), properties)
}

func (h *Host) String() string {
	return fmt.Sprintf("Jetpack[%v]", h.Name)
}

func (h *Host) ImportACI(aci *ACI) error {
	ds, err := h.createFilesystem(
		map[string]string{
			"mountpoint": filepath.Join(
				h.imagesFS.Mountpoint,
				aci.Checksum64(),
				"rootfs",
			),
		},
		"images", aci.Checksum64())
	if err != nil {
		return errors.Trace(err)
	}

	props := map[string]string{
		"jetpack:image":          aci.String(),
		"jetpack:image:checksum": aci.Checksum(),
		"jetpack:image:name":     string(aci.Name),
		"jetpack:image:app":      bool2zfs(aci.IsApp()),
	}
	for _, label := range aci.Labels {
		props[fmt.Sprintf("jetpack:image:label:%v", label.Name)] = label.Value
	}
	for annName, annValue := range aci.Annotations {
		props[fmt.Sprintf("jetpack:image:annotation:%v", annName)] = annValue
	}

	for propName, propValue := range props {
		if err := ds.SetProperty(propName, propValue); err != nil {
			return errors.Trace(err)
		}
	}

	err = runCommand("tar", "-C", filepath.Dir(ds.Mountpoint), "-xf", aci.Path)
	if err != nil {
		return errors.Trace(err)
	}

	_, err = ds.Snapshot("image", false)
	if err != nil {
		return errors.Trace(err)
	}

	for propName, propValue := range map[string]string{"canmount": "off", "readonly": "on"} {
		if err := ds.SetProperty(propName, propValue); err != nil {
			return errors.Trace(err)
		}
	}

	return nil
}
