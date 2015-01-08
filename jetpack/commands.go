package jetpack

import "path/filepath"
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

func (rt *Runtime) CmdDestroy() error {
	for _, uuid := range rt.Args {
		if obj, err := rt.Host.Get(uuid); err != nil {
			return errors.Trace(err)
		} else {
			if err := obj.(Destroyable).Destroy(); err != nil {
				return errors.Trace(err)
			}
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
		if c, err := rt.Host.Containers.Get(uuid); err != nil {
			return errors.Trace(err)
		} else {
			if err := c.Kill(); err != nil {
				return errors.Trace(err)
			}
		}
	}
	return nil
}

func (rt *Runtime) CmdInfo() error {
	return errors.Trace(rt.Show(rt.Host))
}

func (rt *Runtime) CmdShow() error {
	switch len(rt.Args) {
	case 0:
		return cli.ErrUsage
	case 1:
		if obj, err := rt.Host.Get(rt.Args[0]); err != nil {
			return errors.Trace(err)
		} else {
			return errors.Trace(rt.Show(obj))
		}
	default:
		erred := false
		for _, arg := range rt.Args {
			if obj, err := rt.Host.Get(arg); err != nil {
				rt.UI.Sayf("%v: ERROR: %v", arg, err)
				erred = true
			} else {
				if err := rt.Show(obj); err != nil {
					// error when showing is fatal
					return errors.Trace(err)
				}
			}
		}
		if erred {
			return errors.New("There were errors")
		}
		return nil
	}
}

func (rt *Runtime) CmdRun() error {
	if len(rt.Args) != 1 {
		return cli.ErrUsage
	}
	var app *types.App
	if rt.Console {
		app = ConsoleApp("root")
	}
	if obj, err := rt.Host.Get(rt.Args[0]); err != nil {
		return errors.Trace(err)
	} else {
		switch obj.(type) {
		case *Container:
			return errors.Trace(obj.(*Container).Run(app))
		case *Image:
			return errors.Trace(obj.(*Image).Run(app, rt.Keep))
		default:
			return errors.New("CAN'T HAPPEN")
		}
	}
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

func (rt *Runtime) CmdTest() error {
	return errors.Trace(
		run.Command(filepath.Join(LibexecPath, "test.integration"),
			append(rt.Args, "dataset="+rt.Host.Dataset.Name)...).Run())
}
