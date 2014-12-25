package jetpack

import "bytes"
import "encoding/json"
import "fmt"
import "io/ioutil"
import "os"
import "path"
import "strconv"
import "strings"
import "text/template"
import "time"

import "github.com/appc/spec/schema"
import "github.com/appc/spec/schema/types"
import "github.com/juju/errors"

import "github.com/3ofcoins/jetpack/run"
import "github.com/3ofcoins/jetpack/zfs"

var jailConfTmpl *template.Template

func init() {
	tmpl, err := template.New("jail.conf").Parse(
		`"{{.JailName}}" {
  path = "{{.Dataset.Mountpoint}}/rootfs";
  devfs_ruleset="4";
  exec.clean="true";
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

type ContainerStatus uint

const (
	ContainerStatusInvalid ContainerStatus = iota
	ContainerStatusRunning
	ContainerStatusDying
	ContainerStatusStopped
)

var containerStatusNames = []string{
	ContainerStatusInvalid: "invalid",
	ContainerStatusRunning: "running",
	ContainerStatusDying:   "dying",
	ContainerStatusStopped: "stopped",
}

func (cs ContainerStatus) String() string {
	if int(cs) < len(containerStatusNames) {
		return containerStatusNames[cs]
	}
	return fmt.Sprintf("ContainerStatus[%d]", cs)
}

type Container struct {
	Dataset  *zfs.Dataset                    `json:"-"`
	Manifest schema.ContainerRuntimeManifest `json:"-"`
	Manager  *ContainerManager               `json:"-"`

	JailParameters map[string]string

	image *Image
}

func NewContainer(ds *zfs.Dataset, mgr *ContainerManager) *Container {
	return &Container{Dataset: ds, Manager: mgr, JailParameters: make(map[string]string)}
}

func GetContainer(ds *zfs.Dataset, mgr *ContainerManager) (*Container, error) {
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

	if len(c.Manifest.Apps) == 0 {
		return errors.Errorf("No application set?")
	}

	if len(c.Manifest.Apps) > 1 {
		return errors.Errorf("TODO: Multi-application containers are not supported")
	}

	if len(c.Manifest.Isolators) != 0 || len(c.Manifest.Apps[0].Isolators) != 0 {
		return errors.Errorf("TODO: isolators are not supported")
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

func (c *Container) findVolume(name types.ACName) *types.Volume {
	for _, vol := range c.Manifest.Volumes {
		for _, fulfills := range vol.Fulfills {
			if fulfills == name {
				return &vol
			}
		}
	}
	return nil
}

func (c *Container) Prep() error {
	img, err := c.GetImage()
	if err != nil {
		return errors.Trace(err)
	}

	if app := img.Manifest.App; app != nil && len(app.MountPoints) > 0 {
		fstab := make([]string, len(app.MountPoints))
		for i, mnt := range app.MountPoints {
			if vol := c.findVolume(mnt.Name); vol == nil {
				return errors.Errorf("No volume found for %v", mnt.Name)
			} else {
				opts := "rw"
				if vol.ReadOnly {
					opts = "ro"
				}
				fstab[i] = fmt.Sprintf("%v %v nullfs %v 0 0\n",
					vol.Source,
					c.Dataset.Path("rootfs", mnt.Path),
					opts,
				)
			}
		}
		fstabPath := c.Dataset.Path("fstab")
		if err := ioutil.WriteFile(fstabPath, []byte(strings.Join(fstab, "")), 0600); err != nil {
			return errors.Trace(err)
		}
		c.JailParameters["mount.fstab"] = fstabPath
	}

	if bb, err := ioutil.ReadFile("/etc/resolv.conf"); err != nil {
		return errors.Trace(err)
	} else {
		if err := ioutil.WriteFile(c.Dataset.Path("rootfs/etc/resolv.conf"), bb, 0644); err != nil {
			return errors.Trace(err)
		}
	}

	jc, err := os.OpenFile(c.Dataset.Path("jail.conf"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0400)
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

func (c *Container) Status() ContainerStatus {
	if status, err := c.GetJailStatus(false); err != nil {
		panic(err)
	} else {
		if status == NoJailStatus {
			return ContainerStatusStopped
		}
		if status.Dying {
			return ContainerStatusDying
		}
		return ContainerStatusRunning
	}
}

func (c *Container) runJail(op string) error {
	if err := c.Prep(); err != nil {
		return err
	}
	return run.Command("jail", "-f", c.Dataset.Path("jail.conf"), "-v", op, c.JailName()).Run()
}

func (c *Container) Spawn() error {
	return errors.Trace(c.runJail("-c"))
}

func (c *Container) Kill() error {
	t0 := time.Now()
retry:
	switch status := c.Status(); status {
	case ContainerStatusStopped:
		// All's fine
		return nil
	case ContainerStatusRunning:
		if err := c.runJail("-r"); err != nil {
			return errors.Trace(err)
		}
		goto retry
	case ContainerStatusDying:
		// TODO: UI? Log?
		fmt.Printf("Container dying since %v, waiting...\n", time.Now().Sub(t0))
		time.Sleep(2500 * time.Millisecond)
		goto retry
	default:
		return errors.Errorf("Container is %v, I am confused", status)
	}
}

func (c *Container) Destroy() error {
	return c.Dataset.Destroy("-r")
}

func (c *Container) JailName() string {
	return c.Manager.JailNamePrefix + c.Manifest.UUID.String()
}

func (c *Container) GetJailStatus(refresh bool) (JailStatus, error) {
	return c.Manager.Host.GetJailStatus(c.JailName(), refresh)
}

func (c *Container) Jid() int {
	if status, err := c.GetJailStatus(false); err != nil {
		panic(err) // do we need to?
	} else {
		return status.Jid
	}
}

func (c *Container) imageUUID() string {
	return strings.Split(path.Base(c.Dataset.Origin), "@")[0]
}

func (c *Container) GetImage() (*Image, error) {
	if c.image == nil {
		if img, err := c.Manager.Host.Images.Get(c.imageUUID()); err != nil {
			return nil, errors.Trace(err)
		} else {
			c.image = img
		}
	}
	return c.image, nil
}

func (c *Container) Run(app *types.App) (err1 error) {
	if err := c.Spawn(); err != nil {
		return errors.Trace(err)
	}
	defer func() {
		if err := c.Kill(); err != nil {
			if err1 != nil {
				err1 = errors.Wrap(err1, errors.Trace(err))
			} else {
				err1 = errors.Trace(err)
			}
		}
	}()
	return c.Stage2(app)
}

func (c *Container) Stage2(app *types.App) error {
	img, err := c.GetImage()
	if err != nil {
		return errors.Trace(err)
	}

	if app == nil {
		app = img.GetApp()
	}

	jid := c.Jid()
	if jid == 0 {
		return errors.New("Not started")
	}

	user := app.User
	if user == "" {
		user = "root"
	}

	args := []string{
		"-jid", strconv.Itoa(jid),
		"-user", user,
		"-group", app.Group,
		"-name", string(img.Manifest.Name),
	}

	for k, v := range app.Environment {
		args = append(args, "-setenv", k+"="+v)
	}

	args = append(args, app.Exec...)

	// FIXME:libexec
	return run.Command("/home/japhy/Go/src/github.com/3ofcoins/jetpack/bin/stage2", args...).Run()
}

type ContainerSlice []*Container

func (cc ContainerSlice) Len() int { return len(cc) }
func (cc ContainerSlice) Less(i, j int) bool {
	return bytes.Compare(cc[i].Manifest.UUID[:], cc[j].Manifest.UUID[:]) < 0
}
func (cc ContainerSlice) Swap(i, j int) { cc[i], cc[j] = cc[j], cc[i] }

func (cc ContainerSlice) Table() [][]string {
	rows := make([][]string, len(cc)+1)
	rows[0] = []string{"UUID", "IMAGE", "APP", "IP", "STATUS"}
	for i, c := range cc {
		imageID := ""
		if img, err := c.GetImage(); err != nil {
			imageID = fmt.Sprintf("[%v]", err)
		} else {
			imageID = img.UUID.String()
		}

		appName := ""
		if len(c.Manifest.Apps) > 0 {
			appName = string(c.Manifest.Apps[0].Name)
		}
		rows[i+1] = []string{
			c.Manifest.UUID.String(),
			imageID,
			appName,
			c.GetAnnotation("ip-address", ""),
			c.Status().String(),
		}
	}
	return rows
}
