package zfs

import "errors"
import "fmt"
import "io"
import "strings"

import "github.com/3ofcoins/jetpack/run"

const zfsBin = "/sbin/zfs"

func zfs(command string, args []string) *run.Cmd {
	return run.Command(zfsBin, append([]string{command}, args...)...)
}

func zfsHp(command string, args []string) *run.Cmd {
	return zfs(command, append([]string{"-Hp"}, args...))
}

func Zfs(cmd string, args ...string) error {
	return zfs(cmd, args).Run()
}

func ZfsOutput(cmd string, args ...string) (string, error) {
	return zfsHp(cmd, args).OutputString()
}

func ZfsLines(cmd string, args ...string) ([]string, error) {
	return zfsHp(cmd, args).OutputLines()
}

func ZfsFields(cmd string, args ...string) ([][]string, error) {
	if lines, err := ZfsLines(cmd, args...); err != nil {
		return nil, err
	} else {
		rv := make([][]string, len(lines))
		for i, line := range lines {
			rv[i] = strings.Split(line, "\t")
		}
		return rv, nil
	}
}

func ZfsReceive(r io.Reader, args ...string) error {
	return zfs("receive", args).ReadFrom(r).Run()
}

func ZfsSend(w io.Writer, args ...string) error {
	return zfs("send", args).WriteTo(w).Run()
}

type Dataset struct {
	Name       string
	Type       string
	Mounted    bool
	Mountpoint string
}

func (ds *Dataset) String() string {
	pieces := []string{"ZFS", ds.Type, ds.Name, "unmounted"}
	if ds.Mounted {
		pieces[3] = "mounted"
	}
	if ds.Mountpoint != "" {
		pieces = append(pieces, "at", ds.Mountpoint)
	}
	return strings.Join(pieces, " ")
}

func (ds *Dataset) Zfs(cmd string, args ...string) error {
	args = append(args, ds.Name)
	return Zfs(cmd, args...)
}

func (ds *Dataset) ZfsOutput(cmd string, args ...string) (string, error) {
	args = append(args, ds.Name)
	return ZfsOutput(cmd, args...)
}

func (ds *Dataset) ZfsFields(cmd string, args ...string) ([][]string, error) {
	args = append(args, ds.Name)
	return ZfsFields(cmd, args...)
}

var ErrNotFound = errors.New("Not found")

func ListDatasets(typ string) ([]string, error) {
	if typ == "" {
		typ = "all"
	}
	return ZfsLines("list", "-H", "-t"+typ, "-oname")
}

func (ds *Dataset) load() error {
	if props, err := ds.GetMany("type", "mounted", "mountpoint"); err != nil {
		return err
	} else {
		ds.Type = props["type"]
		ds.Mountpoint = props["mountpoint"]
		ds.Mounted = (props["mounted"] == "yes")
		return nil
	}
}

func GetDataset(name string) (*Dataset, error) {
	ds := &Dataset{Name: name}
	if err := ds.load(); err != nil {
		// Check if dataset exists
		if dss, err2 := ListDatasets(""); err2 != nil {
			// Can't list datasets, assume original error was not "not
			// found" and return it rather than the new one
			return nil, err
		} else {
			for _, ds := range dss {
				if ds == name {
					// Dataset exists, the error was legit after all
					return nil, err
				}
			}
			return nil, ErrNotFound
		}
	}
	return ds, nil
}

func ReceiveDataset(r io.Reader, name string, mounted bool) (*Dataset, error) {
	var args []string
	if mounted {
		args = []string{name}
	} else {
		args = []string{"-u", name}
	}
	if err := ZfsReceive(r, args...); err != nil {
		return nil, err
	}
	return GetDataset(name)
}

func (ds *Dataset) GetMany(attr ...string) (map[string]string, error) {
	attrArg := "all"
	if len(attr) > 0 {
		attrArg = strings.Join(attr, ",")
	}

	if lines, err := ds.ZfsFields("get", "-oproperty,value", attrArg); err != nil {
		return nil, err
	} else {
		rv := make(map[string]string)
		for _, line := range lines {
			rv[line[0]] = line[1]
		}
		return rv, nil
	}
}

func (ds *Dataset) Get(name string) (string, error) {
	return ds.ZfsOutput("get", "-oproperty", name)
}

func firstError(err1, err2 error) error {
	if err1 == nil {
		return err2
	}
	return err1
}

func (ds *Dataset) SetMany(attr map[string]string) (err error) {
	reload := false
	args := make([]string, 0, len(attr))
	for k, v := range attr {
		if k == "mountpoint" {
			reload = true
		}
		args = append(args, k+"="+v)
	}
	if reload {
		defer func() {
			err = firstError(err, ds.load())
		}()
	}
	return ds.Zfs("set", args...)
}

func (ds *Dataset) Set(name, value string) (err error) {
	if name == "mountpoint" {
		defer func() {
			err = firstError(err, ds.load())
		}()
	}
	return ds.Zfs("set", name+"="+value)
}

func (ds *Dataset) Snapshot(name string, args ...string) (*Dataset, error) {
	snapName := ds.Name + "@" + name
	if err := Zfs("snapshot", append(args, snapName)...); err != nil {
		return nil, err
	} else {
		return GetDataset(snapName)
	}
}

func (ds *Dataset) GetSnapshot(name string) (*Dataset, error) {
	return GetDataset(ds.Name + "@" + name)
}

func (ds *Dataset) RollbackTo(name string, args ...string) error {
	if snap, err := ds.GetSnapshot(name); err != nil {
		return err
	} else {
		return snap.Rollback(args...)
	}
}

func (ds *Dataset) Send(w io.Writer, args ...string) error {
	if ds.Type != "snapshot" {
		return fmt.Errorf("Not a snapshot: %v", ds)
	}
	return ZfsSend(w, append(args, ds.Name)...)
}

func (ds *Dataset) Rollback(args ...string) error {
	if ds.Type != "snapshot" {
		return fmt.Errorf("Not a snapshot: %v", ds)
	}
	return ds.Zfs("rollback", args...)
}

func (ds *Dataset) Mount() (err error) {
	defer func() {
		err = firstError(err, ds.load())
	}()
	return ds.Zfs("mount")
}

// depth: -1: self and all descendants (unlimited recursion); 0: only
// all descendants (unlimited recursion); >0: set depth, not include
// self
func (ds *Dataset) Children(depth int, args ...string) ([]*Dataset, error) {
	args = append([]string{"-r", "-oname"}, args...)
	if depth > 0 {
		args[0] = fmt.Sprintf("-d%d", depth)
	}
	if lines, err := ds.ZfsFields("list", args...); err != nil {
		return nil, err
	} else {
		rv := make([]*Dataset, len(lines))
		for i, ln := range lines {
			// TODO: use "zfs get" to get data of all children at the same
			// time and excessive forking
			if ds, err := GetDataset(ln[0]); err != nil {
				return nil, err
			} else {
				rv[i] = ds
			}
		}
		if depth < 0 {
			return rv, nil
		} else {
			return rv[1:], nil
		}

	}
}

func (ds *Dataset) Destroy(flags ...string) error {
	return ds.Zfs("destroy", flags...)
}
