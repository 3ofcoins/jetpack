package jetpack

import "encoding/json"
import "fmt"
import "io/ioutil"
import "os"
import "log"
import "path"
import "path/filepath"

import "github.com/3ofcoins/go-zfs"
import "github.com/juju/errors"

const DefaultMountpoint = "/srv/jetpack"

type Host struct {
	*zfs.Dataset `json:"-"`

	Images       ImageManager
	containersFS *zfs.Dataset `json:"-"`

	// Configuration
	Interface   string
	AddressPool string
}

var hostDefaults = Host{
	Interface:   "lo1",
	AddressPool: "172.23.0.1/16",
}

func GetHost(rootDataset string) (*Host, error) {
	ds, err := zfs.GetDataset(rootDataset)
	if err != nil {
		return nil, errors.Trace(err)
	}
	h := hostDefaults
	h.Dataset = ds

	if ds, err := h.getDataset("images"); err != nil {
		return nil, errors.Trace(err)
	} else {
		h.Images.Dataset = ds
	}

	h.containersFS, err = h.getDataset("containers")
	if err != nil {
		return nil, errors.Trace(err)
	}

	config, err := ioutil.ReadFile(h.configPath())
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("WARN: config not found, saving now")
			if err = h.SaveConfig(); err != nil {
				return nil, errors.Trace(err)
			}
			return &h, nil
		} else {
			return nil, errors.Trace(err)
		}
	}

	err = json.Unmarshal(config, &h)
	if err != nil {
		return nil, err
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
	if ds, err := zfs.CreateFilesystem(
		rootDataset,
		map[string]string{"mountpoint": rootMountpoint},
	); err != nil {
		return nil, errors.Trace(err)
	} else {
		h.Dataset = ds
	}

	if ds, err := h.CreateFilesystem(storageZFSProperties, "images"); err != nil {
		return nil, errors.Trace(err)
	} else {
		h.Images.Dataset = ds
	}

	if ds, err := h.CreateFilesystem(nil, "containers"); err != nil {
		return nil, errors.Trace(err)
	} else {
		h.containersFS = ds
	}

	// TODO: accept configuration
	if err := h.SaveConfig(); err != nil {
		return nil, errors.Trace(err)
	}

	return &h, nil
}

func (h *Host) configPath() string {
	return filepath.Join(h.Mountpoint, "config")
}

func (h *Host) SaveConfig() error {
	config, err := json.Marshal(h)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(h.configPath(), config, 0600)
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
