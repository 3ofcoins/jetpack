package jetpack

import "fmt"

import "bytes"
import "encoding/json"
import "io/ioutil"
import "path/filepath"
import "os"
import "os/exec"
import "strconv"
import "strings"
import "syscall"
import "text/template"

import "github.com/appc/spec/schema"
import "github.com/appc/spec/schema/types"
import "github.com/juju/errors"

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

	if len(c.Manifest.Apps) == 0 {
		return errors.New("No application set?")
	}

	if len(c.Manifest.Apps) > 1 {
		return errors.New("Multi-application containers are not supported (yet)")
	}

	if len(c.Manifest.Volumes) != 0 {
		return errors.New("TODO: columes are not supported")
	}

	if len(c.Manifest.Isolators) != 0 || len(c.Manifest.Apps[0].Isolators) != 0 {
		return errors.New("TODO: isolators are not supported")
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

func (c *Container) GetImage() (*Image, error) {
	hash := c.Manifest.Apps[0].ImageID.Val
	if !strings.HasPrefix(hash, "000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000") {
		return nil, errors.New("FIXME: sha512 is a real checksum, not wrapped UUID, and I am confused now.")
	}
	hash = hash[128-32:]
	uuid := strings.Join([]string{hash[:8], hash[8:12], hash[12:16], hash[16:20], hash[20:]}, "-")
	return c.Manager.Host.Images.Get(uuid)
}

func (c *Container) Stage2(app *types.App) error {
	img, err := c.GetImage()
	if err != nil {
		return errors.Trace(err)
	}

	jid := c.Jid()
	if jid == 0 {
		return errors.New("Not started")
	}

	if err := JailAttach(jid); err != nil {
		return errors.Trace(err)
	}

	if err := os.Chdir("/"); err != nil {
		return errors.Trace(err)
	}

	if app == nil {
		app = img.Manifest.App
	}

	if app == nil {
		app = ConsoleApp("root")
	}

	username := app.User
	if username == "" {
		username = "root"
	}

	user, err := getUserData(username)
	if err != nil {
		return errors.Trace(err)
	} else if user == nil {
		return errors.Errorf("User not found: %s", username)
	}

	var gid int
	if app.Group == "" {
		gid = user.gid
	} else {
		agid, err := getGid(app.Group)
		if err != nil {
			return errors.Trace(err)
		} else if agid < 0 {
			return errors.Errorf("Group not found: %s", app.Group)
		}
		gid = agid
	}

	os.Clearenv()

	// Put environment in a map to avoid duplicates when App.Environment
	// overrides one of the default variables

	env := map[string]string{
		"PATH":    "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"USER":    user.username,
		"LOGNAME": user.username,
		"HOME":    user.home,
		"SHELL":   user.shell,
	}

	for k, v := range app.Environment {
		env[k] = v
	}

	env["AC_APP_NAME"] = string(img.Manifest.Name)
	env["AC_METADATA_URL"] = ""

	envv := make([]string, 0, len(env))
	for k, v := range env {
		envv = append(envv, k+"="+v)
	}

	return errors.Trace(untilError(
		func() error { return syscall.Setgroups([]int{}) },
		func() error { return syscall.Setregid(gid, gid) },
		func() error { return syscall.Setreuid(user.uid, user.uid) },
		func() error { return syscall.Exec(app.Exec[0], app.Exec, envv) },
	))
}

type ContainerSlice []*Container

func (ii ContainerSlice) Len() int { return len(ii) }
func (ii ContainerSlice) Less(i, j int) bool {
	return bytes.Compare(ii[i].Manifest.UUID[:], ii[j].Manifest.UUID[:]) < 0
}
func (ii ContainerSlice) Swap(i, j int) { ii[i], ii[j] = ii[j], ii[i] }
