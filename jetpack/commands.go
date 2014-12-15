package jetpack

import "log"

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

	if aciAddr[0] == '.' || aciAddr[0] == '/' {
		log.Println("Importing ACI from", aciAddr)
		aci, err := ReadACI(aciAddr)
		if err != nil {
			return errors.Trace(err)
		}
		return errors.Trace(rt.Host().ImportACI(aci))
	} else {
		return errors.New("DYOD") // Do Your Own Discovery
	}

	return nil
}
