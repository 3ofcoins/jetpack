package jetpack

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

import "golang.org/x/sys/unix"

import "code.google.com/p/go-uuid/uuid"
import "github.com/appc/spec/schema"
import "github.com/appc/spec/schema/types"
import "github.com/juju/errors"

import "lib/run"
import "lib/ui"
import "lib/zfs"

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

	volumesDirCreated := false
	for i, vol := range pod.Manifest.Volumes {
		if vol.Kind == "empty" {
			pod.ui.Debugf("Creating volume.%v for volume %v", i, vol.Name)
			if !volumesDirCreated {
				if err := os.Mkdir(ds.Path("volumes"), 0700); err != nil {
					return nil, errors.Trace(err)
				}
				volumesDirCreated = true
			}
			if volds, err := ds.CreateDataset(fmt.Sprintf("volume.%v", i), "-omountpoint="+ds.Path("volumes", strconv.Itoa(i))); err != nil {
				return nil, errors.Trace(err)
			} else if err := volds.Set("jetpack:name", string(vol.Name)); err != nil {
				return nil, errors.Trace(err)
			}
		}
	}

	for i, rtApp := range pod.Manifest.Apps {
		pod.ui.Debugf("Cloning rootfs.%d for app %v", i, rtApp.Name)
		img, err := h.GetImageByHash(rtApp.Image.ID)
		if err != nil {
			return nil, errors.Annotate(err, rtApp.Image.ID.String())
		}

		rootds, err := img.Clone(ds.ChildName(fmt.Sprintf("rootfs.%v", i)), ds.Path("rootfs", strconv.Itoa(i)))
		if err != nil {
			return nil, errors.Trace(err)
		}

		if err := rootds.Set("jetpack:name", string(rtApp.Name)); err != nil {
			return nil, errors.Trace(err)
		}

		if _, err := rootds.Snapshot("parent"); err != nil {
			return nil, errors.Trace(err)
		}

		app := rtApp.App
		if app == nil {
			app = img.Manifest.App
		}

		if app != nil {
			for _, mnt := range app.MountPoints {
				if err := os.MkdirAll(rootds.Path(mnt.Path), 0755); err != nil && !os.IsExist(err) {
					return nil, errors.Trace(err)
				}
			}
		}
		if os_, _ := img.Manifest.GetLabel("os"); os_ == "linux" {
			for _, dir := range []string{"sys", "proc"} {
				if err := os.MkdirAll(rootds.Path(dir), 0755); err != nil && !os.IsExist(err) {
					return nil, errors.Trace(err)
				}
			}
		}
	}

	// FIXME: smarter IP allocation?
	if ip, err := h.nextIP(); err != nil {
		return nil, errors.Trace(err)
	} else {
		pod.ui.Debug("Using IP", ip)
		pod.Manifest.Annotations.Set("ip-address", ip.String())
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
	var fstab []string

	for i, app := range c.Manifest.Apps {
		rootno := strconv.Itoa(i)

		fstab = append(fstab, fmt.Sprintf(". %v devfs ruleset=4 0 0\n",
			c.Path("rootfs", rootno, "dev")))

		img, err := c.Host.GetImageByHash(app.Image.ID)
		if err != nil {
			// TODO: someday we may offer to install missing images
			return errors.Trace(err)
		}

		if os, _ := img.Manifest.GetLabel("os"); os == "linux" {
			fstab = append(fstab,
				fmt.Sprintf("linsys %v linsysfs  rw 0 0\n", c.Path("rootfs", rootno, "sys")),
				fmt.Sprintf("linproc %v linprocfs rw 0 0\n", c.Path("rootfs", rootno, "proc")),
			)
		}

		if dnsServers, ok := c.Host.Properties.Get("ace.dns-servers"); !ok {
			// By default, copy /etc/resolv.conf from host
			if bb, err := ioutil.ReadFile("/etc/resolv.conf"); err != nil {
				return errors.Trace(err)
			} else {
				if err := ioutil.WriteFile(c.Path("rootfs", rootno, "etc/resolv.conf"), bb, 0644); err != nil {
					return errors.Trace(err)
				}
			}
		} else if err := os.MkdirAll(c.Path("rootfs", rootno, "etc"), 0755); err != nil {
			return errors.Trace(err)
		} else if resolvconf, err := os.Create(c.Path("rootfs", rootno, "etc/resolv.conf")); err != nil {
			return errors.Trace(err)
		} else {
			for _, server := range strings.Fields(dnsServers) {
				fmt.Fprintln(resolvconf, "nameserver", server)
			}
			resolvconf.Close()
		}

		if err := os.MkdirAll(c.Path("rootfs", rootno, "dev"), 0555); err != nil {
			return errors.Trace(err)
		}

		imgApp := app.App
		if imgApp == nil {
			imgApp = img.Manifest.App
		}
		if imgApp == nil {
			continue
		}

		fulfilledMountPoints := make(map[types.ACName]bool)
		for _, mnt := range app.Mounts {
			var vol types.Volume
			volNo := -1
			for i, cvol := range c.Manifest.Volumes {
				if cvol.Name == mnt.Volume {
					vol = cvol
					volNo = i
					break
				}
			}
			if volNo < 0 {
				return errors.Errorf("Volume not found: %v", mnt.Volume)
			}

			var mntPoint *types.MountPoint
			for _, mntp := range imgApp.MountPoints {
				if mntp.Name == mnt.MountPoint {
					mntPoint = &mntp
					break
				}
			}
			if mntPoint == nil {
				return errors.Errorf("No mount point found: %v", mnt.MountPoint)
			}

			fulfilledMountPoints[mnt.MountPoint] = true

			podPath := c.Path("rootfs", rootno, mntPoint.Path)
			hostPath := vol.Source

			if vol.Kind == "empty" {
				hostPath = c.Path("volumes", strconv.Itoa(volNo))
				var st unix.Stat_t
				if err := unix.Stat(podPath, &st); err != nil {
					if !os.IsNotExist(err) {
						return errors.Trace(err)
					} else {
						// TODO: make path?
					}
				} else {
					// Copy ownership & mode from image's already existing mount
					// point.
					// TODO: What if multiple images use same empty volume, and
					// have conflicting modes?
					if err := unix.Chmod(hostPath, uint32(st.Mode&07777)); err != nil {
						return errors.Trace(err)
					}
					if err := unix.Chown(hostPath, int(st.Uid), int(st.Gid)); err != nil {
						return errors.Trace(err)
					}
				}
			}

			opts := "rw"
			if (vol.ReadOnly != nil && *vol.ReadOnly) || mntPoint.ReadOnly {
				opts = "ro"
			}

			fstab = append(fstab, fmt.Sprintf("%v %v nullfs %v 0 0\n",
				hostPath, podPath, opts))
		}

		var unfulfilled []types.ACName
		for _, mntp := range imgApp.MountPoints {
			if !fulfilledMountPoints[mntp.Name] {
				unfulfilled = append(unfulfilled, mntp.Name)
			}
		}
		if len(unfulfilled) > 0 {
			return errors.Errorf("Unfulfilled mount points for %v: %v", img.Manifest.Name, unfulfilled)
		}
	}

	if err := ioutil.WriteFile(c.Path("fstab"), []byte(strings.Join(fstab, "")), 0600); err != nil {
		return errors.Trace(err)
	}

	return errors.Trace(
		ioutil.WriteFile(c.Path("jail.conf"), []byte(c.jailConf()), 0400))
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
		img, err := c.Host.GetImageByHash(rtapp.Image.ID)
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

func (c *Pod) getChroot(appName types.ACName) string {
	for i, app := range c.Manifest.Apps {
		if app.Name == appName {
			return fmt.Sprintf("/%v", i)
		}
	}
	return "/"
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
			if err := c.stage2(name, "0", "0", app.WorkingDirectory, env, eh.Exec...); err != nil {
				return errors.Trace(err)
			}
		case "post-stop":
			defer func(exec []string) {
				// TODO: log
				if err := c.stage2(name, "0", "0", app.WorkingDirectory, env, exec...); err != nil {
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

func (c *Pod) stage2(name types.ACName, user, group string, cwd string, env []string, exec ...string) error {
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

	if user == "" {
		user = "0"
	}

	if group == "" {
		group = "0"
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
					"-chroot", c.getChroot(name),
					"-name", string(name),
					"-mds", mds,
					"-user", user,
					"-group", group,
					"-cwd", cwd,
				},
				env...),
			exec...)...).Run()
}
