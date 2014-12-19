package jetpack

import "fmt"
import "io/ioutil"
import "os"
import "path"
import "path/filepath"
import "sort"
import "strconv"
import "time"

import "github.com/juju/errors"

import "github.com/3ofcoins/jetpack/cli"

func (rt *Runtime) CmdInfo() error {
	h := rt.Host()
	if rt.ImageName == "" {
		rt.UI.Show(h)
		rt.UI.Show(nil)
	} else {
		img, err := h.Images.Get(rt.ImageName)
		if err != nil {
			return err
		}
		rt.UI.Show(img)
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
		rt.UI.Show(img)
	}

	return nil
}

func (rt *Runtime) CmdImages() error {
	if len(rt.Args) != 0 {
		return cli.ErrUsage
	}
	ii, err := rt.Host().Images.All()
	if err != nil {
		return errors.Trace(err)
	}
	sort.Sort(ii)
	if rt.Verbose {
		rt.UI.Show(ii)
	} else {
		rt.UI.Summarize(ii)
	}
	return nil
}

func (rt *Runtime) CmdContainers() error {
	if len(rt.Args) != 0 {
		return cli.ErrUsage
	}
	cc, err := rt.Host().Containers.All()
	if err != nil {
		return errors.Trace(err)
	}
	if rt.Verbose {
		rt.UI.Show(cc)
	} else {
		rt.UI.Summarize(cc)
	}
	return nil
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
		rt.UI.Show(c)
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
			rt.UI.Sayf("Cloning a container from image %v...", img.Summary())
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

	rt.UI.Sayf("Working in container: %v", buildContainer.Summary())

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

	if parentImage != nil {
		img.Manifest = parentImage.Manifest
		img.Origin = parentImage.UUID.String()
	} else {
		// img.Origin = …
	}

	img.Timestamp = time.Now()

	if err := img.readManifest(img.Dataset.Path("build.manifest")); err != nil {
		return errors.Trace(err)
	}

	if err := os.Remove(img.Dataset.Path("build.manifest")); err != nil {
		return errors.Trace(err)
	}

	return errors.Trace(img.Commit())
}
