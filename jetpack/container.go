package jetpack

import "encoding/json"
import "fmt"
import "io/ioutil"
import "path/filepath"
import "os"
import "os/exec"
import "strconv"
import "strings"
import "text/template"

import "github.com/appc/spec/schema"
import "github.com/appc/spec/schema/types"
import "github.com/juju/errors"

import "github.com/3ofcoins/jetpack/ui"

var jailConfTmpl *template.Template

func init() {
	tmpl, err := template.New("jail.conf").Parse(
		`"{{.JailName}}" {
  path = "{{.Dataset.Mountpoint}}/rootfs";
  devfs_ruleset="4";
  exec.clean="true";
  # exec.start="/bin/sh /etc/rc";
  # exec.stop="/bin/sh /etc/rc.shutdown";
  host.hostname="{{(.GetAnnotation "hostname" .Manifest.UUID.String)}}";
  host.hostuuid="{{.Manifest.UUID}}";
  interface="{{.Manager.Interface}}";
  ip4.addr="{{(.GetAnnotation "ip-address" "CAN'T HAPPEN")}}";
  mount.devfs="true";
  persist="true";
{{ range $param, $value := .JailParameters }}
  {{$param}} = "{{$value}}";
{{ end }}
}
`)
	if err != nil {
		panic(err)
	} else {
		jailConfTmpl = tmpl
	}
}

var ErrContainerIsEmpty = errors.New("Container is empty")

type Container struct {
	Dataset  *Dataset                        `json:"-"`
	Manifest schema.ContainerRuntimeManifest `json:"-"`
	Manager  *ContainerManager               `json:"-"`

	JailParameters map[string]string
}

func NewContainer(ds *Dataset, mgr *ContainerManager) *Container {
	return &Container{Dataset: ds, Manager: mgr, JailParameters: make(map[string]string)}
}

func GetContainer(ds *Dataset, mgr *ContainerManager) (*Container, error) {
	c := NewContainer(ds, mgr)
	if err := c.Load(); err != nil {
		return nil, err
	} else {
		return c, nil
	}
}

func (c *Container) IsEmpty() bool {
	_, err := os.Stat(c.Dataset.Path("manifest"))
	return os.IsNotExist(err)
}

func (c *Container) IsLoaded() bool {
	return !c.Manifest.ACVersion.Empty()
}

func (c *Container) Load() error {
	if c.IsLoaded() {
		return errors.New("Already loaded")
	}

	if c.IsEmpty() {
		return ErrContainerIsEmpty
	}

	if err := c.readManifest(); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (c *Container) readManifest() error {
	manifestJSON, err := ioutil.ReadFile(c.Dataset.Path("manifest"))
	if err != nil {
		return errors.Trace(err)
	}

	if err = json.Unmarshal(manifestJSON, &c.Manifest); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (c *Container) Save() error {
	manifestJSON, err := json.Marshal(c.Manifest)
	if err != nil {
		return errors.Trace(err)
	}
	return errors.Trace(ioutil.WriteFile(c.Dataset.Path("manifest"), manifestJSON, 0400))
}

func (c *Container) Prep() error {
	jc, err := os.OpenFile(c.Dataset.Path("jail.conf"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0400)
	if err != nil {
		return errors.Trace(err)
	}
	defer jc.Close()

	err = jailConfTmpl.Execute(jc, c)
	if err != nil {
		return errors.Trace(err)
	}

	if bb, err := ioutil.ReadFile("/etc/resolv.conf"); err != nil {
		return errors.Trace(err)
	} else {
		return errors.Trace(
			ioutil.WriteFile(filepath.Join(c.Dataset.Mountpoint, "rootfs/etc/resolv.conf"), bb, 0644))
	}
}

func (c *Container) GetAnnotation(key, defval string) string {
	if val, ok := c.Manifest.Annotations[types.ACName(key)]; ok {
		return val
	} else {
		return defval
	}
}

func (c *Container) JailName() string {
	return c.Manager.JailNamePrefix + c.Manifest.UUID.String()
}

func (c *Container) Summary() string {
	started := " "
	if c.Jid() > 0 {
		started = "*"
	}
	name := " (anonymous)"
	if len(c.Manifest.Apps) > 0 {
		name = " " + string(c.Manifest.Apps[0].Name)
	}
	return fmt.Sprintf("%v%v%v", started, c.Manifest.UUID, name)
}

func (c *Container) Show(ui *ui.UI) {
	ui.RawShow(c)
	ui.Sayf(".JID: %d", c.Jid())
}

func (c *Container) Jid() int {
	cmd := exec.Command("jls", "-j", c.JailName(), "jid")
	out, err := cmd.Output()
	switch err.(type) {
	case nil:
		// Jail found
		jid, err := strconv.Atoi(strings.TrimSpace(string(out)))
		if err != nil {
			panic(err)
		}
		return jid
	case *exec.ExitError:
		// Jail not found (or so we assume)
		return 0
	default:
		// Other error
		panic(err)
	}
}

func (c *Container) RunJail(op string) error {
	if err := c.Prep(); err != nil {
		return err
	}
	return runCommand("jail", "-f", c.Dataset.Path("jail.conf"), "-v", op, c.JailName())
}

func (c *Container) RunJexec(user string, jcmd []string) error {
	if c.Jid() == 0 {
		return errors.New("Not started")
	}

	args := []string{}
	if user != "" {
		args = append(args, "-U", user)
	}
	args = append(args, c.JailName())
	args = append(args, jcmd...)

	return runCommand("jexec", args...)
}
