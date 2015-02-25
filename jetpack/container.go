package jetpack

import "bytes"
import "encoding/json"
import "fmt"
import "io/ioutil"
import "os"
import "path"
import "path/filepath"
import "sort"
import "strconv"
import "strings"
import "time"

import "code.google.com/p/go-uuid/uuid"
import "github.com/appc/spec/schema"
import "github.com/appc/spec/schema/types"
import "github.com/juju/errors"

import "../run"
import "../zfs"

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
	UUID     uuid.UUID
	Host     *Host
	Manifest schema.ContainerRuntimeManifest

	image *Image
}

func NewContainer(h *Host, id uuid.UUID) *Container {
	if id == nil {
		panic("Container UUID can't be nil!")
	}
	c := &Container{Host: h, UUID: id, Manifest: *schema.BlankContainerRuntimeManifest()}
	if mid, err := types.NewUUID(id.String()); err != nil {
		// CAN'T HAPPEN
		panic(err)
	} else {
		c.Manifest.UUID = *mid
	}
	return c
}

func (c *Container) Path(elem ...string) string {
	return c.Host.Path(append(
		[]string{"containers", c.UUID.String()},
		elem...,
	)...)
}

func (c *Container) Exists() bool {
	if _, err := os.Stat(c.Path("manifest")); err != nil {
		if os.IsNotExist(err) {
			return false
		}
		panic(err)
	}
	return true
}

func (c *Container) Load() error {
	if !c.Exists() {
		return ErrNotFound
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
	return errors.Trace(ioutil.WriteFile(c.Path("manifest"), manifestJSON, 0400))
}

func (c *Container) findVolume(name types.ACName) *types.Volume {
	for _, vol := range c.Manifest.Volumes {
		if vol.Name == name {
			return &vol
		}
	}
	return nil
}

func (c *Container) jailConf() string {
	parameters := map[string]string{
		"devfs_ruleset": "4",
		"exec.clean":    "true",
		"host.hostuuid": c.Manifest.UUID.String(),
		"interface":     c.Host.Properties.MustGetString("jail.interface"),
		"mount.devfs":   "true",
		"path":          c.Path("rootfs"),
		"persist":       "true",
	}

	if hostname, ok := c.Manifest.Annotations.Get("hostname"); ok {
		parameters["host.hostname"] = hostname
	} else {
		parameters["host.hostname"] = parameters["host.hostuuid"]
	}

	if ip, ok := c.Manifest.Annotations.Get("ip-address"); ok {
		parameters["ip4.addr"] = ip
	} else {
		panic(fmt.Sprintf("No IP address for container %v", c.Manifest.UUID))
	}

	for _, antn := range c.Manifest.Annotations {
		if strings.HasPrefix(string(antn.Name), "jetpack/jail.conf/") {
			parameters[strings.Replace(string(antn.Name)[len("jetpack/jail.conf/"):], "-", "_", -1)] = antn.Value
		}
	}

	lines := make([]string, 0, len(parameters))
	for k, v := range parameters {
		lines = append(lines, fmt.Sprintf("  %v=%#v;", k, v))
	}
	sort.Strings(lines)

	return fmt.Sprintf("%#v {\n%v\n}\n", c.jailName(), strings.Join(lines, "\n"))
}

func (c *Container) prepJail() error {
	img, err := c.GetImage()
	if err != nil {
		return errors.Trace(err)
	}

	var fstab []string
	if app := img.Manifest.App; app != nil && len(app.MountPoints) > 0 {
		for _, mnt := range app.MountPoints {
			if vol := c.findVolume(mnt.Name); vol == nil {
				return errors.Errorf("No volume found for %v", mnt.Name)
			} else if vol.Kind == "host" {
				opts := "rw"
				if vol.ReadOnly {
					opts = "ro"
				}
				fstab = append(fstab, fmt.Sprintf("%v %v nullfs %v 0 0\n",
					vol.Source,
					c.Path(filepath.Join("rootfs", mnt.Path)),
					opts,
				))
			}
		}
	}
	if os, _ := img.Manifest.GetLabel("os"); os == "linux" {
		fstab = append(fstab,
			fmt.Sprintf("linsys %v linsysfs  rw 0 0\n", c.Path("rootfs/sys")),
			fmt.Sprintf("linproc %v linprocfs rw 0 0\n", c.Path("rootfs/proc")),
		)
	}
	if len(fstab) > 0 {
		fstabPath := c.Path("fstab")
		if err := ioutil.WriteFile(fstabPath, []byte(strings.Join(fstab, "")), 0600); err != nil {
			return errors.Trace(err)
		}
		c.Manifest.Annotations.Set("jetpack/jail.conf/mount.fstab", fstabPath)
	}

	if bb, err := ioutil.ReadFile("/etc/resolv.conf"); err != nil {
		return errors.Trace(err)
	} else {
		if err := ioutil.WriteFile(c.Path("rootfs/etc/resolv.conf"), bb, 0644); err != nil {
			return errors.Trace(err)
		}
	}

	return errors.Trace(
		ioutil.WriteFile(c.Path("jail.conf"), []byte(c.jailConf()), 0400))
}

func (c *Container) Status() ContainerStatus {
	if status, err := c.jailStatus(false); err != nil {
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
	if err := c.prepJail(); err != nil {
		return err
	}
	verbosity := "-q"
	if c.Host.Properties.GetBool("debug", false) {
		verbosity = "-v"
	}
	return run.Command("jail", "-f", c.Path("jail.conf"), verbosity, op, c.jailName()).Run()
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

// FIXME: multi-app containers
func (c *Container) getDataset() *zfs.Dataset {
	ds, err := c.Host.Dataset.GetDataset(path.Join("containers", c.UUID.String()))
	if err != nil {
		panic(err)
	}
	return ds
}

func (c *Container) Destroy() error {
	if err := c.getDataset().Destroy("-r"); err != nil {
		return errors.Trace(err)
	}
	return errors.Trace(os.RemoveAll(c.Path()))
}

func (c *Container) jailName() string {
	return c.Host.Properties.MustGetString("jail.namePrefix") + c.Manifest.UUID.String()
}

func (c *Container) jailStatus(refresh bool) (JailStatus, error) {
	return c.Host.GetJailStatus(c.jailName(), refresh)
}

func (c *Container) Jid() int {
	if status, err := c.jailStatus(false); err != nil {
		panic(err) // do we need to?
	} else {
		return status.Jid
	}
}

func (c *Container) imageUUID() uuid.UUID {
	return uuid.Parse(strings.Split(path.Base(c.getDataset().Origin), "@")[0])
}

func (c *Container) GetImage() (*Image, error) {
	if c.image == nil {
		if img, err := c.Host.GetImage(c.imageUUID()); err != nil {
			return nil, errors.Trace(err)
		} else {
			c.image = img
		}
	}
	return c.image, nil
}

func (c *Container) Run(app *types.App) (err1 error) {
	if err := errors.Trace(c.runJail("-c")); err != nil {
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

	if app.WorkingDirectory != "" {
		args = append(args, "-cwd", app.WorkingDirectory)
	}

	for _, env_var := range app.Environment {
		args = append(args, "-setenv", env_var.Name+"="+env_var.Value)
	}

	args = append(args, app.Exec...)

	return run.Command(filepath.Join(LibexecPath, "stage2"), args...).Run()
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
		ipAddress, _ := c.Manifest.Annotations.Get("ip-address")
		rows[i+1] = []string{
			c.Manifest.UUID.String(),
			imageID,
			appName,
			ipAddress,
			c.Status().String(),
		}
	}
	return rows
}

func ContainerRuntimeManifest(imgs []*Image) *schema.ContainerRuntimeManifest {
	if len(imgs) != 1 {
		panic("FIXME: only one-image manifests are supported")
	}
	crm := schema.BlankContainerRuntimeManifest()

	if id, err := types.NewUUID(uuid.NewRandom().String()); err != nil {
		panic(err)
	} else {
		crm.UUID = *id
	}

	crm.Apps = make([]schema.RuntimeApp, len(imgs))
	for i, img := range imgs {
		crm.Apps[i] = img.RuntimeApp()
		if img.Manifest.App != nil {
			for _, mnt := range img.Manifest.App.MountPoints {
				crm.Volumes = append(crm.Volumes, types.Volume{
					Kind:     "empty",
					Name:     mnt.Name,
					ReadOnly: mnt.ReadOnly,
				})
			}
		}
	}

	return crm
}
