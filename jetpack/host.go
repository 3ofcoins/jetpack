package jetpack

import "encoding/json"
import stderrors "errors"
import "fmt"
import "io/ioutil"
import "net"
import "net/url"
import "os"
import "path"
import "path/filepath"
import "strconv"
import "strings"
import "time"

import "github.com/appc/spec/schema"
import "github.com/appc/spec/schema/types"
import "github.com/juju/errors"
import "github.com/magiconair/properties"

import "../run"
import "../zfs"

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
}

func NewHost(configPath string) (*Host, error) {
	h := Host{}
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

	return nil
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
	if ifi, err := net.InterfaceByName(h.Properties.MustGetString("jail.interface")); err != nil {
		return nil, errors.Trace(err)
	} else {
		if addrs, err := ifi.Addrs(); err != nil {
			return nil, errors.Trace(err)
		} else {
			if ip, ipnet, err := net.ParseCIDR(addrs[0].String()); err != nil {
				return nil, errors.Trace(err)
			} else {
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
		}
	}
}

func (h *Host) NewPod() *Pod {
	return NewPod(h, ZeroUUID)
}

func (h *Host) CreatePod(pm *schema.PodManifest) (*Pod, error) {
	if len(pm.Apps) != 1 {
		return nil, errors.New("Only single application pods are supported")
	}

	// FIXME: method for that?
	c := &Pod{Host: h, Manifest: *pm}

	for _, app := range pm.Apps {
		uuid_str, err := os.Readlink(h.Dataset.Path("images", app.Image.ID.String()))
		if err != nil {
			return nil, errors.Trace(err)
		}

		id, err := types.NewUUID(uuid_str)
		if err != nil {
			panic(fmt.Sprintf("Invalid UUID: %#v", uuid_str))
		}

		img, err := h.GetImage(*id)
		if err != nil {
			return nil, errors.Trace(err)
		}

		// FIXME: code until end of `for` depends on len(pm.Apps)==1

		ds, err := img.Clone(path.Join(h.Dataset.Name, "pods", c.Manifest.UUID.String()), c.Path("rootfs"))
		if err != nil {
			return nil, errors.Trace(err)
		}

		if img.Manifest.App != nil {
			for _, mnt := range img.Manifest.App.MountPoints {
				// TODO: host volumes
				if err := os.MkdirAll(ds.Path(mnt.Path), 0755); err != nil {
					return nil, errors.Trace(err)
				}
			}
			if os_, _ := img.Manifest.GetLabel("os"); os_ == "linux" {
				for _, dir := range []string{"sys", "proc"} {
					if err := os.MkdirAll(ds.Path(dir), 0755); err != nil && !os.IsExist(err) {
						return nil, errors.Trace(err)
					}
				}
			}
		}
	}

	// TODO: lock until saved?
	if ip, err := h.nextIP(); err != nil {
		return nil, errors.Trace(err)
	} else {
		c.Manifest.Annotations.Set("ip-address", ip.String())
	}

	if err := c.Save(); err != nil {
		return nil, errors.Trace(err)
	}

	return c, nil
}

func (h *Host) ClonePod(img *Image) (*Pod, error) {
	// DEPRECATED
	return h.CreatePod(PodManifest([]*Image{img}))
}

func (h *Host) GetPod(id types.UUID) (*Pod, error) {
	if c, err := LoadPod(h, id); err != nil {
		return nil, errors.Trace(err)
	} else {
		return c, nil
	}
}

func (h *Host) FindPod(query string) (*Pod, error) {
	if id, err := types.NewUUID(query); err == nil {
		return h.GetPod(*id)
	}
	return nil, ErrNotFound
}

func (h *Host) Pods() PodSlice {
	mm, _ := filepath.Glob(h.Path("pods/*/manifest"))
	rv := make(PodSlice, 0, len(mm))
	for _, m := range mm {
		if id, err := types.NewUUID(filepath.Base(filepath.Dir(m))); err != nil {
			panic(err)
		} else if c, err := h.GetPod(*id); err != nil {
			fmt.Fprintf(os.Stderr, "%v: WARNING: %v\n", c.Manifest.UUID, err)
		} else {
			rv = append(rv, c)
		}
	}
	return rv
}

func (h *Host) GetImage(id types.UUID) (*Image, error) {
	dsName := h.Dataset.ChildName("images")
	if lines, err := zfs.ZfsFields("list", "-tfilesystem", "-d1", "-oname", dsName); err != nil {
		return nil, errors.Trace(err)
	} else {
		for _, ln := range lines {
			if ln[0] == dsName {
				continue
			}
			if curId, err := types.NewUUID(path.Base(ln[0])); err != nil {
				return nil, errors.Trace(err)
			} else if *curId == id {
				if ds, err := zfs.GetDataset(ln[0]); err != nil {
					return nil, errors.Trace(err)
				} else if id, err := types.NewUUID(filepath.Base(ds.Name)); err != nil {
					return nil, errors.Trace(err)
				} else {
					img := NewImage(h, *id)
					if err := img.Load(); err != nil {
						return nil, errors.Trace(err)
					} else {
						return img, nil
					}
				}
			}
		}
	}
	return nil, ErrNotFound
}

func (h *Host) GetImageByHash(hash types.Hash) (*Image, error) {
	if idStr, err := os.Readlink(h.Path("images", hash.String())); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		} else {
			return nil, errors.Trace(err)
		}
	} else {
		if id, err := types.NewUUID(idStr); err != nil {
			return nil, errors.Errorf("Invalid UUID: %v", idStr)
		} else {
			return h.GetImage(*id)
		}
	}
}

func (h *Host) Images() ImageSlice {
	mm, _ := filepath.Glob(h.Path("images/*/manifest"))
	rv := make(ImageSlice, 0, len(mm))
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

		if id, err := types.NewUUID(filepath.Base(d)); err != nil {
			panic(err)
		} else if img, err := h.GetImage(*id); err != nil {
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

func (h *Host) FindImages(query string) (ImageSlice, error) {
	// Empty query means all images
	if query == "" {
		if imgs := h.Images(); len(imgs) == 0 {
			return nil, ErrNotFound
		} else {
			return imgs, nil
		}
	}

	// Try UUID
	if id, err := types.NewUUID(query); err == nil {
		if img, err := h.GetImage(*id); err != nil {
			return nil, errors.Trace(err)
		} else {
			return ImageSlice{img}, nil
		}
	}

	// We'll search for images, let's prepare the list now
	imgs := h.Images()

	// Try hash
	if hash, err := types.NewHash(query); err == nil {
		for _, img := range imgs {
			if img.Hash != nil && *img.Hash == *hash {
				return ImageSlice{img}, nil
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

	rv := ImageSlice{}
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
	if id, err := types.NewUUID(query); err == nil {
		if img, err := h.GetImage(*id); err != nil {
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

func (h *Host) ImportImage(imageUri, manifestUri string) (*Image, error) {
	newId := RandomUUID()
	newIdStr := newId.String()
	if _, err := h.Dataset.CreateDataset(Path.join("images", newIdStr), "-o", "mountpoint="+h.Dataset.Path("images", newIdStr, "rootfs")); err != nil {
		return nil, errors.Trace(err)
	}

	img := NewImage(h, newId)
	img.Origin = imageUri
	img.Timestamp = time.Now()

	if manifestUri == "" {
		if hash, err := UnpackImage(imageUri, img.Path(), img.Path("ami")); err != nil {
			return nil, errors.Trace(err)
		} else {
			img.Hash = hash
		}
	} else {
		if _, err := UnpackImage(imageUri, img.Path("rootfs"), ""); err != nil {
			return nil, errors.Trace(err)
		}

		manifestBytes, err := run.Command("fetch", "-o", "-", manifestUri).Output()
		if err != nil {
			return nil, errors.Trace(err)
		}

		// Construct final manifest
		// FIXME: this may be somehow merged with build, and final manifest should be validated
		manifest := map[string]interface{}{
			"acKind":    "ImageManifest",
			"acVersion": schema.AppContainerVersion,
		}

		if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
			return nil, errors.Trace(err)
		}

		if manifest["annotations"] == nil {
			manifest["annotations"] = make([]interface{}, 0)
		}

		manifest["annotations"] = append(manifest["annotations"].([]interface{}),
			map[string]interface{}{"name": "timestamp", "value": time.Now()})

		if manifestBytes, err := json.Marshal(manifest); err != nil {
			return nil, errors.Trace(err)
		} else {
			if err := ioutil.WriteFile(img.Path("manifest"), manifestBytes, 0400); err != nil {
				return nil, errors.Trace(err)
			}
		}
	}

	if err := img.Seal(); err != nil {
		return nil, errors.Trace(err)
	}

	return img, nil
}
