package jetpack

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"code.google.com/p/go-uuid/uuid"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/juju/errors"

	"lib/passwd"
	"lib/run"
	"lib/ui"
	"lib/zfs"
)

type PodStatus uint

const (
	PodStatusInvalid PodStatus = iota
	PodStatusRunning
	PodStatusDying
	PodStatusStopped
)

var podStatusNames = []string{
	PodStatusInvalid: "invalid",
	PodStatusRunning: "running",
	PodStatusDying:   "dying",
	PodStatusStopped: "stopped",
}

func (cs PodStatus) String() string {
	if int(cs) < len(podStatusNames) {
		return podStatusNames[cs]
	}
	return fmt.Sprintf("PodStatus[%d]", cs)
}

type Pod struct {
	UUID     uuid.UUID
	Host     *Host
	Manifest schema.PodManifest

	sealed bool
	ui     *ui.UI
}

func newPod(h *Host, id uuid.UUID) *Pod {
	if id == nil {
		id = uuid.NewRandom()
	}
	return &Pod{
		Host: h,
		UUID: id,
		ui:   ui.NewUI("yellow", "pod", id.String()),
	}
}

func CreatePod(h *Host, pm *schema.PodManifest) (pod *Pod, rErr error) {
	if pm == nil {
		return nil, errors.New("Pod manifest is nil")
	}
	if len(pm.Apps) == 0 {
		return nil, errors.New("Pod manifest has no apps")
	}
	pod = newPod(h, nil)
	pod.Manifest = *pm

	pod.ui.Debug("Initializing dataset")
	ds, err := h.Dataset.CreateDataset(path.Join("pods", pod.UUID.String()))
	if err != nil {
		return nil, errors.Trace(err)
	}

	// If we haven't finished successfully, clean up the remains
	defer func() {
		if rErr != nil {
			ds.Destroy("-r")
		}
	}()

	_, mdsGID := h.GetMDSUGID()
	if err := os.Chown(ds.Mountpoint, 0, mdsGID); err != nil {
		return nil, errors.Trace(err)
	}

	if err := os.Chmod(ds.Mountpoint, 0750); err != nil {
		return nil, errors.Trace(err)
	}

	if err := os.Mkdir(ds.Path("rootfs"), 0700); err != nil {
		return nil, errors.Trace(err)
	}

	if err := os.Mkdir(ds.Path("rootfs", "app"), 0755); err != nil {
		return nil, errors.Trace(err)
	}

	var fstab []string

	if len(pod.Manifest.Volumes) > 0 {
		for i, vol := range pod.Manifest.Volumes {
			volPath := ds.Path("rootfs", "vol", vol.Name.String())
			if err := os.MkdirAll(volPath, 0755); err != nil {
				return nil, errors.Trace(err)
			}
			switch vol.Kind {
			case "empty":
				pod.ui.Debugf("Creating volume.%v for volume %v", i, vol.Name)
				if volds, err := ds.CreateDataset(fmt.Sprintf("volume.%v", i), "-omountpoint="+volPath); err != nil {
					return nil, errors.Trace(err)
				} else if err := volds.Set("jetpack:name", string(vol.Name)); err != nil {
					return nil, errors.Trace(err)
				}
			case "host":
				opts := "rw"
				if vol.ReadOnly != nil && *vol.ReadOnly {
					opts = "ro"
				}
				fstab = append(fstab, fmt.Sprintf("%v %v nullfs %v 0 0\n",
					vol.Source, volPath, opts))
			default:
				return nil, errors.Errorf("Unknown volume kind: %v", vol.Kind)
			}
		}
	}

	for i, rtApp := range pod.Manifest.Apps {
		pod.ui.Debugf("Cloning rootfs.%d for app %v", i, rtApp.Name)
		img, err := h.getRuntimeImage(rtApp.Image)
		if err != nil {
			return nil, errors.Annotate(err, rtApp.Image.ID.String())
		}

		appRootfs := ds.Path("rootfs", strconv.Itoa(i))
		rootds, err := img.Clone(ds.ChildName(fmt.Sprintf("rootfs.%v", i)), appRootfs)
		if err != nil {
			return nil, errors.Trace(err)
		}

		if err := rootds.Set("jetpack:name", string(rtApp.Name)); err != nil {
			return nil, errors.Trace(err)
		}

		if _, err := rootds.Snapshot("parent"); err != nil {
			return nil, errors.Trace(err)
		}

		if err := os.Mkdir(ds.Path("rootfs", "app", rtApp.Name.String()), 0755); err != nil {
			return nil, errors.Trace(err)
		}

		if err := os.Symlink(
			filepath.Join("..", "..", strconv.Itoa(i)),
			ds.Path("rootfs", "app", rtApp.Name.String(), "rootfs"),
		); err != nil {
			return nil, errors.Trace(err)
		}

		app := rtApp.App
		if app == nil {
			app = img.Manifest.App
		}

		// TODO: way to disable auto-devfs? Custom ruleset?
		if err := os.Mkdir(filepath.Join(appRootfs, "dev"), 0555); err != nil && !os.IsExist(err) {
			return nil, errors.Trace(err)
		}
		fstab = append(fstab, fmt.Sprintf(". %v devfs ruleset=4 0 0\n", filepath.Join(appRootfs, "dev")))

		if os_, _ := img.Manifest.GetLabel("os"); os_ == "linux" {
			for _, dir := range []string{"sys", "proc"} {
				if err := os.MkdirAll(filepath.Join(appRootfs, dir), 0755); err != nil && !os.IsExist(err) {
					return nil, errors.Trace(err)
				}
			}
			fstab = append(fstab, fmt.Sprintf("linproc %v linprocfs rw 0 0\n", filepath.Join(appRootfs, "proc")))
			fstab = append(fstab, fmt.Sprintf("linsys %v linsysfs  rw 0 0\n", filepath.Join(appRootfs, "sys")))
		}

		if app != nil {
			for _, mntpnt := range app.MountPoints {
				if err := os.MkdirAll(filepath.Join(appRootfs, mntpnt.Path), 0755); err != nil && !os.IsExist(err) {
					return nil, errors.Trace(err)
				}
				var mnt *schema.Mount
				for _, cmnt := range rtApp.Mounts {
					if cmnt.MountPoint == mntpnt.Name {
						mnt = &cmnt
						break
					}
				}
				if mnt == nil {
					return nil, errors.Errorf("Unfulfilled mount point %v:%v", rtApp.Name, mntpnt.Name)
				}
				opts := "rw"
				if mntpnt.ReadOnly {
					opts = "ro"
				}
				fstab = append(fstab, fmt.Sprintf("%v %v nullfs %v 1 0\n",
					ds.Path("rootfs", "vol", mnt.Volume.String()),
					filepath.Join(appRootfs, mntpnt.Path),
					opts))
			}
		}
	}

	if err := ioutil.WriteFile(pod.Path("fstab"), []byte(strings.Join(fstab, "")), 0400); err != nil {
		return nil, errors.Trace(err)
	}

	// FIXME: smarter IP allocation?
	if ip, err := h.nextIP(); err != nil {
		return nil, errors.Trace(err)
	} else {
		pod.ui.Debug("Using IP", ip)
		pod.Manifest.Annotations.Set("ip-address", ip.String())
	}

	if err := ioutil.WriteFile(pod.Path("jail.conf"), []byte(pod.jailConf()), 0400); err != nil {
		return nil, errors.Trace(err)
	}

	pod.ui.Debug("Saving manifest")
	if manifestJSON, err := json.Marshal(pod.Manifest); err != nil {
		return nil, errors.Trace(err)
	} else if err := ioutil.WriteFile(pod.Path("manifest"), manifestJSON, 0440); err != nil {
		return nil, errors.Trace(err)
	} else if err := os.Chown(pod.Path("manifest"), 0, mdsGID); err != nil {
		return nil, errors.Trace(err)
	}
	pod.sealed = true
	return pod, nil
}

func LoadPod(h *Host, id uuid.UUID) (*Pod, error) {
	if id == nil {
		panic("No UUID provided")
	}
	pod := newPod(h, id)
	if err := pod.Load(); err != nil {
		return nil, errors.Trace(err)
	}
	return pod, nil
}

func (p *Pod) ID() string {
	return p.UUID.String()
}

func (c *Pod) Path(elem ...string) string {
	return c.Host.Path(append(
		[]string{"pods", c.UUID.String()},
		elem...,
	)...)
}

func (c *Pod) Exists() bool {
	if _, err := os.Stat(c.Path("manifest")); err != nil {
		if os.IsNotExist(err) {
			return false
		}
		panic(err)
	}
	return true
}

func (c *Pod) loadManifest() error {
	c.ui.Debug("Loading manifest")
	manifestJSON, err := ioutil.ReadFile(c.Path("manifest"))
	if err != nil {
		return errors.Trace(err)
	}

	if err = json.Unmarshal(manifestJSON, &c.Manifest); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (c *Pod) Load() error {
	if c.sealed {
		panic("tried to load an already sealed pod")
	}

	if !c.Exists() {
		return ErrNotFound
	}

	if err := c.loadManifest(); err != nil {
		return errors.Trace(err)
	}

	if len(c.Manifest.Apps) == 0 {
		return errors.Errorf("No application set?")
	}

	if len(c.Manifest.Isolators) != 0 {
		return errors.Errorf("TODO: isolators are not supported")
	}

	c.sealed = true
	return nil
}

func (c *Pod) jailConf() string {
	parameters := map[string]string{
		"exec.clean":    "true",
		"host.hostuuid": c.UUID.String(),
		"interface":     c.Host.Properties.MustGetString("jail.interface"),
		"path":          c.Path("rootfs"),
		"persist":       "true",
		"mount.fstab":   c.Path("fstab"),
	}

	for pk, pv := range c.Host.GetPrefixProperties("ace.jailConf.") {
		parameters[pk] = pv
	}

	if hostname, ok := c.Manifest.Annotations.Get("hostname"); ok {
		parameters["host.hostname"] = hostname
	} else {
		parameters["host.hostname"] = parameters["host.hostuuid"]
	}

	if ip, ok := c.Manifest.Annotations.Get("ip-address"); ok {
		parameters["ip4.addr"] = ip
	} else {
		panic(fmt.Sprintf("No IP address for pod %v", c.UUID))
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

func (c *Pod) prepJail() error {
	for _, app := range c.Manifest.Apps {
		etcPath := c.Path("rootfs", "app", app.Name.String(), "rootfs", "etc")
		if fi, err := os.Stat(etcPath); err == nil && fi.IsDir() {
			// TODO: option (isolator?) to prevent creation of resolv.conf
			if dnsServers, ok := c.Host.Properties.Get("ace.dns-servers"); !ok {
				// By default, copy /etc/resolv.conf from host
				if bb, err := ioutil.ReadFile("/etc/resolv.conf"); err != nil {
					return errors.Trace(err)
				} else {
					if err := ioutil.WriteFile(filepath.Join(etcPath, "resolv.conf"), bb, 0644); err != nil {
						return errors.Trace(err)
					}
				}
			} else if resolvconf, err := os.Create(filepath.Join(etcPath, "resolv.conf")); err != nil {
				return errors.Trace(err)
			} else {
				for _, server := range strings.Fields(dnsServers) {
					fmt.Fprintln(resolvconf, "nameserver", server)
				}
				resolvconf.Close()
			}
		}
	}
	return nil
}

func (c *Pod) Status() PodStatus {
	if status, err := c.jailStatus(false); err != nil {
		panic(err)
	} else {
		if status == NoJailStatus {
			return PodStatusStopped
		}
		if status.Dying {
			return PodStatusDying
		}
		return PodStatusRunning
	}
}

func (c *Pod) runJail(op string) error {
	if err := c.prepJail(); err != nil {
		return err
	}
	verbosity := "-q"
	if c.Host.Properties.GetBool("debug", false) {
		verbosity = "-v"
	}
	c.ui.Debug("Running: jail", op)
	return run.Command("jail", "-f", c.Path("jail.conf"), verbosity, op, c.jailName()).Run()
}

func (c *Pod) Kill() error {
	c.ui.Println("Shutting down")
	spin := ui.NewSpinner("Waiting for jail to die", ui.SuffixElapsed(), nil)
	defer spin.Finish()
retry:
	switch status := c.Status(); status {
	case PodStatusStopped:
		// All's fine
		return nil
	case PodStatusRunning:
		if err := c.runJail("-r"); err != nil {
			return errors.Trace(err)
		}
		goto retry
	case PodStatusDying:
		// TODO: UI? Log?
		spin.Step()
		time.Sleep(250 * time.Millisecond)
		goto retry
	default:
		return errors.Errorf("Pod is %v, I am confused", status)
	}
}

// FIXME: multi-app pods
func (c *Pod) getDataset() *zfs.Dataset {
	if ds, err := c.Host.Dataset.GetDataset(path.Join("pods", c.UUID.String())); err == zfs.ErrNotFound {
		return nil
	} else if err != nil {
		panic(err)
	} else {
		return ds
	}
}

func (c *Pod) Destroy() error {
	c.ui.Println("Destroying")
	if jid := c.Jid(); jid != 0 {
		if err := c.Kill(); err != nil {
			// FIXME: plow through, ensure it's destroyed
			return errors.Trace(err)
		}
	}
	if ds := c.getDataset(); ds != nil {
		if err := ds.Destroy("-r"); err != nil {
			return errors.Trace(err)
		}
	}
	return errors.Trace(os.RemoveAll(c.Path()))
}

func (c *Pod) jailName() string {
	return c.Host.Properties.MustGetString("jail.namePrefix") + c.UUID.String()
}

func (c *Pod) jailStatus(refresh bool) (JailStatus, error) {
	return c.Host.getJailStatus(c.jailName(), refresh)
}

func (c *Pod) Jid() int {
	if status, err := c.jailStatus(false); err != nil {
		panic(err) // FIXME: better error flow
	} else {
		return status.Jid
	}
}

func (c *Pod) RunApp(name types.ACName) error {
	if rta := c.Manifest.Apps.Get(name); rta != nil {
		return c.runRuntimeApp(rta)
	}
	return ErrNotFound
}

func (c *Pod) runRuntimeApp(rtapp *schema.RuntimeApp) error {
	app := rtapp.App
	if app == nil {
		img, err := c.Host.getRuntimeImage(rtapp.Image)
		if err != nil {
			return errors.Trace(err)
		}
		app = img.Manifest.App
		if app == nil {
			app = ConsoleApp("root")
		}
	}
	return c.runApp(rtapp.Name, app)
}

func (c *Pod) Console(name types.ACName, user string) error {
	return c.runApp(name, ConsoleApp(user))
}

func (c *Pod) runApp(name types.ACName, app *types.App) (re error) {
	if _, err := c.Host.NeedMDS(); err != nil {
		return errors.Trace(err)
	}

	env := []string{}

	for _, env_var := range app.Environment {
		env = append(env, "-setenv", env_var.Name+"="+env_var.Value)
	}

	for _, eh := range app.EventHandlers {
		switch eh.Name {
		case "pre-start":
			// TODO: log
			if err := c.stage2(name, "", "", app.WorkingDirectory, env, eh.Exec...); err != nil {
				return errors.Trace(err)
			}
		case "post-stop":
			defer func(exec []string) {
				// TODO: log
				if err := c.stage2(name, "", "", app.WorkingDirectory, env, exec...); err != nil {
					if re != nil {
						re = errors.Trace(err)
					} // else? log?
				}
			}(eh.Exec)
		default:
			return errors.Errorf("Unrecognized eventHandler: %v", eh.Name)
		}
	}

	return errors.Trace(c.stage2(name, app.User, app.Group, app.WorkingDirectory, env, app.Exec...))
}

func (c *Pod) stage2(app types.ACName, user, group string, cwd string, env []string, exec ...string) error {
	if strings.HasPrefix(user, "/") || strings.HasPrefix(group, "/") {
		return errors.New("Path-based user/group not supported yet, sorry")
	}

	// Ensure jail is created
	jid := c.Jid()
	if jid == 0 {
		if err := errors.Trace(c.runJail("-c")); err != nil {
			return errors.Trace(err)
		}
		jid = c.Jid()
		if jid == 0 {
			panic("Could not start jail")
		}
	}

	mds, err := c.Host.MetadataURL()
	if err != nil {
		return errors.Trace(err)
	}

	pwf, err := passwd.ReadPasswd(c.Path("rootfs", "app", app.String(), "rootfs", "etc", "passwd"))
	if err != nil {
		return errors.Trace(err)
	}

	pwent := pwf.Find(user)
	if pwent == nil {
		return errors.Errorf("Cannot find user: %#v", user)
	}

	if group != "" {
		grf, err := passwd.ReadGroup(c.Path("rootfs", "app", app.String(), "rootfs", "etc", "group"))
		if err != nil {
			return errors.Trace(err)
		}
		pwent.Gid = grf.FindGid(group)
		if pwent.Gid < 0 {
			return errors.Errorf("Cannot find group: %#v", group)
		}
	}

	if cwd == "" {
		cwd = "/"
	}

	return run.Command(
		filepath.Join(LibexecPath, "stage2"),
		append(
			append(
				[]string{
					"-jid", strconv.Itoa(jid),
					"-app", string(app),
					"-mds", mds,
					"-uid", strconv.Itoa(pwent.Uid),
					"-gid", strconv.Itoa(pwent.Gid),
					"-cwd", cwd,
					"-setenv", "USER=" + pwent.Username,
					"-setenv", "LOGNAME=" + pwent.Username,
					"-setenv", "HOME=" + pwent.Home,
					"-setenv", "SHELL=" + pwent.Shell,
				},
				env...),
			exec...)...).Run()
}
