package jetpack

import "fmt"
import "log"
import "path"
import "sort"

import "github.com/3ofcoins/go-zfs"

type Host struct {
	Dataset
}

var DefaultRootProperties = map[string]string{
	"atime":                      "off",
	"compress":                   "lz4",
	"dedup":                      "on",
	"mountpoint":                 "/srv/jetpack",
	"jetpack:jail":               "no",
	"jetpack:jail:devfs_ruleset": "4",
	"jetpack:jail:exec.clean":    "true",
	"jetpack:jail:exec.start":    "/bin/sh /etc/rc",
	"jetpack:jail:exec.stop":     "/bin/sh /etc/rc.shutdown",
	"jetpack:jail:interface":     "lo1",
	"jetpack:jail:mount.devfs":   "true",
}

func ElucidateDefaultRootDataset() string {
	//DONE	pools, err := zfs.ListZpools()
	//DONE	if err != nil {
	//DONE		log.Fatalln(err)
	//DONE	}
	//DONE	if len(pools) == 0 {
	//DONE		log.Fatalln("No ZFS pools found")
	//DONE	}
	//DONE	if len(pools) > 1 {
	//DONE		log.Fatalln("Multiple pools found, please set IL_ROOT environment variable or use -root flag")
	//DONE	}
	//DONE	return path.Join(pools[0].Name, "jetpack")
	return ""
}

func NewHost(zfsRootDS string) *Host {
	//DONE	if zfsRootDS == "" {
	//DONE		zfsRootDS = ElucidateDefaultRootDataset()
	//DONE	}
	//DONE
	//DONE	if ds, err := GetDataset(zfsRootDS); err != nil {
	//DONE		// go-zfs doesn't let us distinguish between "dataset does not
	//DONE		// exist" and "there was a horrible error as we tried to get
	//DONE		// dataset", all we can do here is assume that this is a
	//DONE		// nonexistent dataset.
	//DONE		log.Println("ERROR:", err)
	//DONE		return nil
	//DONE	} else {
	//DONE		return &Host{ds}
	//DONE	}
	//DONE	return nil
}

func CreateHost(name string, userProperties map[string]string) (*Host, error) {
	//DONE 	properties := make(map[string]string)
	//DONE 	for prop, val := range DefaultRootProperties {
	//DONE 		properties[prop] = val
	//DONE 	}
	//DONE 	for prop, val := range userProperties {
	//DONE 		if prop[0] == '-' {
	//DONE 			delete(properties, prop[1:])
	//DONE 		} else {
	//DONE 			properties[prop] = val
	//DONE 		}
	//DONE 	}
	//DONE
	//DONE 	ds, err := zfs.CreateFilesystem(name, properties)
	//DONE 	if err != nil {
	//DONE 		return nil, err
	//DONE 	}
	//DONE
	//DONE 	return &Host{Dataset{ds}}, nil
	return nil, nil
}

//IRRELEVANT func (h *Host) CreateFolder(name string, properties map[string]string) (*Host, error) {
//IRRELEVANT 	ds, err := zfs.CreateFilesystem(path.Join(h.Name, name), properties)
//IRRELEVANT 	if err != nil {
//IRRELEVANT 		return nil, err
//IRRELEVANT 	}
//IRRELEVANT 	return &Host{Dataset{ds}}, nil
//IRRELEVANT }
//IRRELEVANT
//IRRELEVANT func (h *Host) GetFolder(name string) (*Host, error) {
//IRRELEVANT 	ds, err := GetDataset(path.Join(h.Name, name))
//IRRELEVANT 	if err != nil {
//IRRELEVANT 		return nil, err
//IRRELEVANT 	}
//IRRELEVANT 	return &Host{ds}, nil
//IRRELEVANT }

type jailsByName []*Jail

func (jj jailsByName) Len() int           { return len(jj) }
func (jj jailsByName) Swap(i, j int)      { jj[i], jj[j] = jj[j], jj[i] }
func (jj jailsByName) Less(i, j int) bool { return jj[i].Name < jj[j].Name }

func (h *Host) Jails() []*Jail {
	//DONE 	children, err := h.Dataset.Children(0)
	//DONE 	if err != nil {
	//DONE 		log.Fatalln("ERROR:", err)
	//DONE 	}
	//DONE
	//DONE 	rv := make([]*Jail, 0, len(children))
	//DONE 	for _, child := range children {
	//DONE 		if child.Type == "filesystem" && child.Properties["jetpack:jail"] == "on" {
	//DONE 			jail := NewJail(h, Dataset{child})
	//DONE 			rv = append(rv, jail)
	//DONE 		}
	//DONE 	}
	//DONE
	//DONE 	sort.Sort(jailsByName(rv))
	//DONE 	return rv
	return nil
}

func (h *Host) GetJail(name string) (*Jail, error) {
	//DONE 	ds, err := GetDataset(path.Join(h.Name, name))
	//DONE 	if err != nil {
	//DONE 		return nil, err
	//DONE 	}
	//DONE 	if ds.Type == "filesystem" && ds.Properties["jetpack:jail"] == "on" {
	//DONE 		return NewJail(h, ds), nil
	//DONE 	} else {
	//DONE 		return nil, fmt.Errorf("Not a jail: %v", ds.Name)
	//DONE 	}
	return nil, nil
}

func (h *Host) newJailProperties(name string, properties map[string]string) map[string]string {
	//IRRELEVANT 	if properties == nil {
	//IRRELEVANT 		properties = make(map[string]string)
	//IRRELEVANT 	}
	//IRRELEVANT
	//IRRELEVANT 	if _, hasMountpoint := properties["mountpoint"]; !hasMountpoint {
	//IRRELEVANT 		properties["mountpoint"] = path.Join(h.Mountpoint, name, "rootfs")
	//IRRELEVANT 	}
	//IRRELEVANT
	//IRRELEVANT 	if _, hasHostname := properties["jetpack:jail:host.hostname"]; !hasHostname {
	//IRRELEVANT 		properties["jetpack:jail:host.hostname"] = path.Base(name)
	//IRRELEVANT 	}
	//IRRELEVANT
	//IRRELEVANT 	// Expand default console log
	//IRRELEVANT 	switch properties["jetpack:jail:exec.consolelog"] {
	//IRRELEVANT 	case "true":
	//IRRELEVANT 		properties["jetpack:jail:exec.consolelog"] = properties["mountpoint"] + ".log"
	//IRRELEVANT 	case "false":
	//IRRELEVANT 		delete(properties, "jetpack:jail:exec.consolelog")
	//IRRELEVANT 	}
	//IRRELEVANT
	//IRRELEVANT 	properties["jetpack:jail"] = "on"
	//IRRELEVANT 	return properties
	return nil
}

func (h *Host) CreateJail(name string, properties map[string]string) (*Jail, error) {
	//DONE 	properties = h.newJailProperties(name, properties)
	//DONE
	//DONE 	ds, err := zfs.CreateFilesystem(path.Join(h.Name, name), properties)
	//DONE 	if err != nil {
	//DONE 		return nil, err
	//DONE 	}
	//DONE 	return NewJail(h, Dataset{ds}), nil
	return nil, nil
}

func (h *Host) CloneJail(snapshot, name string, properties map[string]string) (*Jail, error) {
	//DONE 	// FIXME: base properties off snapshot's properties, at least for jetpack:*
	//DONE 	properties = h.newJailProperties(name, properties)
	//DONE 	snap, err := zfs.GetDataset(path.Join(h.Name, snapshot))
	//DONE 	if err != nil {
	//DONE 		return nil, err
	//DONE 	}
	//DONE 	ds, err := snap.Clone(path.Join(h.Name, name), properties)
	//DONE 	if err != nil {
	//DONE 		return nil, err
	//DONE 	}
	//DONE 	return NewJail(h, Dataset{ds}), nil
	return nil
}
