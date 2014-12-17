package jetpack

import "path"

import "code.google.com/p/go-uuid/uuid"
import "github.com/appc/spec/schema"
import "github.com/appc/spec/schema/types"
import "github.com/juju/errors"

type ContainerManager struct {
	Dataset `json:"-"`

	Interface   string
	AddressPool string
}

var defaultContainerManager = ContainerManager{
	Interface:   "lo1",
	AddressPool: "172.23.0.1/16",
}

func (cmgr *ContainerManager) All() ([]*Container, error) {
	if dss, err := cmgr.Children(1); err != nil {
		return nil, errors.Trace(err)
	} else {
		rv := make([]*Container, len(dss))
		for i, ds := range dss {
			if c, err := GetContainer(ds); err != nil {
				return nil, errors.Trace(err)
			} else {
				rv[i] = c
			}
		}
		return rv, nil
	}
}

func (cmgr *ContainerManager) Get(uuid string) (*Container, error) {
	if ds, err := cmgr.GetDataset(uuid); err != nil {
		return nil, err
	} else {
		return GetContainer(ds)
	}
}

func (cmgr *ContainerManager) Clone(img *Image) (*Container, error) {
	ds, err := img.Clone(path.Join(cmgr.Name, uuid.NewRandom().String()))
	if err != nil {
		return nil, errors.Trace(err)
	}

	c := NewContainer(ds)

	uuid, err := types.NewUUID(path.Base(c.Name))
	if err != nil {
		return nil, errors.Trace(err)
	}

	c.Manifest.ACVersion = schema.AppContainerVersion
	c.Manifest.ACKind = types.ACKind("ContainerRuntimeManifest")
	c.Manifest.Annotations = make(map[types.ACName]string)
	c.Manifest.Apps = []schema.RuntimeApp{img.RuntimeApp()}
	c.Manifest.UUID = *uuid

	// c.Manifest.Annotations["ip-address"] = FIXME

	err = c.Save()
	if err != nil {
		return nil, errors.Trace(err)
	}

	return c, nil
}
