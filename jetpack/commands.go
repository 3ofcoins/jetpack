package jetpack

import "log"
import "sort"

import "github.com/appc/spec/schema"
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
	img, err := rt.Host().Images.Import(aciAddr)
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
	ii, err := rt.Host().Images.All()
	if err != nil {
		return errors.Trace(err)
	}
	sort.Sort(ii)
	for _, img := range ii {
		log.Println(img.Name, img.PrettyLabels())
	}
	return nil
}

func (rt *Runtime) CmdPoke() error {
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

	// TODO: type Container
	manifest := NewContainerRuntimeManifest()
	manifest.Annotations["ip-address"] = "172.23.0.2"
	manifest.Apps = append(manifest.Apps, schema.RuntimeApp{
		Name:    img.Name,
		ImageID: img.Hash,
	})

	ds, err := img.Clone(h.containersFS.Name + "/" + manifest.UUID.String())
	if err != nil {
		return errors.Trace(err)
	}
	log.Println(manifest)
	log.Println(ds)

	return nil
}
