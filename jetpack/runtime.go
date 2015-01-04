package jetpack

import "os"

import "code.google.com/p/go-uuid/uuid"
import "github.com/juju/errors"

import "github.com/3ofcoins/jetpack/cli"
import "github.com/3ofcoins/jetpack/ui"

type Runtime struct {
	// CLI stuff
	*cli.Cli
	Command string
	Args    []string

	// Global switches
	configPath string

	// Per-command switches
	Verbose       bool
	User          string
	Console, Keep bool

	// Global runtime state
	Host *Host
	UI   *ui.UI
}

type CommandFlag uint32

const (
	_                              CommandFlag = iota
	CommandAcceptUninitializedHost             = 1 << iota
)

func (rt *Runtime) AddCommand(name, synopsis string, runner func() error, flags ...CommandFlag) {
	fl := CommandFlag(0)
	for _, flag := range flags {
		fl |= flag
	}
	rt.Cli.AddCommand(name, synopsis,
		func(command string, args []string) error {
			if fl&CommandAcceptUninitializedHost == 0 && rt.Host.Dataset == nil {
				return errors.New("Host is not initialized")
			}
			rt.Command = command
			rt.Args = args
			return runner()
		})
}

func (rt *Runtime) Parse(args []string) error {
	rt.Cli.Parse(args)
	if host, err := NewHost(rt.configPath); err != nil {
		return errors.Trace(err)
	} else {
		rt.Host = host
	}
	return nil
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

func (rt *Runtime) Show(obj ...interface{}) error {
	return Show(rt.UI, obj...)
}

func NewRuntime(name string) (*Runtime, error) {
	rt := &Runtime{
		Cli: cli.NewCli(name),
		UI:  ui.NewUI(os.Stdout),
	}

	// Global flags
	rt.StringVar(&rt.configPath, "config", ConfigPath, "Path to the configuration file")

	// Commands
	rt.AddCommand("build", "IMAGE BUILD-DIR COMMAND...", rt.CmdBuild)
	rt.AddCommand("destroy", "UUID ... -- destroy images or containers", rt.CmdDestroy)
	rt.AddCommand("images", "[QUERY] -- list images", rt.CmdImages)
	rt.AddCommand("import", "URI_OR_PATH [MANIFEST] -- import an image from ACI or rootfs tarball", rt.CmdImport)
	rt.AddCommand("info", "[UUID] -- show global info or image/container details", rt.CmdInfo)
	rt.AddCommand("init", "-- initialize host", rt.CmdInit, CommandAcceptUninitializedHost)
	rt.AddCommand("list", "-- list containers", rt.CmdList)
	rt.AddCommand("ps", "CONTAINER [ps options...] -- show list of jail's processes", rt.CmdPs)
	rt.AddCommand("run", "[OPTIONS] IMAGE|CONTAINER  [--] [PARAMETERS...] -- run container or image", rt.CmdRun)
	rt.AddCommand("kill", "UUID... -- kill running containers", rt.CmdKill)
	rt.AddCommand("create", "IMAGE [--] [PARAMETERS...] -- create container from image", rt.CmdCreate)

	// Switches

	rt.Commands["run"].BoolVar(&rt.Console, "console", false, "Run console, not image's app")
	rt.Commands["run"].BoolVar(&rt.Keep, "keep", false, "Keep container after command finishes")

	return rt, nil
}

func Run(name string, args []string) error {
	if rt, err := NewRuntime(name); err != nil {
		return err
	} else {
		if err := rt.Parse(args); err != nil {
			return err
		}
		return rt.Run()
	}
}
