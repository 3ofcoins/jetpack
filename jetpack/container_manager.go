package jetpack

import "net"
import "path"

import "code.google.com/p/go-uuid/uuid"
import "github.com/appc/spec/schema"
import "github.com/appc/spec/schema/types"
import "github.com/juju/errors"

// import "github.com/tgulacsi/go-locking"

type ContainerManager struct {
	Dataset `json:"-"`

	Interface      string
	AddressPool    string
	JailNamePrefix string
}

var defaultContainerManager = ContainerManager{
	Interface:      "lo1",
	AddressPool:    "172.23.0.1/16",
	JailNamePrefix: "jetpack:",
}

func (cmgr *ContainerManager) All() ([]*Container, error) {
	if dss, err := cmgr.Children(1); err != nil {
		return nil, errors.Trace(err)
	} else {
		rv := make([]*Container, 0, len(dss))
		for _, ds := range dss {
			if c, err := GetContainer(ds, cmgr); err != nil {
				if err != ErrContainerIsEmpty {
					return nil, errors.Trace(err)
				}
			} else {
				rv = append(rv, c)
			}
		}
		return rv, nil
	}
}

func (cmgr *ContainerManager) Get(uuid string) (*Container, error) {
	if ds, err := cmgr.GetDataset(uuid); err != nil {
		return nil, err
	} else {
		return GetContainer(ds, cmgr)
	}
}

func (cmgr *ContainerManager) Clone(img *Image) (*Container, error) {
	ds, err := img.Clone(path.Join(cmgr.Name, uuid.NewRandom().String()))
	if err != nil {
		return nil, errors.Trace(err)
	}

	c := NewContainer(ds, cmgr)

	uuid, err := types.NewUUID(path.Base(c.Name))
	if err != nil {
		return nil, errors.Trace(err)
	}

	c.Manifest.ACVersion = schema.AppContainerVersion
	c.Manifest.ACKind = types.ACKind("ContainerRuntimeManifest")
	c.Manifest.Annotations = make(map[types.ACName]string)
	c.Manifest.Apps = []schema.RuntimeApp{img.RuntimeApp()}
	c.Manifest.UUID = *uuid

	// TODO: lock, defer unlock
	c.Manifest.Annotations["ip-address"] = cmgr.NextIP().String()

	err = c.Save()
	if err != nil {
		return nil, errors.Trace(err)
	}

	return c, nil
}

func (cmgr *ContainerManager) NextIP() net.IP {
	ip, ipnet, err := net.ParseCIDR(cmgr.AddressPool)
	if err != nil {
		panic(err)
	}

	ips := make(map[string]bool)
	if cc, err := cmgr.All(); err != nil {
		panic(err)
	} else {
		for _, c := range cc {
			ips[c.Manifest.Annotations["ip-address"]] = true
		}
	}

	for ip = nextIP(ip); ips[ip.String()]; ip = nextIP(ip) {
	}

	if ipnet.Contains(ip) {
		return ip
	} else {
		return nil
	}
}
