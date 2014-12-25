package jetpack

import "encoding/json"
import stderrors "errors"
import "io/ioutil"
import "log"
import "os"
import "strconv"
import "strings"
import "time"

import "github.com/juju/errors"

import "github.com/3ofcoins/jetpack/run"
import "github.com/3ofcoins/jetpack/zfs"

const DefaultMountpoint = "/srv/jetpack"

var ErrNotFound = stderrors.New("Not found")
var ErrManyFound = stderrors.New("Multiple results found")

type JailStatus struct {
	Jid   int
	Dying bool
}

var NoJailStatus = JailStatus{}

type Host struct {
	Dataset    *zfs.Dataset `json:"-"`
	Images     ImageManager
	Containers ContainerManager

	jailStatusTimestamp time.Time
	jailStatusCache     map[string]JailStatus
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

func (h *Host) GetJailStatus(name string, refresh bool) (JailStatus, error) {
	if refresh || h.jailStatusCache == nil || time.Now().Sub(h.jailStatusTimestamp) > (2*time.Second) {
		// FIXME: nicer cache/expiry implementation?
		if lines, err := run.Command("/usr/sbin/jls", "-d", "jid", "dying", "name").OutputLines(); err != nil {
			return NoJailStatus, errors.Trace(err)
		} else {
			stat := make(map[string]JailStatus)
			for _, line := range lines {
				fields := strings.SplitN(line, " ", 3)
				status := NoJailStatus
				if len(fields) != 3 {
					return NoJailStatus, errors.Errorf("Cannot parse jls line %#v", line)
				}

				if jid, err := strconv.Atoi(fields[0]); err != nil {
					return NoJailStatus, errors.Annotatef(err, "Cannot parse jls line %#v", line)
				} else {
					status.Jid = jid
				}

				if dying, err := strconv.Atoi(fields[1]); err != nil {
					return NoJailStatus, errors.Annotatef(err, "Cannot parse jls line %#v", line)
				} else {
					status.Dying = (dying != 0)
				}

				stat[fields[2]] = status
			}
			h.jailStatusCache = stat
		}
	}
	return h.jailStatusCache[name], nil
}
