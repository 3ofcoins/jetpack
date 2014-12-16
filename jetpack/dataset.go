package jetpack

import "fmt"
import "path"
import "path/filepath"

import "github.com/juju/errors"

import "github.com/3ofcoins/go-zfs"

func getZpool() (*zfs.Zpool, error) {
	if pools, err := zfs.ListZpools(); err != nil {
		return nil, errors.Trace(err)
	} else {
		switch len(pools) {
		case 0:
			return nil, errors.New("No ZFS pools found")
		case 1:
			return pools[0], nil
		default:
			return nil, errors.New("Multiple ZFS pools found")
		}
	}
}

type Dataset struct {
	zfs.Dataset
}

func GetDataset(name string) (*Dataset, error) {
	if ds, err := zfs.GetDataset(name); err != nil {
		return nil, err
	} else {
		return &Dataset{*ds}, nil
	}
}

func CreateFilesystem(name string, properties map[string]string) (*Dataset, error) {
	if ds, err := zfs.CreateFilesystem(name, properties); err != nil {
		return nil, err
	} else {
		return &Dataset{*ds}, nil
	}
}

func (ds *Dataset) String() string {
	return fmt.Sprintf("#<ZFS %v %v>", ds.Type, ds.Name)
}

func (ds *Dataset) Path(filename string) string {
	return filepath.Join(ds.Mountpoint, filename)
}

func (ds *Dataset) ChildName(name string) string {
	return path.Join(ds.Name, name)
}

func (ds *Dataset) GetDataset(name string) (*Dataset, error) {
	return GetDataset(ds.ChildName(name))
}

func (ds *Dataset) CreateFilesystem(name string, properties map[string]string) (*Dataset, error) {
	return CreateFilesystem(ds.ChildName(name), properties)
}

func (ds *Dataset) GetSnapshot(name string) (*Dataset, error) {
	return GetDataset(ds.Name + "@" + name)
}

func (ds *Dataset) Clone(name string, properties map[string]string) (*Dataset, error) {
	if ds, err := ds.Dataset.Clone(name, properties); err != nil {
		return nil, err
	} else {
		return &Dataset{*ds}, nil
	}
}

func (ds *Dataset) Snapshot(name string, recursive bool) (*Dataset, error) {
	if ds, err := ds.Dataset.Snapshot(name, recursive); err != nil {
		return nil, err
	} else {
		return &Dataset{*ds}, nil
	}
}

func (ds *Dataset) Children(depth uint64) ([]*Dataset, error) {
	if zchildren, err := ds.Dataset.Children(depth); err != nil {
		return nil, err
	} else {
		rv := make([]*Dataset, len(zchildren))
		for i, zchild := range zchildren {
			rv[i] = &Dataset{*zchild}
		}
		return rv, nil
	}
}
