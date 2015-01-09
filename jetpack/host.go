package jetpack

import stderrors "errors"
import "os"
import "path/filepath"
import "strconv"
import "strings"
import "time"

import "github.com/juju/errors"
import "github.com/magiconair/properties"

import "../run"
import "../zfs"

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

	Properties *properties.Properties

	jailStatusTimestamp time.Time
	jailStatusCache     map[string]JailStatus
}

func NewHost(configPath string) (*Host, error) {
	h := Host{}
	h.Images.Host = &h
	h.Containers.Host = &h
	h.Properties = properties.MustLoadFiles(
		[]string{
			filepath.Join(SharedPath, "jetpack.conf.defaults"),
			configPath,
		},
		properties.UTF8,
		true)

	if ds, err := zfs.GetDataset(h.Properties.MustGetString("root.zfs")); err != nil {
		if err == zfs.ErrNotFound {
			return &h, nil
		}
		return nil, err
	} else {
		h.Dataset = ds
	}

	if ds, err := h.Dataset.GetDataset("images"); err != nil {
		return nil, err
	} else {
		h.Images.Dataset = ds
	}

	if ds, err := h.Dataset.GetDataset("containers"); err != nil {
		return nil, err
	} else {
		h.Containers.Dataset = ds
	}

	return &h, nil
}

func (h *Host) zfsOptions(prefix string, opts ...string) []string {
	l := len(prefix)
	p := h.Properties.FilterPrefix(prefix)
	for _, k := range p.Keys() {
		if v, ok := p.Get(k); ok && v != "" {
			opts = append(opts, strings.Join([]string{"-o", k[l:], "=", v}, ""))
		}
	}
	return opts
}

func (h *Host) Initialize() error {
	if h.Dataset != nil {
		return errors.New("Host already initialized")
	}

	// We use GetString, as user can specify "root.zfs.mountpoint=" (set
	// to empty string) in config to unset property
	if mntpnt := h.Properties.GetString("root.zfs.mountpoint", ""); mntpnt != "" {
		if err := os.MkdirAll(mntpnt, 0755); err != nil {
			return errors.Trace(err)
		}
	}

	if ds, err := zfs.CreateDataset(h.Properties.MustGetString("root.zfs"), h.zfsOptions("root.zfs.", "-p")...); err != nil {
		return errors.Trace(err)
	} else {
		h.Dataset = ds
	}

	if ds, err := h.Dataset.CreateDataset("images", h.zfsOptions("images.zfs.")...); err != nil {
		return errors.Trace(err)
	} else {
		h.Images.Dataset = ds
	}

	if ds, err := h.Dataset.CreateDataset("containers", h.zfsOptions("containers.zfs.")...); err != nil {
		return errors.Trace(err)
	} else {
		h.Containers.Dataset = ds
	}

	return nil
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

func (h *Host) Get(spec interface{}) (interface{}, error) {
	switch c, err := h.Containers.Get(spec); err {
	case nil:
		return c, nil
	case ErrNotFound:
		return h.Images.Get(spec)
	default:
		return nil, err
	}
}

type Destroyable interface {
	Destroy() error
}
