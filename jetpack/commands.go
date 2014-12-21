package jetpack

import "encoding/json"
import "fmt"
import "io/ioutil"
import "os"
import "path"
import "path/filepath"
import "sort"
import "strconv"
import "time"

import "github.com/appc/spec/schema"
import "github.com/appc/spec/schema/types"
import "github.com/juju/errors"

import "github.com/3ofcoins/jetpack/cli"

func (rt *Runtime) CmdInit() error {
	mountpoint := ""
	switch len(rt.Args) {
	case 0: // pass
	case 1:
		mountpoint = rt.Args[0]
	default:
		return cli.ErrUsage
	}

	if host, err := CreateHost(rt.ZFSRoot, mountpoint); err != nil {
		return errors.Trace(err)
	} else {
		rt.host = host
	}
	return rt.CmdInfo()
}

func (rt *Runtime) CmdImport() error {
	if len(rt.Args) != 1 {
		return cli.ErrUsage
	}

	aciAddr := rt.Args[0]

	rt.UI.Sayf("Importing image from %s", aciAddr)
	img, err := rt.Host().Images.Import(aciAddr)
	if err != nil {
		return errors.Trace(err)
	} else {
		rt.UI.Sayf("Imported image %v", img.UUID)
	}

	return nil
}

func (rt *Runtime) listImages() error {
	ii, err := rt.Host().Images.All()
	if err != nil {
		return errors.Trace(err)
	}
	if len(ii) == 0 {
		rt.UI.Say("No images")
		return nil
	}
	sort.Sort(ii)
	rt.UI.Table(ii.Table())

	return nil
}

func (rt *Runtime) listContainers() error {
	cc, err := rt.Host().Containers.All()
	if err != nil {
		return errors.Trace(err)
	}
	if len(cc) == 0 {
		rt.UI.Say("No containers")
		return nil
	}
	sort.Sort(cc)
	rt.UI.Table(cc.Table())

	return nil
}

func (rt *Runtime) CmdList() error {
	switch len(rt.Args) {
	case 0:
		return untilError(
			func() error { return rt.UI.Section("Images", rt.listImages) },
			func() error { return rt.UI.Section("Containers", rt.listContainers) },
		)
	case 1:
		switch rt.Args[0] {
		case "images":
			return rt.listImages()
		case "containers":
			return rt.listContainers()
		default:
			return cli.ErrUsage
		}
	default:
		return cli.ErrUsage
	}
}

func (rt *Runtime) byUUID(
	uuid string,
	handleContainer func(*Container) error,
	handleImage func(*Image) error,
) error {
	h := rt.Host()

	// TODO: distinguish "not found" from actual errors
	if c, err := h.Containers.Get(uuid); err == nil {
		return handleContainer(c)
	}

	if i, err := h.Images.Get(uuid); err == nil {
		return handleImage(i)
	}

	return errors.Errorf("Not found: %#v", uuid)
}

func (rt *Runtime) CmdRm() error {
	for _, uuid := range rt.Args {
		if err := rt.byUUID(uuid,
			func(c *Container) error { return c.Destroy() },
			func(i *Image) error { return i.Destroy() },
		); err != nil {
			return err
		}
	}
	return nil
}

func (rt *Runtime) CmdInfo() error {
	switch len(rt.Args) {
	case 0: // host info
		return errors.Trace(rt.Show(rt.Host()))
	case 1: // UUID
		return rt.byUUID(rt.Args[0],
			func(c *Container) error { return errors.Trace(rt.Show(c)) },
			func(i *Image) error { return errors.Trace(rt.Show(i)) },
		)
	default:
		return cli.ErrUsage
	}

	return nil
}

func (rt *Runtime) CmdRun() (err1 error) {
	if len(rt.Args) != 1 {
		return cli.ErrUsage
	}
	var app *types.App
	if rt.Console {
		app = ConsoleApp("root")
	}
	return errors.Trace(
		rt.byUUID(rt.Args[0],
			func(c *Container) error { return errors.Trace(c.Run(app)) },
			func(i *Image) error { return errors.Trace(i.Run(app, rt.Keep)) },
		))
}

func (rt *Runtime) CmdPs() error {
	c, err := rt.Host().Containers.Get(rt.Shift())
	if err != nil {
		return err
	}
	jid := c.Jid()
	if jid == 0 {
		return errors.Errorf("Container %s is not running", c.Manifest.UUID)
	}
	psArgs := []string{"-J", strconv.Itoa(jid)}
	psArgs = append(psArgs, rt.Args...)
	return runCommand("ps", psArgs...)
}

func (rt *Runtime) CmdBuild() error {
	var buildContainer *Container
	var parentImage *Image
	var h = rt.Host()
	var buildDir = rt.Shift()

	if rt.ImageName == "" {
		rt.UI.Say("Creating a new, empty container...")
		if c, err := h.Containers.Create(); err != nil {
			return errors.Trace(err)
		} else {
			buildContainer = c
		}
	} else {
		if img, err := h.Images.Get(rt.ImageName); err != nil {
			return errors.Trace(err)
		} else {
			rt.UI.Sayf("Cloning a container from image %v...", img.UUID)
			parentImage = img
			if c, err := h.Containers.Clone(img); err != nil {
				return errors.Trace(err)
			} else {
				buildContainer = c
			}
		}
	}

	// This is needed by freebsd-update at least, should be okay to
	// allow this in builders.
	buildContainer.JailParameters["allow.chflags"] = "true"

	rt.UI.Sayf("Working in container: %v", buildContainer.Manifest.UUID)

	destroot := buildContainer.Dataset.Path("rootfs")

	if rt.Tarball != "" {
		rt.UI.Sayf("Unpacking %v into %v...", rt.Tarball, destroot)
		runCommand("tar", "-C", destroot, "-xf", rt.Tarball)
	}

	workDir, err := ioutil.TempDir(destroot, ".jetpack.build.")
	if err != nil {
		return errors.Trace(err)
	}

	rt.UI.Sayf("Copying build directory %v to %v...", buildDir, workDir)
	if err := runCommand("cp", "-R", buildDir, workDir); err != nil {
		return errors.Trace(err)
	}

	rt.UI.Sayf("Starting the container")
	if err := buildContainer.RunJail("-c"); err != nil {
		return errors.Trace(err)
	}

	cWorkDir := filepath.Base(workDir)

	buildCommand := append([]string{
		"/bin/sh", "-c",
		fmt.Sprintf("cd '%s' && exec \"${@}\"", cWorkDir),
		"jetpack-build@" + cWorkDir,
	}, rt.Args...)

	rt.UI.Sayf("Running build command: %v", buildCommand)
	if err := buildContainer.RunJexec("", buildCommand); err != nil {
		return errors.Trace(err)
	}

	rt.UI.Say("Stopping the container")
	if err := buildContainer.RunJail("-r"); err != nil {
		return errors.Trace(err)
	}

	rt.UI.Say("Cleaning up")
	if err := os.Rename(
		filepath.Join(workDir, rt.Manifest),
		buildContainer.Dataset.Path("build.manifest"),
	); err != nil {
		return errors.Trace(err)
	}

	if err := os.RemoveAll(workDir); err != nil {
		return errors.Trace(err)
	}

	if err := os.Remove(filepath.Join(destroot, "/etc/resolv.conf")); err != nil {
		return errors.Trace(err)
	}

	rt.UI.Say("FIXME: Sleeping for 5 seconds")
	time.Sleep(5 * time.Second)

	rt.UI.Say("Committing build container as an image")
	uuid := path.Base(buildContainer.Dataset.Name)
	if err := buildContainer.Dataset.Rename(h.Images.Dataset.ChildName(uuid)); err != nil {
		return errors.Trace(err)
	}

	ds, err := h.Images.Dataset.GetDataset(uuid)
	if err != nil {
		return errors.Trace(err)
	}

	// Clean up container's runtime stuff, leave only rootfs and new manifest
	if entries, err := ioutil.ReadDir(ds.Mountpoint); err != nil {
		return errors.Trace(err)
	} else {
		for _, entry := range entries {
			filename := entry.Name()
			if filename == "rootfs" || filename == "build.manifest" {
				continue
			}
			if err := os.RemoveAll(ds.Path(filename)); err != nil {
				return errors.Trace(err)
			}
		}
	}

	img, err := NewImage(ds, &(h.Images))
	if err != nil {
		return errors.Trace(err)
	}

	// Construct the final manifest

	// defaults that are always present
	manifest := map[string]interface{}{
		"acKind":    "ImageManifest",
		"acVersion": schema.AppContainerVersion,
	}

	// Merge what the build directory has left for us
	if manifestBytes, err := ioutil.ReadFile(img.Dataset.Path("build.manifest")); err != nil {
		return errors.Trace(err)
	} else {
		if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
			return errors.Trace(err)
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
		return errors.Trace(err)
	} else {
		if err := ioutil.WriteFile(img.Dataset.Path("manifest"), manifestBytes, 0400); err != nil {
			return errors.Trace(err)
		}
	}

	if err := os.Remove(img.Dataset.Path("build.manifest")); err != nil {
		return errors.Trace(err)
	}

	img.Timestamp = time.Now()
	return errors.Trace(img.Seal())
}
