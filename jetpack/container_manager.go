package jetpack

import "io/ioutil"
import "net"
import "os"
import "path"
import "path/filepath"
import "syscall"

import "code.google.com/p/go-uuid/uuid"
import "github.com/appc/spec/schema"
import "github.com/appc/spec/schema/types"
import "github.com/juju/errors"

import "github.com/3ofcoins/jetpack/zfs"

type ContainerManager struct {
	Dataset *zfs.Dataset `json:"-"`
	Host    *Host        `json:"-"`
}

func (cmgr *ContainerManager) All() (ContainerSlice, error) {
	if dss, err := cmgr.Dataset.Children(1); err != nil {
		return nil, errors.Trace(err)
	} else {
		rv := make(ContainerSlice, 0, len(dss))
		for _, ds := range dss {
			if ds.Type != "filesystem" {
				continue
			}
			if c, err := GetContainer(ds, cmgr); err != nil {
				if err != ErrContainerIsEmpty {
					// TODO: warn but still return useful containers
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
	if ds, err := cmgr.Dataset.GetDataset(uuid); err != nil {
		return nil, err
	} else {
		return GetContainer(ds, cmgr)
	}
}

func (cmgr *ContainerManager) newContainer(ds *zfs.Dataset) (*Container, error) {
	c := NewContainer(ds, cmgr)

	var resolvConf []byte
	err := untilError(
		func() error { return os.MkdirAll(c.Dataset.Path("rootfs"), 0700) },
		func() error { return os.MkdirAll(c.Dataset.Path("rootfs/etc"), 0755) },
		func() (err error) {
			resolvConf, err = ioutil.ReadFile("/etc/resolv.conf")
			return
		},
		func() error { return ioutil.WriteFile(c.Dataset.Path("rootfs/etc/resolv.conf"), resolvConf, 0644) },
	)
	if err != nil {
		return nil, errors.Trace(err)
	}

	uuid, err := types.NewUUID(path.Base(c.Dataset.Name))
	if err != nil {
		return nil, errors.Trace(err)
	}

	c.Manifest.ACVersion = schema.AppContainerVersion
	c.Manifest.ACKind = types.ACKind("ContainerRuntimeManifest")
	c.Manifest.Annotations = make(map[types.ACName]string)
	c.Manifest.UUID = *uuid

	// TODO: lock until saved?
	if ip, err := cmgr.nextIP(); err != nil {
		return nil, errors.Trace(err)
	} else {
		c.Manifest.Annotations["ip-address"] = ip.String()
	}

	return c, nil
}

func (cmgr *ContainerManager) Create() (*Container, error) {
	ds, err := cmgr.Dataset.CreateDataset(uuid.NewRandom().String())
	if err != nil {
		return nil, errors.Trace(err)
	}

	c, err := cmgr.newContainer(ds)
	if err != nil {
		return nil, errors.Trace(err)
	}

	if err := c.Save(); err != nil {
		return nil, errors.Trace(err)
	}

	return c, nil
}

func (cmgr *ContainerManager) Clone(img *Image) (*Container, error) {
	ds, err := img.Clone("seal", path.Join(cmgr.Dataset.Name, uuid.NewRandom().String()))
	if err != nil {
		return nil, errors.Trace(err)
	}

	vols := []types.Volume{}

	if img.Manifest.App != nil {
		for _, mnt := range img.Manifest.App.MountPoints {
			// TODO: host volumes
			sourcePath := filepath.Join(ds.Mountpoint, "volumes", string(mnt.Name))
			targetPath := filepath.Join(ds.Mountpoint, "rootfs", mnt.Path)

			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return nil, errors.Trace(err)
			}
			// FIXME: we should handle the situation when volume names are "nested". UUIDs? Hash the name?
			if err := os.MkdirAll(sourcePath, 0700); err != nil {
				return nil, errors.Trace(err)
			}

			// Initialize empty volume with same permissions as directory in image.
			if fi, err := os.Stat(targetPath); err != nil {
				return nil, errors.Trace(err)
			} else {
				st := fi.Sys().(*syscall.Stat_t)
				if err := os.Chmod(sourcePath, fi.Mode().Perm()); err != nil {
					return nil, errors.Trace(err)
				}
				if err := os.Chown(sourcePath, int(st.Uid), int(st.Gid)); err != nil {
					return nil, errors.Trace(err)
				}
			}

			vols = append(vols, types.Volume{
				Kind:     "empty",
				Fulfills: []types.ACName{mnt.Name},
				Source:   sourcePath,
				ReadOnly: mnt.ReadOnly,
			})
		}
	}

	c, err := cmgr.newContainer(ds)
	if err != nil {
		return nil, errors.Trace(err)
	}

	c.image = img
	c.Manifest.Apps = []schema.RuntimeApp{img.RuntimeApp()}
	c.Manifest.Volumes = vols

	err = c.Save()
	if err != nil {
		return nil, errors.Trace(err)
	}

	return c, nil
}

func (cmgr *ContainerManager) nextIP() (net.IP, error) {
	if ifi, err := net.InterfaceByName(cmgr.Host.Config["jail/interface"]); err != nil {
		return nil, errors.Trace(err)
	} else {
		if addrs, err := ifi.Addrs(); err != nil {
			return nil, errors.Trace(err)
		} else {
			if ip, ipnet, err := net.ParseCIDR(addrs[0].String()); err != nil {
				return nil, errors.Trace(err)
			} else {
				ips := make(map[string]bool)
				if cc, err := cmgr.All(); err != nil {
					return nil, errors.Trace(err)
				} else {
					for _, c := range cc {
						ips[c.Manifest.Annotations["ip-address"]] = true
					}
				}

				for ip = nextIP(ip); ip != nil && ips[ip.String()]; ip = nextIP(ip) {
				}

				if ip == nil {
					return nil, errors.New("Out of IPs")
				}

				if ipnet.Contains(ip) {
					return ip, nil
				} else {
					return nil, errors.New("Out of IPs")
				}
			}
		}
	}
}
