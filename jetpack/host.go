package jetpack

import stderrors "errors"
import "fmt"
import "strconv"
import "strings"
import "time"

import "github.com/juju/errors"

import "github.com/3ofcoins/jetpack/config"
import "github.com/3ofcoins/jetpack/run"
import "github.com/3ofcoins/jetpack/zfs"

var ErrNotFound = stderrors.New("Not found")
var ErrManyFound = stderrors.New("Multiple results found")

type JailStatus struct {
	Jid   int
	Dying bool
}

var NoJailStatus = JailStatus{}

var DefaultConfig = config.Config{
	"jail/interface":   "lo1",
	"jail/name-prefix": "jetpack:",
}

type Host struct {
	Dataset    *zfs.Dataset `json:"-"`
	Images     ImageManager
	Containers ContainerManager

	Config config.Config

	jailStatusTimestamp time.Time
	jailStatusCache     map[string]JailStatus
}

func newHost() *Host {
	h := Host{Config: config.NewConfig()}
	h.Images.Host = &h
	h.Containers.Host = &h
	h.Config.UpdateFrom(DefaultConfig, "")
	return &h
}

func GetHost(rootDataset string) (*Host, error) {
	h := newHost()

	if ds, err := zfs.GetDataset(rootDataset); err != nil {
		return nil, errors.Trace(err)
	} else {
		h.Dataset = ds
	}

	if ds, err := h.Dataset.GetDataset("images"); err != nil {
		return nil, errors.Trace(err)
	} else {
		h.Images.Dataset = ds
	}

	if ds, err := h.Dataset.GetDataset("containers"); err != nil {
		return nil, errors.Trace(err)
	} else {
		h.Containers.Dataset = ds
	}

	if err := h.Config.LoadOrSave(h.Dataset.Path("config"), 0600); err != nil {
		return nil, errors.Trace(err)
	}

	return h, nil
}

func CreateHost(rootDataset string, cfg config.Config) (*Host, error) {
	zfsConfig := cfg.ExtractSubtree("zfs")
	zfsOptions := make([]string, 0, len(zfsConfig))
	for k, v := range zfsConfig {
		zfsOptions = append(zfsOptions, fmt.Sprintf("-o%v=%v", k, v))
	}

	h := newHost()

	if ds, err := zfs.CreateDataset(rootDataset, zfsOptions...); err != nil {
		return nil, errors.Trace(err)
	} else {
		h.Dataset = ds
	}

	if ds, err := h.Dataset.CreateDataset("images", "-oatime=off", "-ocompress=lz4", "-odedup=on"); err != nil {
		return nil, errors.Trace(err)
	} else {
		h.Images.Dataset = ds
	}

	if ds, err := h.Dataset.CreateDataset("containers"); err != nil {
		return nil, errors.Trace(err)
	} else {
		h.Containers.Dataset = ds
	}

	if err := h.UpdateConfig(cfg); err != nil {
		return nil, errors.Trace(err)
	}

	return h, nil
}

func (h *Host) UpdateConfig(c config.Config) error {
	h.Config.UpdateFrom(c, "")
	return errors.Trace(h.Config.SaveToFile(h.Dataset.Path("config"), 0600))
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
