package jetpack

import "sort"
import "strconv"

import "github.com/appc/spec/schema/types"
import "github.com/juju/errors"

import "github.com/3ofcoins/jetpack/cli"
import "github.com/3ofcoins/jetpack/run"

func (rt *Runtime) CmdInit() error {
	if err := rt.Host.Initialize(); err != nil {
		return errors.Trace(err)
	}
	return errors.Trace(rt.Show(rt.Host))
}

func (rt *Runtime) CmdImport() error {
	if narg := len(rt.Args); narg == 0 || narg > 2 {
		return cli.ErrUsage
	}

	imageUri := rt.Args[0]
	manifestUri := ""
	if len(rt.Args) == 2 {
		manifestUri = rt.Args[1]
	}

	if img, err := rt.Host.Images.Import(imageUri, manifestUri); err != nil {
		return errors.Trace(err)
	} else {
		return errors.Trace(rt.Show(img))
	}
}

func (rt *Runtime) CmdImages() error {
	q := rt.Shift()
	if len(rt.Args) > 0 {
		return cli.ErrUsage
	}

	switch imgs, err := rt.Host.Images.Find(q); err {
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
	cc, err := rt.Host.Containers.All()
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
	h := rt.Host

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
			return errors.Trace(err)
		}
	}
	return nil
}

func (rt *Runtime) CmdCreate() error {
	h := rt.Host
	name := rt.Shift()
	if img, err := h.Images.Get(name); err != nil {
		return errors.Trace(err)
	} else {
		if c, err := h.Containers.Clone(img); err != nil {
			return errors.Trace(err)
		} else {
			return errors.Trace(rt.Show(c))
		}
	}
}

func (rt *Runtime) CmdKill() error {
	for _, uuid := range rt.Args {
		if err := rt.byUUID(uuid,
			func(c *Container) error { return c.Kill() },
			func(i *Image) error { return ErrNotFound },
		); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (rt *Runtime) CmdInfo() error {
	switch len(rt.Args) {
	case 0: // host info
		return errors.Trace(rt.Show(rt.Host))
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
	c, err := rt.Host.Containers.Get(rt.Shift())
	if err != nil {
		return err
	}
	jid := c.Jid()
	if jid == 0 {
		return errors.Errorf("Container %s is not running", c.Manifest.UUID)
	}
	return run.Command("ps", append([]string{"-J", strconv.Itoa(jid)}, rt.Args...)...).Run()
}

func (rt *Runtime) CmdBuild() error {
	if len(rt.Args) < 3 {
		return cli.ErrUsage
	}

	h := rt.Host

	if parentImg, err := h.Images.Find1(rt.Shift()); err != nil {
		return errors.Trace(err)
	} else {
		buildDir := rt.Shift()
		if childImg, err := parentImg.Build(buildDir, rt.CopyFiles, rt.Args); err != nil {
			return errors.Trace(err)
		} else {
			return errors.Trace(rt.Show(childImg))
		}
	}
}
