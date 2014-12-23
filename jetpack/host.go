package jetpack

import "encoding/json"
import stderrors "errors"
import "fmt"
import "io/ioutil"
import "log"
import "os"
import "path"
import "path/filepath"
import "time"

import "github.com/appc/spec/schema"
import "github.com/appc/spec/schema/types"
import "github.com/juju/errors"

const DefaultMountpoint = "/srv/jetpack"

var ErrNotFound = stderrors.New("Not found")
var ErrManyFound = stderrors.New("Multiple results found")

type Host struct {
	Dataset    *Dataset `json:"-"`
	Images     ImageManager
	Containers ContainerManager
}

var hostDefaults = Host{
	Containers: defaultContainerManager,
}

func GetHost(rootDataset string) (*Host, error) {
	ds, err := GetDataset(rootDataset)
	if err != nil {
		return nil, errors.Trace(err)
	}
	h := hostDefaults
	h.Dataset = ds

	if config, err := ioutil.ReadFile(h.Dataset.Path("config")); err != nil {
		if os.IsNotExist(err) {
			if err = h.SaveConfig(); err != nil {
				return nil, errors.Trace(err)
			}
			return &h, nil
		} else {
			return nil, errors.Trace(err)
		}
	} else {
		err = json.Unmarshal(config, &h)
		if err != nil {
			return nil, err
		}
	}

	h.Images.Host = &h
	if ds, err := h.Dataset.GetDataset("images"); err != nil {
		return nil, errors.Trace(err)
	} else {
		h.Images.Dataset = ds
	}

	h.Containers.Host = &h
	if ds, err := h.Dataset.GetDataset("containers"); err != nil {
		return nil, errors.Trace(err)
	} else {
		h.Containers.Dataset = ds
	}

	return &h, nil
}

var storageZFSProperties = map[string]string{
	"atime":    "off",
	"compress": "lz4",
	"dedup":    "on",
}

func CreateHost(rootDataset, rootMountpoint string) (*Host, error) {
	h := hostDefaults

	// Create root dataset
	if rootMountpoint == "" {
		rootMountpoint = DefaultMountpoint
	}

	log.Printf("Creating root ZFS dataset %#v at %v\n", rootDataset, rootMountpoint)
	if ds, err := CreateFilesystem(
		rootDataset,
		map[string]string{"mountpoint": rootMountpoint},
	); err != nil {
		return nil, errors.Trace(err)
	} else {
		h.Dataset = ds
	}

	if ds, err := h.Dataset.CreateFilesystem("images", storageZFSProperties); err != nil {
		return nil, errors.Trace(err)
	} else {
		h.Images.Dataset = ds
	}

	if ds, err := h.Dataset.CreateFilesystem("containers", nil); err != nil {
		return nil, errors.Trace(err)
	} else {
		h.Containers.Dataset = ds
	}

	// TODO: accept configuration
	if err := h.SaveConfig(); err != nil {
		return nil, errors.Trace(err)
	}

	return &h, nil
}

func (h *Host) SaveConfig() error {
	config, err := json.Marshal(h)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(h.Dataset.Path("config"), config, 0600)
}

// FIXME: using *Image here smells bad. Maybe we should go the Docker
// way, and separate "import" (from a tarball, via ImageManager) from
// "build" (from an image, via Image), with possible option to
// "squash" the inheritance (`zfs promote` the child image)?  Maybe we
// should just have a more sophisticated BuildConfig struct, rather
// than a simple command line?

func (h *Host) Build(parentImage *Image, tarball, buildDir string, buildExec []string) (*Image, error) {
	var buildContainer *Container
	if parentImage == nil {
		if c, err := h.Containers.Create(); err != nil {
			return nil, errors.Trace(err)
		} else {
			buildContainer = c
		}
	} else {
		if c, err := h.Containers.Clone(parentImage); err != nil {
			return nil, errors.Trace(err)
		} else {
			buildContainer = c
		}
	}

	// This is needed by freebsd-update at least, should be okay to
	// allow this in builders.
	buildContainer.JailParameters["allow.chflags"] = "true"

	destroot := buildContainer.Dataset.Path("rootfs")

	if tarball != "" {
		if err := runCommand("tar", "-C", destroot, "-xf", tarball); err != nil {
			return nil, errors.Trace(err)
		}
	}

	workDir, err := ioutil.TempDir(destroot, ".jetpack.build.")
	if err != nil {
		return nil, errors.Trace(err)
	}

	if err := runCommand("cp", "-R", buildDir, workDir); err != nil {
		return nil, errors.Trace(err)
	}

	cWorkDir := filepath.Base(workDir)
	buildApp := &types.App{
		Exec: append([]string{
			"/bin/sh", "-c",
			fmt.Sprintf("cd '%s' && exec \"${@}\"", cWorkDir),
			"jetpack-build@" + cWorkDir,
		}, buildExec...),
	}

	if err := buildContainer.Run(buildApp); err != nil {
		return nil, errors.Trace(err)
	}

	if err := os.Rename(
		filepath.Join(workDir, "manifest.json"),
		buildContainer.Dataset.Path("build.manifest"),
	); err != nil {
		return nil, errors.Trace(err)
	}

	if err := os.RemoveAll(workDir); err != nil {
		return nil, errors.Trace(err)
	}

	if err := os.Remove(filepath.Join(destroot, "/etc/resolv.conf")); err != nil {
		return nil, errors.Trace(err)
	}

	// FIXME HARD: sleep to avoid race condition before `zfs rename`
	time.Sleep(time.Second)

	// Pivot container into an image
	uuid := path.Base(buildContainer.Dataset.Name)
	if err := buildContainer.Dataset.Rename(h.Images.Dataset.ChildName(uuid)); err != nil {
		return nil, errors.Trace(err)
	}

	ds, err := h.Images.Dataset.GetDataset(uuid)
	if err != nil {
		return nil, errors.Trace(err)
	}

	// Clean up container's runtime stuff, leave only rootfs and new manifest
	if entries, err := ioutil.ReadDir(ds.Mountpoint); err != nil {
		return nil, errors.Trace(err)
	} else {
		for _, entry := range entries {
			filename := entry.Name()
			if filename == "rootfs" || filename == "build.manifest" {
				continue
			}
			if err := os.RemoveAll(ds.Path(filename)); err != nil {
				return nil, errors.Trace(err)
			}
		}
	}

	img, err := NewImage(ds, &(h.Images))
	if err != nil {
		return nil, errors.Trace(err)
	}

	// Construct the final manifest

	// defaults that are always present
	manifest := map[string]interface{}{
		"acKind":    "ImageManifest",
		"acVersion": schema.AppContainerVersion,
	}

	// Merge what the build directory has left for us
	if manifestBytes, err := ioutil.ReadFile(img.Dataset.Path("build.manifest")); err != nil {
		return nil, errors.Trace(err)
	} else {
		if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
			return nil, errors.Trace(err)
		}
	}

	// FIXME: the below is UGLY. Can we make it prettier?

	// Annotate with timestamp if there is no such annotation yet
	if manifest["annotations"] == nil {
		manifest["annotations"] = make(map[string]interface{})
	}

	annotations := manifest["annotations"].(map[string]interface{})
	if _, ok := annotations["timestamp"]; !ok {
		annotations["timestamp"] = time.Now()
	}

	if parentImage != nil {
		// Merge OS and arch from parent image if it's not set
		// TODO: can we prevent this?
		hasLabels := map[string]bool{}
		for _, labelI := range manifest["labels"].([]interface{}) {
			label := labelI.(map[string]interface{})
			hasLabels[label["name"].(string)] = true
		}

		if val, ok := parentImage.Manifest.GetLabel("os"); ok && !hasLabels["os"] {
			manifest["labels"] = append(manifest["labels"].([]interface{}),
				map[string]interface{}{"name": "os", "val": val})
		}

		if val, ok := parentImage.Manifest.GetLabel("arch"); ok && !hasLabels["arch"] {
			manifest["labels"] = append(manifest["labels"].([]interface{}),
				map[string]interface{}{"name": "arch", "val": val})
		}

		// TODO: merge app from parent image

		img.Origin = parentImage.UUID.String()
	} else {
		// img.Origin = …
	}

	if manifestBytes, err := json.Marshal(manifest); err != nil {
		return nil, errors.Trace(err)
	} else {
		if err := ioutil.WriteFile(img.Dataset.Path("manifest"), manifestBytes, 0400); err != nil {
			return nil, errors.Trace(err)
		}
	}

	if err := os.Remove(img.Dataset.Path("build.manifest")); err != nil {
		return nil, errors.Trace(err)
	}

	img.Timestamp = time.Now()

	if err := img.Seal(); err != nil {
		return nil, errors.Trace(err)
	}

	return img, nil
}
