package jetpack

import "log"
import "os"
import "path"

import "code.google.com/p/go-uuid/uuid"
import "github.com/juju/errors"

import "github.com/3ofcoins/jetpack/cli"
import "github.com/3ofcoins/jetpack/ui"
import "github.com/3ofcoins/jetpack/zfs"

type Runtime struct {
	// CLI stuff
	*cli.Cli
	Command string
	Args    []string

	// Global switches
	ZFSRoot string

	// Per-command switches
	Verbose       bool
	User          string
	Console, Keep bool

	// Global runtime state
	host *Host
	UI   *ui.UI
}

func (rt *Runtime) AddCommand(name, synopsis string, runner func() error) {
	rt.Cli.AddCommand(name, synopsis,
		func(command string, args []string) error {
			rt.Command = command
			rt.Args = args
			return runner()
		})
}

// Command helpers

func (rt *Runtime) Shift() string {
	if len(rt.Args) == 0 {
		return ""
	}
	rv := rt.Args[0]
	rt.Args = rt.Args[1:]
	return rv
}

func (rt *Runtime) ShiftUUID() uuid.UUID {
	if len(rt.Args) == 0 {
		return nil
	}
	if uuid := uuid.Parse(rt.Args[0]); uuid != nil {
		rt.Args = rt.Args[1:]
		return uuid
	}
	return nil
}

func (rt *Runtime) Host() *Host {
	if rt.host == nil {
		if host, err := GetHost(rt.ZFSRoot); err != nil {
			log.Printf("Root ZFS dataset %v does not seem to exist (%#v)\n", rt.ZFSRoot, err.Error())
			log.Printf("Run `%v init' to create data set\n", rt.Name)
			os.Exit(1)
		} else {
			rt.host = host
		}
	}
	return rt.host
}

func (rt *Runtime) Show(obj ...interface{}) error {
	return Show(rt.UI, obj...)
}

func guessRoot() (string, error) {
	if pools, err := zfs.ZPools(); err != nil {
		return "", err
	} else {
		switch len(pools) {
		case 0:
			return "", errors.New("No ZFS pools found")
		case 1:
			return path.Join(pools[0], "jetpack"), nil
		default:
			return "", errors.New("Multiple ZFS pools found, I don't dare guess")
		}
	}
}

func NewRuntime(name string) (*Runtime, error) {
	rt := &Runtime{
		Cli: cli.NewCli(name),
		UI:  ui.NewUI(os.Stdout),
	}

	rt.ZFSRoot = os.Getenv("JETPACK_ROOT")

	if rt.ZFSRoot == "" {

		if root, err := guessRoot(); err != nil {
			return nil, errors.Maskf(err, "Can't guess default ZFS filesystem; please set JETPACK_ROOT environment variable or use -root flag")
		} else {
			rt.ZFSRoot = root
		}
	}

	// Global flags
	rt.StringVar(&rt.ZFSRoot, "root", rt.ZFSRoot, "Root ZFS filesystem")

	// Commands
	rt.AddCommand("build", "IMAGE BUILD-DIR COMMAND...", rt.CmdBuild)
	rt.AddCommand("destroy", "UUID ... -- destroy images or containers", rt.CmdDestroy)
	rt.AddCommand("images", "[QUERY] -- list images", rt.CmdImages)
	rt.AddCommand("import", "URI_OR_PATH [MANIFEST] -- import an image from ACI or rootfs tarball", rt.CmdImport)
	rt.AddCommand("info", "[UUID] -- show global info or image/container details", rt.CmdInfo)
	rt.AddCommand("init", "[MOUNTPOINT] -- initialize or modify host (NFY)", rt.CmdInit)
	rt.AddCommand("list", "-- list containers", rt.CmdList)
	rt.AddCommand("ps", "CONTAINER [ps options...] -- show list of jail's processes", rt.CmdPs)
	rt.AddCommand("run", "[OPTIONS] UUID -- run container or image", rt.CmdRun)
	rt.AddCommand("kill", "UUID... -- kill running containers", rt.CmdKill)

	// Switches

	rt.Commands["run"].BoolVar(&rt.Console, "console", false, "Run console, not image's app")
	rt.Commands["run"].BoolVar(&rt.Keep, "keep", false, "Keep container after command finishes")

	return rt, nil
}

func Run(name string, args []string) error {
	if rt, err := NewRuntime(name); err != nil {
		return err
	} else {
		rt.Parse(args)
		return rt.Run()
	}
}
