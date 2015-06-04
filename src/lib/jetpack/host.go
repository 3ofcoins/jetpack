package jetpack

import (
	"crypto/sha512"
	stderrors "errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.google.com/p/go-uuid/uuid"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/juju/errors"
	"github.com/magiconair/properties"
	openpgp_err "golang.org/x/crypto/openpgp/errors"

	"lib/fetch"
	"lib/keystore"
	"lib/run"
	"lib/zfs"
)

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

	if ds, err := zfs.GetDataset(h.Properties.MustGetString("root.zfs")); err != nil {
		if err == zfs.ErrNotFound {
			return &h, nil
		}
		return nil, err
	} else {
		h.Dataset = ds
	}

	return &h, nil
}

func (h *Host) Path(elem ...string) string {
	return h.Dataset.Path(elem...)
}

func (h *Host) zfsOptions(prefix string, opts ...string) []string {
	l := len(prefix)
	p := h.Properties.FilterPrefix(prefix)
	for _, k := range p.Keys() {
		if v, ok := p.Get(k); ok && v != "" {
			opts = append(opts, strings.Join([]string{"-o", k[l:], "=", v}, ""))
		}
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

	if ds, err := zfs.CreateDataset(h.Properties.MustGetString("root.zfs"), h.zfsOptions("root.zfs.", "-p")...); err != nil {
		return errors.Trace(err)
	} else {
		h.Dataset = ds
	}

	if _, err := h.Dataset.CreateDataset("images", h.zfsOptions("images.zfs.")...); err != nil {
		return errors.Trace(err)
	}

	if _, err := h.Dataset.CreateDataset("pods", h.zfsOptions("pods.zfs.")...); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (h *Host) Keystore() *keystore.Keystore {
	return keystore.New(h.Path("keys"))
}

func (h *Host) GetMDSUGID() (int, int) {
	if h.mdsUid < 0 {
		u, err := user.Lookup(h.Properties.MustGetString("mds.user"))
		if err != nil {
			panic(err)
		}
		h.mdsUid, err = strconv.Atoi(u.Uid)
		if err != nil {
			panic(err)
		}
		h.mdsGid, err = strconv.Atoi(u.Gid)
		if err != nil {
			panic(err)
		}
	}
	return h.mdsUid, h.mdsGid
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

func (h *Host) FindPod(query string) (*Pod, error) {
	if id := uuid.Parse(query); id != nil {
		return h.GetPod(id)
	}
	return nil, ErrNotFound
}

func (h *Host) Pods() []*Pod {
	mm, _ := filepath.Glob(h.Path("pods/*/manifest"))
	rv := make([]*Pod, 0, len(mm))
	for _, m := range mm {
		if id := uuid.Parse(filepath.Base(filepath.Dir(m))); id == nil {
			panic(fmt.Sprintf("Invalid UUID: %#v", filepath.Base(filepath.Dir(m))))
		} else if c, err := h.GetPod(id); err != nil {
			fmt.Fprintf(os.Stderr, "%v: WARNING: %v\n", c.UUID, err)
		} else {
			rv = append(rv, c)
		}
	}
	return rv
}

func (h *Host) GetImageByUUID(id uuid.UUID) (*Image, error) {
	return LoadImage(h, id)
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
			return h.GetImageByUUID(id)
		}
	}
}

func (h *Host) Images() []*Image {
	mm, _ := filepath.Glob(h.Path("images/*/manifest"))
	rv := make([]*Image, 0, len(mm))
	for _, m := range mm {
		d := filepath.Dir(m)
		if fi, err := os.Lstat(d); err != nil {
			panic(err)
		} else {
			if !fi.IsDir() {
				// This is a checksum symlink, skip it.
				// TODO: are checksum symlinks useful, or harmful by not being DRY?
				continue
			}
		}

		if id := uuid.Parse(filepath.Base(d)); id == nil {
			panic(fmt.Sprintf("Invalid UUID: %#v", filepath.Base(d)))
		} else if img, err := h.GetImageByUUID(id); err != nil {
			id := filepath.Base(d)
			if img != nil {
				id = img.UUID.String()
			}
			fmt.Fprintf(os.Stderr, "%v: WARNING: %v\n", id, err)
		} else {
			rv = append(rv, img)
		}
	}
	return rv
}

func (h *Host) FindImages(query string) ([]*Image, error) {
	// Empty query means all images
	if query == "" {
		if imgs := h.Images(); len(imgs) == 0 {
			return nil, ErrNotFound
		} else {
			return imgs, nil
		}
	}

	// Try UUID
	if id := uuid.Parse(query); id != nil {
		if img, err := h.GetImageByUUID(id); err != nil {
			return nil, errors.Trace(err)
		} else {
			return []*Image{img}, nil
		}
	}

	// We'll search for images, let's prepare the list now
	imgs := h.Images()

	// Try hash
	if hash, err := types.NewHash(query); err == nil {
		for _, img := range imgs {
			if img.Hash != nil && *img.Hash == *hash {
				return []*Image{img}, nil
			}
		}
		return nil, ErrNotFound
	}

	// Bad luck, we have a query. Let's transform it into a query string and parse it this way…
	query = strings.Replace(query, ":", ",version=", 1)
	query = strings.Replace(query, ",", "&", -1)
	query = "name=" + query
	v, err := url.ParseQuery(query)
	if err != nil {
		return nil, err
	}

	name := types.ACName(v["name"][0])
	delete(v, "name")

	rv := []*Image{}
images:
	for _, img := range imgs {
		if img.Manifest.Name == name {
		labels:
			for label, values := range v {
				if imgvalue, ok := img.Manifest.GetLabel(label); ok {
					for _, value := range values {
						if imgvalue == value {
							// We got a good value, next label
							continue labels
						}
					}
					// No good values were found, next image
					continue images
				} else {
					continue images
				}
			}
			// If we got here, image was not rejected, so it's a good one.
			rv = append(rv, img)
		}
	}

	if len(rv) == 0 {
		return nil, ErrNotFound
	} else {
		return rv, nil
	}
}

func (h *Host) FindImage(query string) (*Image, error) {
	// Optimize for simple case
	if id := uuid.Parse(query); id != nil {
		if img, err := h.GetImageByUUID(id); err != nil {
			return nil, errors.Trace(err)
		} else {
			return img, nil
		}
	}

	if imgs, err := h.FindImages(query); err != nil {
		return nil, err
	} else {
		if len(imgs) == 1 {
			return imgs[0], nil
		} else {
			return nil, ErrManyFound
		}
	}
}

func (h *Host) TrustKey(prefix types.ACName, location string) error {
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

	// TODO: --yes
	path, err := h.Keystore().StoreTrustedKey(prefix, kf, false)
	if err != nil {
		return errors.Trace(err)
	}

	if path == "" {
		fmt.Println("Key NOT accepted")
	} else {
		fmt.Printf("Key accepted and saved as %v\n", path)
	}

	return nil
}

func (h *Host) FetchImage(name, sigLocation string) (*Image, error) {
	if name, aci, asc, err := fetch.OpenACI(name, sigLocation); err != nil {
		return nil, errors.Trace(err)
	} else {
		defer aci.Close()
		if asc == nil {
			return nil, errors.New("No signature")
		}
		defer asc.Close()
		return h.importImage(name, aci, asc)
	}
}

func (h *Host) importImage(name types.ACName, aci, asc *os.File) (_ *Image, erv error) {
	if asc != nil {
		didKeyDiscovery := false
		ks := h.Keystore()
	checkSig:
		if ety, err := ks.CheckSignature(name, aci, asc); err == openpgp_err.ErrUnknownIssuer && !didKeyDiscovery {
			fmt.Println("Image signed by an unknown issuer, attempting to discover public key...")
			if err := h.TrustKey(name, ""); err != nil {
				return nil, errors.Trace(err)
			}
			didKeyDiscovery = true
			aci.Seek(0, os.SEEK_SET)
			asc.Seek(0, os.SEEK_SET)
			goto checkSig
		} else if err != nil {
			return nil, errors.Trace(err)
		} else {
			fmt.Println("Valid signature for", name, "by:")
			fmt.Println(keystore.KeyDescription(ety))

			aci.Seek(0, os.SEEK_SET)
			asc.Seek(0, os.SEEK_SET)
		}
	}

	newId := uuid.NewRandom()
	newIdStr := newId.String()
	if _, err := h.Dataset.CreateDataset(path.Join("images", newIdStr), "-o", "mountpoint="+h.Dataset.Path("images", newIdStr, "rootfs")); err != nil {
		return nil, errors.Trace(err)
	}

	img := NewImage(h, newId)
	img.Timestamp = time.Now()

	defer func() {
		if erv != nil {
			img.Destroy()
		}
	}()

	// Save copy of the signature
	if asc != nil {
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

	fmt.Println("Importing image ...")

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
	untarCmd := run.Command("tar", "-C", img.Path(), "-xf", "-", "manifest", "rootfs")
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
		img.Hash = hash
	}

	// TODO: load manifest, check name before sealing, enforce PathWhiteList

	if err := img.Seal(); err != nil {
		return nil, errors.Trace(err)
	}

	if name != ACNoName && name != img.Manifest.Name {
		return nil, errors.Errorf("ACI name mismatch: downloaded %#v, got %#v instead", name, img.Manifest.Name)
	}

	return img, nil
}
