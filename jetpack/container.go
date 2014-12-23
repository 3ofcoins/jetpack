package jetpack

import "bytes"
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

import "github.com/3ofcoins/go-zfs"

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
	ContainerStatusStopped
)

var containerStatusNames = []string{
	ContainerStatusInvalid: "invalid",
	ContainerStatusRunning: "running",
	ContainerStatusStopped: "stopped",
}

func (cs ContainerStatus) String() string {
	if int(cs) < len(containerStatusNames) {
		return containerStatusNames[cs]
	}
	return fmt.Sprintf("ContainerStatus[%d]", cs)
}

type Container struct {
	Dataset  *Dataset                        `json:"-"`
	Manifest schema.ContainerRuntimeManifest `json:"-"`
	Manager  *ContainerManager               `json:"-"`

	JailParameters map[string]string

	image *Image
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

	if img != nil && img.Manifest.App != nil && len(img.Manifest.App.MountPoints) > 0 {
		fstab := make([]string, len(img.Manifest.App.MountPoints))
		for i, mnt := range img.Manifest.App.MountPoints {
			if vol := c.findVolume(mnt.Name); vol == nil {
				return errors.Errorf("No volume found for %v", mnt.Name)
			} else {
				opts := "rw"
				if vol.ReadOnly {
					opts = "ro"
				}
				fstab[i] = fmt.Sprintf("%v %v nullfs %v 0 0\n",
					vol.Source,
					filepath.Join(c.Dataset.Mountpoint, "rootfs", mnt.Path),
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
		if err := ioutil.WriteFile(filepath.Join(c.Dataset.Mountpoint, "rootfs/etc/resolv.conf"), bb, 0644); err != nil {
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
	if c.Jid() > 0 {
		return ContainerStatusRunning
	} else {
		return ContainerStatusStopped
	}
}

func (c *Container) Destroy() error {
	return c.Dataset.Destroy(zfs.DestroyRecursive)
}

func (c *Container) JailName() string {
	return c.Manager.JailNamePrefix + c.Manifest.UUID.String()
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

func (c *Container) GetImage() (*Image, error) {
	if c.image == nil {
		if len(c.Manifest.Apps) == 0 {
			// FIXME: Empty container (another argument for import/build split)
			return nil, nil
		}
		hash := c.Manifest.Apps[0].ImageID.Val
		if !strings.HasPrefix(hash, "000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000") {
			return nil, errors.New("FIXME: sha512 is a real checksum, not wrapped UUID, and I am confused now.")
		}
		hash = hash[128-32:]
		uuid := strings.Join([]string{hash[:8], hash[8:12], hash[12:16], hash[16:20], hash[20:]}, "-")
		if img, err := c.Manager.Host.Images.Get(uuid); err != nil {
			return nil, errors.Trace(err)
		} else {
			c.image = img
		}
	}
	return c.image, nil
}

func (c *Container) Run(app *types.App) (err1 error) {
	if err := c.RunJail("-c"); err != nil {
		return errors.Trace(err)
	}
	defer func() {
		if err := c.RunJail("-r"); err != nil {
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

	// FIXME: no-image app
	name := fmt.Sprintf(".%v", c.Manifest.UUID)
	if img != nil {
		name = string(img.Manifest.Name)
	}

	args := []string{
		"-jid", strconv.Itoa(jid),
		"-user", user,
		"-group", app.Group,
		"-name", name,
	}

	for k, v := range app.Environment {
		args = append(args, "-setenv", k+"="+v)
	}

	args = append(args, app.Exec...)

	// FIXME:libexec
	return runCommand("/home/japhy/Go/src/github.com/3ofcoins/jetpack/bin/stage2", args...)
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
