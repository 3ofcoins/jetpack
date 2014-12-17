package jetpack

import "sort"

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

	c, err := h.Containers.Clone(img)
	rt.UI.Show(c)

	return nil
}
