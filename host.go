package zettajail

import "fmt"
import "log"
import "path"
import "sort"

import "github.com/3ofcoins/go-zfs"

type Host struct {
	Dataset
}

var DefaultRootProperties = map[string]string{
	"atime":                        "off",
	"compress":                     "lz4",
	"dedup":                        "on",
	"mountpoint":                   "/srv/zettajail",
	"zettajail:jail":               "no",
	"zettajail:jail:devfs_ruleset": "4",
	"zettajail:jail:exec.clean":    "true",
	"zettajail:jail:exec.start":    "/bin/sh /etc/rc",
	"zettajail:jail:exec.stop":     "/bin/sh /etc/rc.shutdown",
	"zettajail:jail:interface":     "lo1",
	"zettajail:jail:mount.devfs":   "true",
}

func ElucidateDefaultRootDataset() string {
	pools, err := zfs.ListZpools()
	if err != nil {
		log.Fatalln(err)
	}
	if len(pools) == 0 {
		log.Fatalln("No ZFS pools found")
	}
	if len(pools) > 1 {
		log.Fatalln("Multiple pools found, please set ZETTAJAIL_ROOT environment variable or use -root flag")
	}
	return path.Join(pools[0].Name, "zettajail")
}

func NewHost(zfsRootDS string) *Host {
	if zfsRootDS == "" {
		zfsRootDS = ElucidateDefaultRootDataset()
	}

	if ds, err := GetDataset(zfsRootDS); err != nil {
		// go-zfs doesn't let us distinguish between "dataset does not
		// exist" and "there was a horrible error as we tried to get
		// dataset", all we can do here is assume that this is a
		// nonexistent dataset.
		log.Println("ERROR:", err)
		return nil
	} else {
		return &Host{ds}
	}
	return nil
}

func CreateHost(name string, userProperties map[string]string) (*Host, error) {
	properties := make(map[string]string)
	for prop, val := range DefaultRootProperties {
		properties[prop] = val
	}
	for prop, val := range userProperties {
		if prop[0] == '-' {
			delete(properties, prop[1:])
		} else {
			properties[prop] = val
		}
	}

	ds, err := zfs.CreateFilesystem(name, properties)
	if err != nil {
		return nil, err
	}

	return &Host{Dataset{ds}}, nil
}

type jailsByName []*Jail

func (jj jailsByName) Len() int           { return len(jj) }
func (jj jailsByName) Swap(i, j int)      { jj[i], jj[j] = jj[j], jj[i] }
func (jj jailsByName) Less(i, j int) bool { return jj[i].Name < jj[j].Name }

func (h *Host) Jails() []*Jail {
	children, err := h.Dataset.Children(0)
	if err != nil {
		log.Fatalln("ERROR:", err)
	}

	rv := make([]*Jail, 0, len(children))
	for _, child := range children {
		if child.Type == "filesystem" && child.Properties["zettajail:jail"] == "on" {
			jail := NewJail(h, Dataset{child})
			rv = append(rv, jail)
		}
	}

	sort.Sort(jailsByName(rv))
	return rv
}

func (h *Host) GetJail(name string) (*Jail, error) {
	ds, err := GetDataset(path.Join(h.Name, name))
	if err != nil {
		return nil, err
	}
	if ds.Type == "filesystem" && ds.Properties["zettajail:jail"] == "on" {
		return NewJail(h, ds), nil
	} else {
		return nil, fmt.Errorf("Not a jail: %v", ds.Name)
	}
}

func (h *Host) newJailProperties(name string, properties map[string]string) map[string]string {
	if properties == nil {
		properties = make(map[string]string)
	}

	if _, hasMountpoint := properties["mountpoint"]; !hasMountpoint {
		properties["mountpoint"] = path.Join(h.Mountpoint, name, "rootfs")
	}

	if _, hasHostname := properties["zettajail:jail:host.hostname"]; !hasHostname {
		properties["zettajail:jail:host.hostname"] = path.Base(name)
	}

	// Expand default console log
	switch properties["zettajail:jail:exec.consolelog"] {
	case "true":
		properties["zettajail:jail:exec.consolelog"] = properties["mountpoint"] + ".log"
	case "false":
		delete(properties, "zettajail:jail:exec.consolelog")
	}

	properties["zettajail:jail"] = "on"
	return properties
}

func (h *Host) CreateJail(name string, properties map[string]string) (*Jail, error) {
	properties = h.newJailProperties(name, properties)

	ds, err := zfs.CreateFilesystem(path.Join(h.Name, name), properties)
	if err != nil {
		return nil, err
	}
	return NewJail(h, Dataset{ds}), nil
}

func (h *Host) CloneJail(snapshot, name string, properties map[string]string) (*Jail, error) {
	// FIXME: base properties off snapshot's properties, at least for zettajail:*
	properties = h.newJailProperties(name, properties)
	snap, err := zfs.GetDataset(path.Join(h.Name, snapshot))
	if err != nil {
		return nil, err
	}
	ds, err := snap.Clone(path.Join(h.Name, name), properties)
	if err != nil {
		return nil, err
	}
	return NewJail(h, Dataset{ds}), nil
}

func (h *Host) Status() {
	for _, child := range h.Jails() {
		child.Status()
	}
}
