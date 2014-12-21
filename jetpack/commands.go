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
import "github.com/juju/errors"

import "github.com/3ofcoins/jetpack/cli"

func (rt *Runtime) CmdInfo() error {
	h := rt.Host()
	switch len(rt.Args) {
	case 0: // host info
		rt.UI.Sayf("ZFS dataset: %v (%v)", h.Dataset.Name, h.Dataset.Mountpoint)
		rt.UI.Sayf("IP range: %v on %v", h.Containers.AddressPool, h.Containers.Interface)
	case 1: // UUID
		rt.UI.Say("FIXME")
	default:
		return cli.ErrUsage
	}

	return nil
}

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

	rows := make([][]string, len(ii)+1)
	rows[0] = []string{"UUID", "NAME", "LABELS"}
	for i, img := range ii {
		rows[i+1] = append([]string{img.UUID.String(), string(img.Manifest.Name)}, img.PrettyLabels()...)
	}

	rt.UI.Table(rows)

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

	rows := make([][]string, len(cc)+1)
	rows[0] = []string{"UUID", "NAME"}
	for i, c := range cc {
		name := " (anonymous)"
		if len(c.Manifest.Apps) > 0 {
			name = " " + string(c.Manifest.Apps[0].Name)
		}
		rows[i+1] = []string{c.Manifest.UUID.String(), name}
	}

	rt.UI.Table(rows)

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

func (rt *Runtime) CmdClone() error {
	if len(rt.Args) != 1 {
		return cli.ErrUsage
	}
	h := rt.Host()

	img, err := h.Images.Get(rt.Args[0])
	if err != nil {
		return errors.Trace(err)
	}
	if img == nil {
		return errors.Errorf("Image not found: %v", rt.Args[0])
	}

	if c, err := h.Containers.Clone(img); err != nil {
		return errors.Trace(err)
	} else {
		rt.UI.Sayf("Cloned container %v", c.Manifest.UUID)
		return nil
	}
}

func (rt *Runtime) CmdRunJail() error {
	if len(rt.Args) != 1 {
		return cli.ErrUsage
	}
	h := rt.Host()
	c, err := h.Containers.Get(rt.Args[0])
	if err != nil {
		return errors.Trace(err)
	}

	var op string
	switch rt.Command {
	case "start":
		op = "-c"
	case "stop":
		op = "-r"
	default:
		return errors.Errorf("Unrecognized command %#v", rt.Command)
	}

	return errors.Trace(c.RunJail(op))
}

func (rt *Runtime) CmdConsole() error {
	if len(rt.Args) == 0 {
		return cli.ErrUsage
	}
	c, err := rt.Host().Containers.Get(rt.Shift())
	if err != nil {
		return err
	}
	if c.Jid() == 0 {
		return errors.Errorf("Container %s is not started", c.Manifest.UUID)
	}

	args := rt.Args
	user := rt.User
	if len(args) == 0 {
		args = []string{"login", "-f", user}
		user = ""
	}
	if user == "root" {
		user = ""
	}
	return c.RunJexec(user, args)
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

	img, err := NewImage(ds)
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
