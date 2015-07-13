package jetpack

import (
	"crypto/sha512"
	"encoding/json"
	stderrors "errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.google.com/p/go-uuid/uuid"
	"github.com/appc/spec/discovery"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/juju/errors"
	"github.com/magiconair/properties"
	openpgp_err "golang.org/x/crypto/openpgp/errors"

	"lib/acutil"
	"lib/fetch"
	"lib/keystore"
	"lib/run"
	"lib/ui"
	"lib/zfs"
)

var ErrUsage = stderrors.New("Invalid usage")
var ErrNotFound = stderrors.New("Not found")
var ErrManyFound = stderrors.New("Multiple results found")

type JailStatus struct {
	Jid   int
	Dying bool
}

var NoJailStatus = JailStatus{}

type Host struct {
	Dataset    *zfs.Dataset
	Properties *properties.Properties

	jailStatusTimestamp time.Time
	jailStatusCache     map[string]JailStatus
	mdsUid, mdsGid      int
	ui                  *ui.UI
}

func NewHost(configPath string) (*Host, error) {
	h := Host{mdsUid: -1, mdsGid: -1}
	h.Properties = properties.MustLoadFiles(
		[]string{
			filepath.Join(SharedPath, "jetpack.conf.defaults"),
			configPath,
		},
		properties.UTF8,
		true)

	// FIXME: changing global switch based on struct instance
	// variable. There should be only one instance created at a time
	// anyway, but it's kind of ugly.

	// If debug is already on (e.g. from a command line switch), we keep
	// it.
	ui.Debug = ui.Debug || h.Properties.GetBool("debug", false)
	h.ui = ui.NewUI("green", "jetpack", "")

	if ds, err := zfs.GetDataset(h.Properties.MustGetString("root.zfs")); err == zfs.ErrNotFound {
		return &h, nil
	} else if err != nil {
		return nil, err
	} else {
		h.Dataset = ds
	}

	return &h, nil
}

// Host-global stuff
//////////////////////////////////////////////////////////////////////////////

func (h *Host) Path(elem ...string) string {
	return h.Dataset.Path(elem...)
}

func (h *Host) GetPrefixProperties(prefix string) map[string]string {
	rv := make(map[string]string)
	pl := len(prefix)
	pp := h.Properties.FilterPrefix(prefix)
	for _, pk := range pp.Keys() {
		if pv, ok := pp.Get(pk); ok && pv != "" {
			rv[pk[pl:]] = pv
		}
	}
	return rv
}

func (h *Host) zfsOptions(prefix string, opts ...string) []string {
	for k, v := range h.GetPrefixProperties(prefix) {
		opts = append(opts, fmt.Sprintf("-o%v=%v", k, v))
	}
	return opts
}

func (h *Host) Initialize() error {
	if h.Dataset != nil {
		return errors.New("Host already initialized")
	}

	// We use GetString, as user can specify "root.zfs.mountpoint=" (set
	// to empty string) in config to unset property
	if mntpnt := h.Properties.GetString("root.zfs.mountpoint", ""); mntpnt != "" {
		if err := os.MkdirAll(mntpnt, 0755); err != nil {
			return errors.Trace(err)
		}
	}

	dsName := h.Properties.MustGetString("root.zfs")
	dsOptions := h.zfsOptions("root.zfs.", "-p")
	h.ui.Printf("Creating ZFS dataset %v %v", dsName, dsOptions)
	if ds, err := zfs.CreateDataset(dsName, dsOptions...); err != nil {
		return errors.Trace(err)
	} else {
		h.Dataset = ds
	}

	dsOptions = h.zfsOptions("images.zfs.")
	h.ui.Printf("Creating ZFS dataset %v/images %v", dsName, dsOptions)
	if _, err := h.Dataset.CreateDataset("images", dsOptions...); err != nil {
		return errors.Trace(err)
	}

	dsOptions = h.zfsOptions("pods.zfs.")
	h.ui.Printf("Creating ZFS dataset %v/pods %v", dsName, dsOptions)
	if _, err := h.Dataset.CreateDataset("pods", dsOptions...); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (h *Host) HostIP() (net.IP, *net.IPNet, error) {
	ifi, err := net.InterfaceByName(h.Properties.MustGetString("jail.interface"))
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	addrs, err := ifi.Addrs()
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	ip, ipnet, err := net.ParseCIDR(addrs[0].String())
	return ip, ipnet, errors.Trace(err)
}
func (h *Host) getJailStatus(name string, refresh bool) (JailStatus, error) {
	if refresh || h.jailStatusCache == nil || time.Now().Sub(h.jailStatusTimestamp) > (2*time.Second) {
		// FIXME: nicer cache/expiry implementation?
		if lines, err := run.Command("/usr/sbin/jls", "-d", "jid", "dying", "name").OutputLines(); err != nil {
			return NoJailStatus, errors.Trace(err)
		} else {
			stat := make(map[string]JailStatus)
			for _, line := range lines {
				fields := strings.SplitN(line, " ", 3)
				status := NoJailStatus
				if len(fields) != 3 {
					return NoJailStatus, errors.Errorf("Cannot parse jls line %#v", line)
				}

				if jid, err := strconv.Atoi(fields[0]); err != nil {
					return NoJailStatus, errors.Annotatef(err, "Cannot parse jls line %#v", line)
				} else {
					status.Jid = jid
				}

				if dying, err := strconv.Atoi(fields[1]); err != nil {
					return NoJailStatus, errors.Annotatef(err, "Cannot parse jls line %#v", line)
				} else {
					status.Dying = (dying != 0)
				}

				stat[fields[2]] = status
			}
			h.jailStatusCache = stat
		}
	}
	return h.jailStatusCache[name], nil
}

func (h *Host) nextIP() (net.IP, error) {
	ip, ipnet, err := h.HostIP()
	if err != nil {
		return nil, errors.Trace(err)
	}

	ips := make(map[string]bool)
	for _, c := range h.Pods() {
		if ip, ok := c.Manifest.Annotations.Get("ip-address"); ok {
			ips[ip] = true
		}
	}

	for ip = nextIP(ip); ip != nil && ips[ip.String()]; ip = nextIP(ip) {
	}

	if ip == nil {
		return nil, errors.New("Out of IPs")
	}

	if ipnet.Contains(ip) {
		return ip, nil
	} else {
		return nil, errors.New("Out of IPs")
	}
}

// Pods
//////////////////////////////////////////////////////////////////////////////

func (h *Host) ReifyPodManifest(pm *schema.PodManifest) (*schema.PodManifest, error) {
	for i, rtapp := range pm.Apps {
		// TODO: appc/spec PR unifying RuntimeImage & Dependency
		dep := types.Dependency{Labels: rtapp.Image.Labels}
		if rtapp.Image.Name != nil {
			dep.ImageName = *rtapp.Image.Name
		}

		if !rtapp.Image.ID.Empty() {
			dep.ImageID = &rtapp.Image.ID
		}

		img, err := h.GetImageDependency(&dep)
		if err != nil {
			return nil, errors.Trace(err)
		}

		pm.Apps[i].Image.ID = *img.Hash

		app := rtapp.App
		if app == nil {
			app = img.Manifest.App
		}
		if app == nil {
			if len(rtapp.Mounts) > 0 {
				return nil, errors.New("No app (is it valid at all?), yet mounts given")
			}
			continue
		}

	mntpnts:
		for _, mntpnt := range app.MountPoints {
			var mnt *schema.Mount
			for _, mntc := range rtapp.Mounts {
				if mntc.MountPoint == mntpnt.Name {
					mnt = &mntc
				}
			}
			if mnt == nil {
				fmt.Printf("INFO: mount for %v:%v not found, inserting mount for volume %v\n", rtapp.Name, mntpnt.Name, mntpnt.Name)
				mnt = &schema.Mount{MountPoint: mntpnt.Name, Volume: mntpnt.Name}
				pm.Apps[i].Mounts = append(pm.Apps[i].Mounts, *mnt)
			}
			for _, vol := range pm.Volumes {
				if vol.Name == mnt.Volume {
					continue mntpnts
				}
			}
			fmt.Printf("INFO: volume %v not found, inserting empty volume\n", mnt.Volume)
			pm.Volumes = append(pm.Volumes, types.Volume{Name: mnt.Volume, Kind: "empty"})
		}
	}

	return pm, nil
}

// Create new pod from a fully reified manifest.
func (h *Host) CreatePod(pm *schema.PodManifest) (*Pod, error) {
	return CreatePod(h, pm)
}

func (h *Host) GetPod(id uuid.UUID) (*Pod, error) {
	if c, err := LoadPod(h, id); err != nil {
		return nil, errors.Trace(err)
	} else {
		return c, nil
	}
}

func (h *Host) Pods() []*Pod {
	mm, _ := filepath.Glob(h.Path("pods/*/manifest"))
	rv := make([]*Pod, 0, len(mm))
	for _, m := range mm {
		if id := uuid.Parse(filepath.Base(filepath.Dir(m))); id == nil {
			panic(fmt.Sprintf("Invalid UUID: %#v", filepath.Base(filepath.Dir(m))))
		} else if c, err := h.GetPod(id); err != nil {
			h.ui.Printf("WARNING: pods/%v: %v\n", id, err)
		} else {
			rv = append(rv, c)
		}
	}
	return rv
}

// Images
//////////////////////////////////////////////////////////////////////////////

func (h *Host) GetImage(hash types.Hash, name types.ACIdentifier, labels types.Labels) (*Image, error) {
	if hash.Empty() && name.Empty() {
		return nil, ErrUsage
	}

	if !hash.Empty() {
		// TODO: get rid of GetImageByHash
		if img, err := h.GetImageByHash(hash); err == ErrNotFound {
			// TODO: autofetch?
			return nil, err
		} else if err != nil {
			return nil, errors.Trace(err)
		} else {
			// TODO: extract this sanity check to a separate function, or to method on Image
			if !name.Empty() && name != img.Manifest.Name {
				return nil, errors.Errorf("Name mismatch: image %v is %v, wanted %v", hash, img.Manifest.Name, name)
			}
			if !acutil.MatchLabels(labels, img.Manifest.Labels) {
				return nil, stderrors.New("Label mismatch")
			}
			return img, nil
		}
	} else if imgs, err := h.Images(); err != nil {
		return nil, errors.Trace(err)
	} else {
		for _, img := range imgs {
			if img.Manifest.Name != name {
				continue
			}
			if !acutil.MatchLabels(labels, img.Manifest.Labels) {
				continue
			}
			// TODO: multiple matches?
			return img, nil
		}

		// TODO: autofetch?
		return nil, ErrNotFound
	}
}

func (h *Host) GetImageByHash(hash types.Hash) (*Image, error) {
	if idStr, err := os.Readlink(h.Path("images", hash.String())); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		} else {
			return nil, errors.Trace(err)
		}
	} else {
		if id := uuid.Parse(idStr); id == nil {
			return nil, errors.Errorf("Invalid UUID: %v", idStr)
		} else {
			return LoadImage(h, id)
		}
	}
}

func (h *Host) Images() ([]*Image, error) {
	mm, _ := filepath.Glob(h.Path("images/*/manifest"))
	rv := make([]*Image, 0, len(mm))
	for _, m := range mm {
		d := filepath.Dir(m)
		if fi, err := os.Lstat(d); err != nil {
			return nil, err
		} else {
			if !fi.IsDir() {
				// This is a checksum symlink, skip it.
				// TODO: are checksum symlinks useful, or harmful by not being DRY?
				continue
			}
		}

		if id := uuid.Parse(filepath.Base(d)); id == nil {
			return nil, errors.Errorf("Invalid UUID: %#v", filepath.Base(d))
		} else if img, err := LoadImage(h, id); err != nil {
			id := filepath.Base(d)
			if img != nil {
				id = img.UUID.String()
			}
			h.ui.Printf("WARNING: images/%v: %v", id, err)
		} else {
			rv = append(rv, img)
		}
	}
	return rv, nil
}

// TODO: setting with flag override, also for allow-http
var flagAllowNoSignature = false

func AllowNoSignatureFlag(fl *flag.FlagSet) {
	if fl == nil {
		fl = flag.CommandLine
	}
	fl.BoolVar(&flagAllowNoSignature, "insecure-allow-no-signature", false, "Allow non-signed images")
}

func (h *Host) FetchImage(name, sigLocation string) (*Image, error) {
	if name, aci, asc, err := fetch.OpenACI(name, sigLocation); err != nil {
		return nil, errors.Trace(err)
	} else {
		defer aci.Close()
		if asc == nil {
			if flagAllowNoSignature {
				h.ui.Println("WARNING: no signature, proceeding as requested")
			} else {
				return nil, errors.New("No signature")
			}
		} else {
			defer asc.Close()
		}
		return h.importImage(name, aci, asc)
	}
}

func (h *Host) doGetImageDependency(dep types.Dependency) (*Image, error) {
	if dep.ImageID != nil {
		// If the dependency has an ID, do we already have it?
		if img, err := h.GetImageByHash(*dep.ImageID); img != nil {
			return img, err
		} else if err != ErrNotFound {
			return nil, errors.Trace(err)
		}
	} else if imgs, err := h.Images(); err != nil {
		return nil, errors.Trace(err)
	} else {
	imgs:
		for _, img := range imgs {
			if img.Manifest.Name == dep.ImageName {
				for _, label := range dep.Labels {
					if val, ok := img.Manifest.Labels.Get(label.Name.String()); !ok || val != label.Value {
						continue imgs
					}
				}
				return img, nil
			}
		}
	}

	// No luck so far, try to discover the dependency
	app := discovery.App{Name: dep.ImageName, Labels: make(map[types.ACIdentifier]string)}
	for _, label := range dep.Labels {
		app.Labels[label.Name] = label.Value
	}
	if aci, asc, err := fetch.DiscoverACI(app); err != nil {
		return nil, errors.Trace(err)
	} else {
		return h.importImage(dep.ImageName, aci, asc)
	}
}

func (h *Host) GetImageDependency(dep *types.Dependency) (*Image, error) {
	if img, err := h.doGetImageDependency(*dep); err != nil {
		return nil, errors.Trace(err)
	} else {
		// Double check the image vs spec
		if dep.ImageID != nil && *img.Hash != *dep.ImageID {
			return nil, errors.Errorf("Dependency specified hash %v, we got %v", dep.ImageID, img.Hash)
		}
		if dep.ImageName != img.Manifest.Name {
			return nil, errors.Errorf("Dependency specified app %v, got image named %v", dep.ImageName, img.Manifest.Name)
		}
		for _, label := range dep.Labels {
			if val, ok := img.Manifest.Labels.Get(label.Name.String()); !ok {
				return nil, errors.Errorf("Image doesn't have needed label %v", label.Name)
			} else if val != label.Value {
				return nil, errors.Errorf("Requested label %v=%#v, got value %#v instead", label.Name, label.Value, val)
			}
		}
		return img, nil
	}
}

func (h *Host) importImage(name types.ACIdentifier, aci, asc *os.File) (_ *Image, erv error) {
	newId := uuid.NewRandom()
	newIdStr := newId.String()
	ui := ui.NewUI("magenta", "import", newIdStr)
	if name.Empty() {
		ui.Println("Starting import")
	} else {
		ui.Printf("Starting import of %v", name)
	}
	if asc != nil {
		ui.Debug("Checking signature")
		didKeyDiscovery := false
		ks := h.Keystore()
	checkSig:
		if ety, err := ks.CheckSignature(name, aci, asc); err == openpgp_err.ErrUnknownIssuer && !didKeyDiscovery {
			ui.Println("Image signed by an unknown issuer, attempting to discover public key...")
			if err := h.TrustKey(name, "", ""); err != nil {
				return nil, errors.Trace(err)
			}
			didKeyDiscovery = true
			aci.Seek(0, os.SEEK_SET)
			asc.Seek(0, os.SEEK_SET)
			goto checkSig
		} else if err != nil {
			return nil, errors.Trace(err)
		} else {
			ui.Println("Valid signature for", name, "by:")
			ui.Println(keystore.KeyDescription(ety)) // FIXME:ui

			aci.Seek(0, os.SEEK_SET)
			asc.Seek(0, os.SEEK_SET)
		}
	} else {
		ui.Debug("No signature to check")
	}

	img := NewImage(h, newId)

	defer func() {
		if erv != nil {
			img.Destroy()
		}
	}()

	if err := os.MkdirAll(img.Path(), 0700); err != nil {
		return nil, errors.Trace(err)
	}

	// Save copy of the signature
	if asc != nil {
		ui.Debug("Saving signature copy")
		if ascCopy, err := os.OpenFile(img.Path("aci.asc"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0400); err != nil {
			return nil, errors.Trace(err)
		} else {
			_, err := io.Copy(ascCopy, asc)
			ascCopy.Close()
			if err != nil {
				return nil, errors.Trace(err)
			}
		}
	}

	// Load manifest
	ui.Debug("Loading manifest")
	manifestBytes, err := run.Command("tar", "-xOqf", "-", "manifest").ReadFrom(aci).Output()
	if err != nil {
		return nil, errors.Trace(err)
	}
	aci.Seek(0, os.SEEK_SET)

	if err = json.Unmarshal(manifestBytes, &img.Manifest); err != nil {
		return nil, errors.Trace(err)
	}

	if !name.Empty() && name != img.Manifest.Name {
		return nil, errors.Errorf("ACI name mismatch: downloaded %#v, got %#v instead", name, img.Manifest.Name)
	}

	if len(img.Manifest.Dependencies) == 0 {
		ui.Debug("No dependencies to fetch")
		if _, err := h.Dataset.CreateDataset(path.Join("images", newIdStr), "-o", "mountpoint="+h.Dataset.Path("images", newIdStr, "rootfs")); err != nil {
			return nil, errors.Trace(err)
		}
	} else {
		for i, dep := range img.Manifest.Dependencies {
			ui.Println("Looking for dependency:", dep.ImageName, dep.Labels, dep.ImageID)
			if dimg, err := h.GetImageDependency(&dep); err != nil {
				return nil, errors.Trace(err)
			} else {
				// We get a copy of the dependency struct when iterating, not
				// a pointer to it. We need to write to the slice's index to
				// save the hash to the real manifest.
				img.Manifest.Dependencies[i].ImageID = dimg.Hash
				if i == 0 {
					ui.Printf("Cloning parent %v as base rootfs\n", dimg)
					if ds, err := dimg.Clone(path.Join(h.Dataset.Name, "images", newIdStr), h.Dataset.Path("images", newIdStr, "rootfs")); err != nil {
						return nil, errors.Trace(err)
					} else {
						img.rootfs = ds
					}
				} else {
					return nil, errors.New("Not implemented")
				}
			}
		}
	}

	if err := img.saveManifest(); err != nil {
		return nil, errors.Trace(err)
	}

	ui.Println("Unpacking rootfs")

	// Save us a copy of the original, compressed ACI
	aciCopy, err := os.OpenFile(img.Path("aci"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0400)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer aciCopy.Close()
	aciZRd := io.TeeReader(fetch.ProgressBarFileReader(aci), aciCopy)

	// Decompress tarball for checksum
	aciRd, err := DecompressingReader(aciZRd)
	if err != nil {
		return nil, errors.Trace(err)
	}
	hash := sha512.New()
	aciRd = io.TeeReader(aciRd, hash)

	// Unpack the image. We trust system's tar, no need to roll our own
	untarCmd := run.Command("tar", "-C", img.Path(), "-xf", "-", "rootfs")
	untar, err := untarCmd.StdinPipe()
	if err != nil {
		return nil, errors.Trace(err)
	}

	if err := untarCmd.Start(); err != nil {
		return nil, errors.Trace(err)
	}
	// FIXME: defer killing process if survived

	if _, err := io.Copy(untar, aciRd); err != nil {
		return nil, errors.Trace(err)
	}

	if err := untar.Close(); err != nil {
		return nil, errors.Trace(err)
	}

	if err := untarCmd.Wait(); err != nil {
		return nil, errors.Trace(err)
	}

	if hash, err := types.NewHash(fmt.Sprintf("sha512-%x", hash.Sum(nil))); err != nil {
		// CAN'T HAPPEN
		return nil, errors.Trace(err)
	} else {
		ui.Println("Successfully imported", hash)
		img.Hash = hash
	}

	// TODO: enforce PathWhiteList

	if err := img.sealImage(); err != nil {
		return nil, errors.Trace(err)
	}

	return img, nil
}

// Keystore and trust
//////////////////////////////////////////////////////////////////////////////

func (h *Host) Keystore() *keystore.Keystore {
	return keystore.New(h.Path("keys"))
}

func (h *Host) TrustKey(prefix types.ACIdentifier, location, fingerprint string) error {
	if location == "" {
		if prefix == keystore.Root {
			return errors.New("Cannot discover root key!")
		}
		location = prefix.String()
	}

	_, kf, err := fetch.OpenPubKey(location)
	if err != nil {
		return errors.Trace(err)
	}

	defer kf.Close()

	path, err := h.Keystore().StoreTrustedKey(prefix, kf, fingerprint)
	if err != nil {
		return errors.Trace(err)
	}

	if path == "" {
		h.ui.Println("Key NOT accepted")
	} else {
		h.ui.Printf("Key accepted and saved as %v\n", path)
	}

	return nil
}
