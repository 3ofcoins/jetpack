package jetpack

import "encoding/json"
import "fmt"
import "io/ioutil"
import "os"
import "text/template"

import "github.com/appc/spec/schema"
import "github.com/appc/spec/schema/types"
import "github.com/juju/errors"

var jailConfTmpl *template.Template

func init() {
	tmpl, err := template.New("jail.conf").Parse(
		`"{{.JailName}}" {
  path = "{{.Mountpoint}}/rootfs";
  devfs_ruleset="4";
  exec.clean="true";
  # exec.start="/bin/sh /etc/rc";
  # exec.stop="/bin/sh /etc/rc.shutdown";
  host.hostname="{{(.GetAnnotation "hostname" .Manifest.UUID.String)}}";
  interface="{{.Manager.Interface}}";
  ip4.addr="{{(.GetAnnotation "ip-address" "CAN'T HAPPEN")}}";
  mount.devfs="true";
  persist="true";
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
	Dataset  `json:"-"`
	Manifest schema.ContainerRuntimeManifest `json:"-"`
	Manager  *ContainerManager               `json:"-"`
}

func NewContainer(ds *Dataset, mgr *ContainerManager) *Container {
	return &Container{Dataset: *ds, Manager: mgr}
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
	_, err := os.Stat(c.Path("manifest"))
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
	manifestJSON, err := ioutil.ReadFile(c.Path("manifest"))
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

	err = ioutil.WriteFile(c.Path("manifest"), manifestJSON, 0400)
	if err != nil {
		return errors.Trace(err)
	}

	jc, err := os.OpenFile(c.Path("jail.conf"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0400)
	if err != nil {
		return errors.Trace(err)
	}
	defer jc.Close()
	return errors.Trace(jailConfTmpl.Execute(jc, c))
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

func (c *Container) String() string {
	return fmt.Sprintf("#<Container %v %v>",
		c.Manifest.UUID, c.Manifest.Apps[0].Name)
}

func (c *Container) PPPrepare() interface{} {
	if c == nil {
		return nil
	}
	return map[string]interface{}{
		"Manifest": c.Manifest,
		"Path":     c.Mountpoint,
		"Dataset":  c.Name,
	}
}
