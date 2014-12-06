package zettajail

import "fmt"
import "io"
import "log"
import "os"
import "path"
import "sort"

import "github.com/3ofcoins/go-zfs"

type Host struct {
	Dataset
	JidCache map[string]int
}

var RootProperties = map[string]string{
	"atime":                        "off",
	"compress":                     "lz4",
	"dedup":                        "on",
	"zettajail:jail":               "no",
	"zettajail:jail:devfs_ruleset": "4",
	"zettajail:jail:exec.clean":    "true",
	"zettajail:jail:exec.start":    "/bin/sh /etc/rc",
	"zettajail:jail:exec.stop":     "/bin/sh /etc/rc.shutdown",
	"zettajail:jail:interface":     "lo1",
	"zettajail:jail:mount.devfs":   "true",
}

func NewHost(zfsRootDS string) *Host {
	if zfsRootDS == "" {
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
		zfsRootDS = path.Join(pools[0].Name, "zettajail")
	}

	if ds, err := GetDataset(zfsRootDS); err != nil {
		log.Fatalln(err)
	} else {
		return &Host{Dataset: ds}
	}
	return nil
}

func (h *Host) Init(properties map[string]string) error {
	if h.Exists() {
		if err := h.SetProperties(RootProperties); err != nil {
			return err
		}
		if len(properties) > 0 {
			if err := h.SetProperties(properties); err != nil {
				return err
			}
		}
		return h.WriteJailConf()
	} else {
		log.Fatalln("NFY: creating root dataset")
	}
	return nil
}

func (h *Host) Jid(name string) int {
	if h.JidCache == nil {
		h.JidCache = Jls()
	}
	return h.JidCache[name]
}

type jailsByName []Jail

func (jj jailsByName) Len() int           { return len(jj) }
func (jj jailsByName) Swap(i, j int)      { jj[i], jj[j] = jj[j], jj[i] }
func (jj jailsByName) Less(i, j int) bool { return jj[i].Name < jj[j].Name }

func (h *Host) Jails() []Jail {
	children, err := h.Dataset.Children(0)
	if err != nil {
		log.Fatalln("ERROR:", err)
	}

	rv := make([]Jail, 0, len(children))
	for _, child := range children {
		if child.Type == "filesystem" && child.Properties["zettajail:jail"] == "on" {
			jail := Jail{Dataset{child}, h}
			rv = append(rv, jail)
		}
	}

	sort.Sort(jailsByName(rv))
	return rv
}

func (h *Host) GetJail(name string) (Jail, error) {
	ds, err := GetDataset(path.Join(h.Name, name))
	if err != nil {
		return ZeroJail, err
	}
	if ds.Type == "filesystem" && ds.Properties["zettajail:jail"] == "on" {
		return Jail{ds, h}, nil
	} else {
		return ZeroJail, fmt.Errorf("Not a jail: %v", ds.Name)
	}
}

func (h *Host) newJailProperties(name string, properties map[string]string) map[string]string {
	if properties == nil {
		properties = make(map[string]string)
	}

	// Set mountpoint to have a reference path later on
	if _, hasMountpoint := properties["mountpoint"]; !hasMountpoint {
		properties["mountpoint"] = path.Join(h.Mountpoint, name)
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

func (h *Host) CreateJail(name string, properties map[string]string) (Jail, error) {
	properties = h.newJailProperties(name, properties)

	ds, err := zfs.CreateFilesystem(path.Join(h.Name, name), properties)
	if err != nil {
		return ZeroJail, err
	}
	return Jail{Dataset{ds}, h}, h.WriteJailConf()
}

func (h *Host) CloneJail(snapshot, name string, properties map[string]string) (Jail, error) {
	// FIXME: base properties off snapshot's properties, at least for zettajail:*
	properties = h.newJailProperties(name, properties)
	snap, err := zfs.GetDataset(path.Join(h.Name, snapshot))
	if err != nil {
		return ZeroJail, err
	}
	ds, err := snap.Clone(path.Join(h.Name, name), properties)
	if err != nil {
		return ZeroJail, err
	}
	return Jail{Dataset{ds}, h}, h.WriteJailConf()
}

func (h *Host) Status() {
	for _, child := range h.Jails() {
		child.Status()
	}
}

func (h *Host) WriteConfigTo(w io.Writer) error {
	for _, child := range h.Jails() {
		if err := child.WriteConfigTo(w); err != nil {
			return err
		}
	}

	if f, err := os.Open("/etc/jail.conf.local"); err != nil {
		if !os.IsNotExist(err) {
			log.Println("WARNING:", err)
		}
	} else {
		if _, err := io.Copy(w, f); err != nil {
			return err
		}
	}

	return nil
}

func (h *Host) WriteJailConf() error {
	jailconf, err := os.OpenFile("/etc/.jail.conf.new", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer jailconf.Close()

	err = h.WriteConfigTo(jailconf)
	if err != nil {
		return err
	}
	jailconf.Close()

	err = os.Rename("/etc/jail.conf", "/etc/jail.conf~")
	if err != nil {
		return err
	}

	return os.Rename("/etc/.jail.conf.new", "/etc/jail.conf")
}
