package jetpack

import "encoding/json"
import stderrors "errors"
import "io/ioutil"
import "os"
import "log"

import "github.com/juju/errors"

const DefaultMountpoint = "/srv/jetpack"

var ErrNotFound = stderrors.New("Not found")

type Host struct {
	Dataset    *Dataset `json:"-"`
	Images     ImageManager
	Containers ContainerManager
}

var hostDefaults = Host{
	Containers: defaultContainerManager,
}

func GetHost(rootDataset string) (*Host, error) {
	ds, err := GetDataset(rootDataset)
	if err != nil {
		return nil, errors.Trace(err)
	}
	h := hostDefaults
	h.Dataset = ds

	if config, err := ioutil.ReadFile(h.Dataset.Path("config")); err != nil {
		if os.IsNotExist(err) {
			log.Println("WARN: config not found, saving now")
			if err = h.SaveConfig(); err != nil {
				return nil, errors.Trace(err)
			}
			return &h, nil
		} else {
			return nil, errors.Trace(err)
		}
	} else {
		err = json.Unmarshal(config, &h)
		if err != nil {
			return nil, err
		}
	}

	h.Images.Host = &h
	if ds, err := h.Dataset.GetDataset("images"); err != nil {
		return nil, errors.Trace(err)
	} else {
		h.Images.Dataset = ds
	}

	h.Containers.Host = &h
	if ds, err := h.Dataset.GetDataset("containers"); err != nil {
		return nil, errors.Trace(err)
	} else {
		h.Containers.Dataset = ds
	}

	return &h, nil
}

var storageZFSProperties = map[string]string{
	"atime":    "off",
	"compress": "lz4",
	"dedup":    "on",
}

func CreateHost(rootDataset, rootMountpoint string) (*Host, error) {
	h := hostDefaults

	// Create root dataset
	if rootMountpoint == "" {
		rootMountpoint = DefaultMountpoint
	}

	log.Printf("Creating root ZFS dataset %#v at %v\n", rootDataset, rootMountpoint)
	if ds, err := CreateFilesystem(
		rootDataset,
		map[string]string{"mountpoint": rootMountpoint},
	); err != nil {
		return nil, errors.Trace(err)
	} else {
		h.Dataset = ds
	}

	if ds, err := h.Dataset.CreateFilesystem("images", storageZFSProperties); err != nil {
		return nil, errors.Trace(err)
	} else {
		h.Images.Dataset = ds
	}

	if ds, err := h.Dataset.CreateFilesystem("containers", nil); err != nil {
		return nil, errors.Trace(err)
	} else {
		h.Containers.Dataset = ds
	}

	// TODO: accept configuration
	if err := h.SaveConfig(); err != nil {
		return nil, errors.Trace(err)
	}

	return &h, nil
}

func (h *Host) SaveConfig() error {
	config, err := json.Marshal(h)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(h.Dataset.Path("config"), config, 0600)
}
