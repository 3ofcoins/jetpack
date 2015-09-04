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

	_, mdsGID := MDSUidGid()
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

func (pod *Pod) ID() string {
	return pod.UUID.String()
}

func (pod *Pod) Path(elem ...string) string {
	return pod.Host.Path(append(
		[]string{"pods", pod.UUID.String()},
		elem...,
	)...)
}

func (pod *Pod) Exists() bool {
	if _, err := os.Stat(pod.Path("manifest")); err != nil {
		if os.IsNotExist(err) {
			return false
		}
		panic(err)
	}
	return true
}

func (pod *Pod) loadManifest() error {
	pod.ui.Debug("Loading manifest")
	manifestJSON, err := ioutil.ReadFile(pod.Path("manifest"))
	if err != nil {
		return errors.Trace(err)
	}

	if err = json.Unmarshal(manifestJSON, &pod.Manifest); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (pod *Pod) Load() error {
	if pod.sealed {
		panic("tried to load an already sealed pod")
	}

	if !pod.Exists() {
		return ErrNotFound
	}

	if err := pod.loadManifest(); err != nil {
		return errors.Trace(err)
	}

	if len(pod.Manifest.Apps) == 0 {
		return errors.Errorf("No application set?")
	}

	if len(pod.Manifest.Isolators) != 0 {
		return errors.Errorf("TODO: isolators are not supported")
	}

	pod.sealed = true
	return nil
}

func (pod *Pod) jailConf() string {
	parameters := map[string]string{
		"exec.clean":    "true",
		"host.hostuuid": pod.UUID.String(),
		"interface":     Config().MustGetString("jail.interface"),
		"path":          pod.Path("rootfs"),
		"persist":       "true",
		"mount.fstab":   pod.Path("fstab"),
	}

	for pk, pv := range ConfigPrefix("ace.jailConf.") {
		parameters[pk] = pv
	}

	if hostname, ok := pod.Manifest.Annotations.Get("hostname"); ok {
		parameters["host.hostname"] = hostname
	} else {
		parameters["host.hostname"] = parameters["host.hostuuid"]
	}

	if ip, ok := pod.Manifest.Annotations.Get("ip-address"); ok {
		parameters["ip4.addr"] = ip
	} else {
		panic(fmt.Sprintf("No IP address for pod %v", pod.UUID))
	}

	for _, antn := range pod.Manifest.Annotations {
		if strings.HasPrefix(string(antn.Name), "jetpack/jail.conf/") {
			parameters[strings.Replace(string(antn.Name)[len("jetpack/jail.conf/"):], "-", "_", -1)] = antn.Value
		}
	}

	lines := make([]string, 0, len(parameters))
	for k, v := range parameters {
		lines = append(lines, fmt.Sprintf("  %v=%#v;", k, v))
	}
	sort.Strings(lines)

	return fmt.Sprintf("%#v {\n%v\n}\n", pod.jailName(), strings.Join(lines, "\n"))
}

func (pod *Pod) prepJail() error {
	for _, app := range pod.Manifest.Apps {
		etcPath := pod.Path("rootfs", "app", app.Name.String(), "rootfs", "etc")
		if fi, err := os.Stat(etcPath); err == nil && fi.IsDir() {
			// TODO: option (isolator?) to prevent creation of resolv.conf
			if dnsServers, ok := Config().Get("ace.dns-servers"); !ok {
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

func (pod *Pod) Status() PodStatus {
	if status, err := pod.jailStatus(false); err != nil {
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

func (pod *Pod) runJail(op string) error {
	if err := pod.prepJail(); err != nil {
		return err
	}
	verbosity := "-q"
	if Config().GetBool("debug", false) {
		verbosity = "-v"
	}
	pod.ui.Debug("Running: jail", op)
	return run.Command("jail", "-f", pod.Path("jail.conf"), verbosity, op, pod.jailName()).Run()
}

func (pod *Pod) Kill() error {
	pod.ui.Println("Shutting down")
	spin := ui.NewSpinner("Waiting for jail to die", ui.SuffixElapsed(), nil)
	defer spin.Finish()
retry:
	switch status := pod.Status(); status {
	case PodStatusStopped:
		// All's fine
		return nil
	case PodStatusRunning:
		if err := pod.runJail("-r"); err != nil {
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
func (pod *Pod) getDataset() *zfs.Dataset {
	if ds, err := pod.Host.Dataset.GetDataset(path.Join("pods", pod.UUID.String())); err == zfs.ErrNotFound {
		return nil
	} else if err != nil {
		panic(err)
	} else {
		return ds
	}
}

func (pod *Pod) Destroy() error {
	pod.ui.Println("Destroying")
	if jid := pod.Jid(); jid != 0 {
		if err := pod.Kill(); err != nil {
			// FIXME: plow through, ensure it's destroyed
			return errors.Trace(err)
		}
	}
	if ds := pod.getDataset(); ds != nil {
		if err := ds.Destroy("-r"); err != nil {
			return errors.Trace(err)
		}
	}
	return errors.Trace(os.RemoveAll(pod.Path()))
}

func (pod *Pod) jailName() string {
	return Config().MustGetString("jail.namePrefix") + pod.UUID.String()
}

func (pod *Pod) jailStatus(refresh bool) (JailStatus, error) {
	return pod.Host.getJailStatus(pod.jailName(), refresh)
}

func (pod *Pod) Jid() int {
	if status, err := pod.jailStatus(false); err != nil {
		panic(err) // FIXME: better error flow
	} else {
		return status.Jid
	}
}

func (pod *Pod) MetadataURL() (string, error) {
	mds, err := pod.Host.MetadataURL(pod.UUID)
	return mds, errors.Trace(err)
}

func (pod *Pod) App(name types.ACName) *App {
	rtapp := pod.Manifest.Apps.Get(name)
	if rtapp == nil {
		return nil
	}
	app := rtapp.App
	if app == nil {
		img, err := pod.Host.getRuntimeImage(rtapp.Image)
		if err != nil {
			// FIXME: Report error to UI? Panic?
			return nil
		}
		app = img.Manifest.App
		if app == nil {
			app = ConsoleApp("root")
		}
	}
	return &App{Name: name, Pod: pod, app: app}
}

func (pod *Pod) Apps() []*App {
	apps := make([]*App, len(pod.Manifest.Apps))
	for i, rtapp := range pod.Manifest.Apps {
		apps[i] = pod.App(rtapp.Name)
	}
	return apps
}
