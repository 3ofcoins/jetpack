package jetpack

import "sort"
import "strconv"

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

func (rt *Runtime) CmdImages() error {
	q := rt.Shift()
	if len(rt.Args) > 0 {
		return cli.ErrUsage
	}

	switch imgs, err := rt.Host().Images.Find(q); err {
	case nil:
		sort.Sort(imgs)
		rt.UI.Table(imgs.Table())
	case ErrNotFound:
		rt.UI.Say("No images")
	default:
		return errors.Trace(err)
	}

	return nil
}

func (rt *Runtime) CmdList() error {
	if len(rt.Args) > 0 {
		return cli.ErrUsage
	}
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

func (rt *Runtime) CmdDestroy() error {
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
	var parentImage *Image
	var h = rt.Host()
	var buildDir = rt.Shift()

	if rt.ImageName != "" {
		if img, err := h.Images.Find1(rt.ImageName); err != nil {
			return errors.Trace(err)
		} else {
			parentImage = img
		}
	}

	if img, err := h.Build(parentImage, rt.Tarball, buildDir, rt.Args); err != nil {
		return errors.Trace(err)
	} else {
		return errors.Trace(rt.Show(img))
	}
}
