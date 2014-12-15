package jetpack

import "log"
import "sort"

import "github.com/juju/errors"

import "github.com/3ofcoins/jetpack/cli"

func (rt *Runtime) CmdInfo() error {
	log.Println("Host:", rt.Host())
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

	log.Println("Importing image from", aciAddr)
	img, err := ImportImage(rt.Host(), aciAddr)
	if err != nil {
		return errors.Trace(err)
	} else {
		log.Println("Imported:", img)
	}

	return nil
}

func (rt *Runtime) CmdImages() error {
	if len(rt.Args) != 0 {
		return cli.ErrUsage
	}
	ii, err := rt.Host().Images()
	if err != nil {
		return errors.Trace(err)
	}
	sort.Sort(ii)
	for _, img := range ii {
		log.Println(img.Name, img.PrettyLabels())
	}
	return nil
}
