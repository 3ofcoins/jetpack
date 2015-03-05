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

import "code.google.com/p/go-uuid/uuid"
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

	containersDS *zfs.Dataset
	imagesDS     *zfs.Dataset
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

	if ds, err := h.Dataset.GetDataset("images"); err != nil {
		return nil, err
	} else {
		h.imagesDS = ds
	}

	if ds, err := h.Dataset.GetDataset("containers"); err != nil {
		return nil, err
	} else {
		h.containersDS = ds
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

	if ds, err := h.Dataset.CreateDataset("images", h.zfsOptions("images.zfs.")...); err != nil {
		return errors.Trace(err)
	} else {
		h.imagesDS = ds
	}

	if ds, err := h.Dataset.CreateDataset("containers", h.zfsOptions("containers.zfs.")...); err != nil {
		return errors.Trace(err)
	} else {
		h.containersDS = ds
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
				for _, c := range h.Containers() {
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

func (h *Host) CreateContainer(crm *schema.ContainerRuntimeManifest) (*Container, error) {
	if len(crm.Apps) != 1 {
		return nil, errors.New("Only single application containers are supported")
	}

	c := NewContainer(h, uuid.Parse(crm.UUID.String()))
	c.Manifest = *crm

	for _, app := range crm.Apps {
		uuid_str, err := os.Readlink(h.imagesDS.Path(app.ImageID.String()))
		if err != nil {
			return nil, errors.Trace(err)
		}

		uuid := uuid.Parse(uuid_str)
		if uuid == nil {
			panic(fmt.Sprintf("Invalid UUID: %#v", uuid_str))
		}

		img, err := h.GetImage(uuid)
		if err != nil {
			return nil, errors.Trace(err)
		}

		// FIXME: code until end of `for` depends on len(crm.Apps)==1

		ds, err := img.Clone(path.Join(h.containersDS.Name, c.UUID.String()), c.Path("rootfs"))
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

		c.image = img
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

func (h *Host) CloneContainer(img *Image) (*Container, error) {
	// DEPRECATED
	return h.CreateContainer(ContainerRuntimeManifest([]*Image{img}))
}

func (h *Host) GetContainer(id uuid.UUID) (*Container, error) {
	c := NewContainer(h, id)
	if err := c.Load(); err != nil {
		return nil, errors.Trace(err)
	}
	return c, nil
}

func (h *Host) FindContainer(query string) (*Container, error) {
	if id := uuid.Parse(query); id != nil {
		return h.GetContainer(id)
	}
	return nil, ErrNotFound
}

func (h *Host) Containers() ContainerSlice {
	mm, _ := filepath.Glob(h.Path("containers/*/manifest"))
	rv := make(ContainerSlice, 0, len(mm))
	for _, m := range mm {
		c := NewContainer(h, uuid.Parse(filepath.Base(filepath.Dir(m))))
		if err := c.Load(); err != nil {
			fmt.Fprintf(os.Stderr, "%v: WARNING: %v\n", c.UUID, err)
		} else {
			rv = append(rv, c)
		}
	}
	return rv
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

		img := NewImage(h, uuid.Parse(filepath.Base(d)))
		if err := img.Load(); err != nil {
			fmt.Fprintf(os.Stderr, "%v: WARNING: %v\n", img.UUID, err)
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
	if uuid := uuid.Parse(query); uuid != nil {
		if img, err := h.GetImage(uuid); err != nil {
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
	if id := uuid.Parse(query); id != nil {
		if img, err := h.GetImage(id); err != nil {
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

func (h *Host) GetImage(id uuid.UUID) (*Image, error) {
	if lines, err := h.imagesDS.ZfsFields("list", "-tfilesystem", "-d1", "-oname"); err != nil {
		return nil, errors.Trace(err)
	} else {
		for _, ln := range lines {
			if uuid.Equal(id, uuid.Parse(path.Base(ln[0]))) {
				if ds, err := zfs.GetDataset(ln[0]); err != nil {
					return nil, errors.Trace(err)
				} else {
					img := NewImage(h, uuid.Parse(filepath.Base(ds.Name)))
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

func (h *Host) ImportImage(imageUri, manifestUri string) (*Image, error) {
	newId := uuid.NewRandom()
	newIdStr := newId.String()
	if _, err := h.imagesDS.CreateDataset(newIdStr, "-o", "mountpoint="+h.imagesDS.Path(newIdStr, "rootfs")); err != nil {
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
