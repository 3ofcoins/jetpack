package zfs

import "bytes"
import stderrors "errors"
import "fmt"
import "io"
import "os"
import "os/exec"
import "strings"

import "github.com/juju/errors"

type CommandError struct {
	Wrapped error
	Command string
	Args    []string
	Stderr  string
}

func (err *CommandError) Error() string {
	return fmt.Sprintf("%v%v: %v (stderr: %#v)",
		err.Command,
		err.Args,
		err.Wrapped.Error(),
		err.Stderr)
}

func runCommand(stdout io.Writer, stdin io.Reader, command string, arg ...string) error {
	var stderr bytes.Buffer
	if stdout == nil {
		stdout = os.Stdout
	}
	if stdin == nil {
		stdin = os.Stdin
	}
	cmd := exec.Command(command, arg...)
	cmd.Stdout = stdout
	cmd.Stderr = &stderr
	cmd.Stdin = stdin
	if err := cmd.Run(); err != nil {
		return &CommandError{errors.Trace(err), command, arg, stderr.String()}
	}
	return nil
}

func RunCommand(cmd string, arg ...string) error {
	return runCommand(os.Stdout, nil, cmd, arg...)
}

func RunCommandOutput(cmd string, arg ...string) (string, error) {
	var stdout bytes.Buffer
	err := runCommand(&stdout, nil, cmd, arg...)
	return stdout.String(), err
}

func lines(in string) []string {
	return strings.Split(strings.TrimRight(in, "\n"), "\n")
}

func linefields(in string) []string {
	return strings.Split(in, "\t")
}

func fields(in string) [][]string {
	lns := lines(in)
	rv := make([][]string, len(lns))
	for i, ln := range lns {
		rv[i] = linefields(ln)
	}
	return rv
}

func ListZPools() ([]string, error) {
	if out, err := RunCommandOutput("/sbin/zpool", "list", "-H", "-oname"); err != nil {
		return nil, errors.Trace(err)
	} else {
		return lines(out), nil
	}
}

func Zfs(args ...string) error {
	return RunCommand("/sbin/zfs", args...)
}

func Receive(in io.Reader, args ...string) error {
	return runCommand(nil, in, "/sbin/zfs", append([]string{"receive"}, args...)...)
}

func Send(out io.Writer, args ...string) error {
	return runCommand(out, nil, "/sbin/zfs", append([]string{"send"}, args...)...)
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

var ErrNotFound = stderrors.New("Not found")

func ListDatasets(typ string) ([]string, error) {
	if typ == "" {
		typ = "all"
	}
	if out, err := RunCommandOutput("/sbin/zfs", "list", "-H", "-t"+typ, "-oname"); err != nil {
		return nil, errors.Trace(err)
	} else {
		return lines(out), nil
	}
}

func (ds *Dataset) load() error {
	if props, err := ds.Get("type", "mounted", "mountpoint"); err != nil {
		return errors.Trace(err)
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
			// Can't list datasets, assume original error was not "not found"
			return nil, errors.Trace(err)
		} else {
			for _, ds := range dss {
				if ds == name {
					return nil, errors.Trace(err)
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
	if err := Receive(r, args...); err != nil {
		return nil, errors.Trace(err)
	}
	return GetDataset(name)
}

func runZfsFields(cmd ...string) ([][]string, error) {
	if out, err := RunCommandOutput("/sbin/zfs", cmd...); err != nil {
		return nil, errors.Trace(err)
	} else {
		return fields(out), nil
	}
}

func (ds *Dataset) Get(attr ...string) (map[string]string, error) {
	cmd := []string{"get", "-Hp", "-oproperty,value"}

	if len(attr) > 0 && strings.HasPrefix(attr[0], "-s") {
		cmd = append(cmd, attr[0])
		attr = attr[1:]
	}

	if len(attr) == 0 {
		cmd = append(cmd, "all")
	} else {
		cmd = append(cmd, strings.Join(attr, ","))
	}

	cmd = append(cmd, ds.Name)

	if lines, err := runZfsFields(cmd...); err != nil {
		return nil, errors.Trace(err)
	} else {
		rv := make(map[string]string)
		for _, line := range lines {
			rv[line[0]] = line[1]
		}
		return rv, nil
	}
}

func (ds *Dataset) Get1(name string) (string, error) {
	if props, err := ds.Get(name); err != nil {
		return "", errors.Trace(err)
	} else {
		return props[name], nil
	}
}

func (ds *Dataset) Zfs(cmd ...string) error {
	cmd = append(cmd, ds.Name)
	return Zfs(cmd...)
}

func (ds *Dataset) Set(attr map[string]string) (err error) {
	reload := false
	cmd := make([]string, 1, len(attr)+2)
	cmd[0] = "set"
	for k, v := range attr {
		if k == "mountpoint" {
			reload = true
		}
		cmd = append(cmd, k+"="+v)
	}
	cmd = append(cmd, ds.Name)
	if reload {
		defer func() {
			if err == nil {
				err = errors.Trace(ds.load())
			}
		}()
	}
	return errors.Trace(Zfs(cmd...))
}

func (ds *Dataset) Set1(name, value string) error {
	return ds.Set(map[string]string{name: value})
}

func (ds *Dataset) Snapshot(name string, args ...string) (*Dataset, error) {
	snapName := ds.Name + "@" + name
	if err := Zfs(append(append([]string{"snapshot"}, args...), snapName)...); err != nil {
		return nil, errors.Trace(err)
	} else {
		return GetDataset(snapName)
	}
}

func (ds *Dataset) Send(w io.Writer, args ...string) error {
	if ds.Type != "snapshot" {
		return errors.Errorf("Not a snapshot: %v", ds)
	}
	return Send(w, append(args, ds.Name)...)
}

func (ds *Dataset) Rollback(args ...string) error {
	if ds.Type != "snapshot" {
		return errors.Errorf("Not a snapshot: %v", ds)
	}
	return ds.Zfs(append([]string{"rollback"}, args...)...)
}

func (ds *Dataset) Mount() error {
	if err := ds.Zfs("mount"); err != nil {
		return errors.Trace(err)
	} else {
		return errors.Trace(ds.load())
	}
}

// depth: -1: self and all descendants (unlimited recursion); 0: only
// all descendants (unlimited recursion); >0: set depth, not include
// self
func (ds *Dataset) Children(depth int, args ...string) ([]*Dataset, error) {
	cmd := []string{"list", "-r", "-H", "-oname"}
	if depth > 0 {
		cmd[1] = fmt.Sprintf("-d%d", depth)
	}
	cmd = append(cmd, args...)
	cmd = append(cmd, ds.Name)
	if lines, err := runZfsFields(cmd...); err != nil {
		return nil, errors.Trace(err)
	} else {
		rv := make([]*Dataset, len(lines))
		for i, ln := range lines {
			// TODO: use "zfs get" to get all children at the same time and avoid I/O & forking
			if ds, err := GetDataset(ln[0]); err != nil {
				return nil, errors.Trace(err)
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

func (ds *Dataset) GetSnapshot(name string) (*Dataset, error) {
	return GetDataset(ds.Name + "@" + name)
}

func (ds *Dataset) Destroy(flags string) error {
	args := []string{"destroy"}
	if flags != "" {
		args = append(args, flags)
	}
	args = append(args, ds.Name)
	return Zfs(args...)
}
