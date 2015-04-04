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

import "../run"
import "../zfs"

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
}

func CreatePod(h *Host, pm *schema.PodManifest) (pod *Pod, rErr error) {
	if pm == nil {
		return nil, errors.New("Pod manifest is nil")
	}
	if len(pm.Apps) == 0 {
		return nil, errors.New("Pod manifes has no apps")
	}
	pod = &Pod{Host: h, UUID: uuid.NewRandom(), Manifest: *pm}

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

	if err := os.Mkdir(ds.Path("rootfs"), 0700); err != nil {
		return nil, errors.Trace(err)
	}

	for i, app := range pod.Manifest.Apps {
		img, err := h.GetImageByHash(app.Image.ID)
		if err != nil {
			return nil, errors.Trace(err)
		}

		rootds, err := img.Clone(ds.ChildName(fmt.Sprintf("rootfs.%v", i)), ds.Path("rootfs", strconv.Itoa(i)))
		if err != nil {
			return nil, errors.Trace(err)
		}

		if err := rootds.Set("jetpack:name", string(app.Name)); err != nil {
			return nil, errors.Trace(err)
		}

		if img.Manifest.App != nil {
			for _, mnt := range img.Manifest.App.MountPoints {
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
		pod.Manifest.Annotations.Set("ip-address", ip.String())
	}

	if err := pod.Save(); err != nil {
		return nil, errors.Trace(err)
	}

	return pod, nil
}

func LoadPod(h *Host, id uuid.UUID) (*Pod, error) {
	if id == nil {
		panic("No UUID provided")
	}
	pod := &Pod{UUID: id, Host: h}
	if err := pod.Load(); err != nil {
		return nil, errors.Trace(err)
	}
	return pod, nil
}

func (c *Pod) Save() error {
	if manifestJSON, err := json.Marshal(c.Manifest); err != nil {
		return errors.Trace(err)
	} else if err := ioutil.WriteFile(c.Path("manifest"), manifestJSON, 0400); err != nil {
		return errors.Trace(err)
	}
	c.sealed = true
	return nil
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

func (c *Pod) readManifest() error {
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

	if err := c.readManifest(); err != nil {
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

		if bb, err := ioutil.ReadFile("/etc/resolv.conf"); err != nil {
			return errors.Trace(err)
		} else {
			if err := ioutil.WriteFile(c.Path("rootfs", rootno, "etc/resolv.conf"), bb, 0644); err != nil {
				return errors.Trace(err)
			}
		}

		imgApp := img.Manifest.App
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
				if err := os.MkdirAll(hostPath, 0700); err != nil {
					return errors.Trace(err)
				}
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
	return run.Command("jail", "-f", c.Path("jail.conf"), verbosity, op, c.jailName()).Run()
}

func (c *Pod) Kill() error {
	t0 := time.Now()
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
		fmt.Printf("Pod dying since %v, waiting...\n", time.Now().Sub(t0))
		time.Sleep(2500 * time.Millisecond)
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
	return c.Stage2(rtapp.Name, app)
}

func (c *Pod) Console(name types.ACName, user string) error {
	return c.Stage2(name, ConsoleApp(user))
}

func (c *Pod) getChroot(appName types.ACName) string {
	for i, app := range c.Manifest.Apps {
		if app.Name == appName {
			return fmt.Sprintf("/%v", i)
		}
	}
	return "/"
}

func (c *Pod) Stage2(name types.ACName, app *types.App) error {
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

	user := app.User
	if user == "" {
		user = "root"
	}

	args := []string{
		"-jid", strconv.Itoa(jid),
		"-chroot", c.getChroot(name),
		"-user", user,
		"-group", app.Group,
		"-name", string(name),
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
