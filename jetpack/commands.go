package jetpack

import "fmt"
import "io/ioutil"
import "os"
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

	c, err := h.Containers.Clone(img, "aci")
	rt.UI.Show(c)

	return nil
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
	var img *Image
	var h = rt.Host()
	var buildDir = rt.Shift()

	if rt.ImageName == "" {
		rt.UI.Say("Creating a new, empty image...")
		if i, err := h.Images.Create(); err != nil {
			return errors.Trace(err)
		} else {
			img = i
		}

		if err := os.Mkdir(img.Dataset.Path("rootfs"), 0700); err != nil {
			return errors.Trace(err)
		}
	} else {
		// Start by cloning an existing image
		return errors.New("NFY")
	}

	rt.UI.Sayf("Created new image: %v", img.Summary())

	destroot := img.Dataset.Path("rootfs")

	if rt.Tarball != "" {
		rt.UI.Sayf("Unpacking %v into %v...", rt.Tarball, destroot)
		runCommand("tar", "-C", destroot, "-xf", rt.Tarball)
	}

	// FIXME: we're just chroot()ing into rootfs, rather than working
	// from a jail. It should be possible to clone image into a
	// temporary container, and then `zfs send/recv` rootfs changes…

	workDir, err := ioutil.TempDir(destroot, ".jetpack.build.")
	if err != nil {
		return errors.Trace(err)
	}

	rt.UI.Sayf("Copying build directory %v to %v...", buildDir, workDir)
	if err := runCommand("cp", "-R", buildDir, workDir); err != nil {
		return errors.Trace(err)
	}

	args := append([]string{
		destroot,
		"/bin/sh", "-c",
		fmt.Sprintf("cd %s && exec \"${@}\"", filepath.Base(workDir)),
		"_",
	}, rt.Args...)

	if err := runCommand("chroot", args...); err != nil {
		return errors.Trace(err)
	}

	if err := img.readManifest(filepath.Join(workDir, rt.Manifest)); err != nil {
		return errors.Trace(err)
	}

	if err := os.RemoveAll(workDir); err != nil {
		return errors.Trace(err)
	}

	img.Timestamp = time.Now()
	// TODO: img.Origin? Or does it go into manifest's annotations?

	return errors.Trace(img.Commit())
}
