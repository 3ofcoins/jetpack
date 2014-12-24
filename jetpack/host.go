package jetpack

import "encoding/json"
import stderrors "errors"
import "io/ioutil"
import "log"
import "os"

import "github.com/juju/errors"

import "github.com/3ofcoins/jetpack/zfs"

const DefaultMountpoint = "/srv/jetpack"

var ErrNotFound = stderrors.New("Not found")
var ErrManyFound = stderrors.New("Multiple results found")

type Host struct {
	Dataset    *zfs.Dataset `json:"-"`
	Images     ImageManager
	Containers ContainerManager
}

var hostDefaults = Host{
	Containers: defaultContainerManager,
}

func GetHost(rootDataset string) (*Host, error) {
	ds, err := zfs.GetDataset(rootDataset)
	if err != nil {
		return nil, errors.Trace(err)
	}
	h := hostDefaults
	h.Dataset = ds

	if config, err := ioutil.ReadFile(h.Dataset.Path("config")); err != nil {
		if os.IsNotExist(err) {
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

func CreateHost(rootDataset, rootMountpoint string) (*Host, error) {
	h := hostDefaults

	// Create root dataset
	if rootMountpoint == "" {
		rootMountpoint = DefaultMountpoint
	}

	log.Printf("Creating root ZFS dataset %#v at %v\n", rootDataset, rootMountpoint)
	if ds, err := zfs.CreateDataset(rootDataset, "-omountpoint="+rootMountpoint); err != nil {
		return nil, errors.Trace(err)
	} else {
		h.Dataset = ds
	}

	if ds, err := h.Dataset.CreateDataset("images",
		"-oatime=off",
		"-ocompress=lz4",
		"-odedup=on",
	); err != nil {
		return nil, errors.Trace(err)
	} else {
		h.Images.Dataset = ds
	}

	if ds, err := h.Dataset.CreateDataset("containers"); err != nil {
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
