package jetpack

import "log"
import "os"
import "path"

import "github.com/3ofcoins/jetpack/cli"
import "github.com/3ofcoins/jetpack/ui"

type Runtime struct {
	// CLI stuff
	*cli.Cli
	Command string
	Args    []string

	// Global switches
	ZFSRoot string

	// Per-command switches
	ImageName     string
	Tarball       string
	Manifest      string
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

func NewRuntime(name string) *Runtime {
	rt := &Runtime{
		Cli: cli.NewCli(name),
		UI:  ui.NewUI(os.Stdout),
	}

	rt.ZFSRoot = os.Getenv("JETPACK_ROOT")

	if rt.ZFSRoot == "" {
		if pool, err := getZpool(); err != nil {
			log.Printf("Can't guess default ZFS filesystem: %v.", err)
			log.Fatalln("please set JETPACK_ROOT environment variable or use -root flag")
		} else {
			rt.ZFSRoot = path.Join(pool.Name, "jetpack")
		}
	}

	// Global flags
	rt.StringVar(&rt.ZFSRoot, "root", rt.ZFSRoot, "Root ZFS filesystem")

	// Commands
	rt.AddCommand("build", "[OPTIONS] PATH COMMAND...", rt.CmdBuild)
	rt.AddCommand("info", "[UUID] -- show global info or image/container details", rt.CmdInfo)
	rt.AddCommand("rm", "UUID ... -- destroy images or containers", rt.CmdRm)
	rt.AddCommand("init", "[MOUNTPOINT] -- initialize or modify host (NFY)", rt.CmdInit)
	rt.AddCommand("import", "URI_OR_PATH -- import an image", rt.CmdImport)
	rt.AddCommand("list", "[images|containers] -- list images and/or containers", rt.CmdList)
	rt.AddCommand("ps", "CONTAINER [ps options...] -- show list of jail's processes", rt.CmdPs)
	rt.AddCommand("run", "[OPTIONS] UUID -- run container or image", rt.CmdRun)

	// Switches
	rt.Commands["build"].StringVar(&rt.ImageName, "from", "", "Build from an existing image")
	rt.Commands["build"].StringVar(&rt.Tarball, "tarball", "", "Unpack a tarball into filesystem")
	rt.Commands["build"].StringVar(&rt.Manifest, "manifest", "manifest.json", "Image manifest file")

	rt.Commands["run"].BoolVar(&rt.Console, "console", false, "Run console, not image's app")
	rt.Commands["run"].BoolVar(&rt.Keep, "keep", false, "Keep container after command finishes")

	return rt
}

func Run(name string, args []string) error {
	rt := NewRuntime(name)
	rt.Parse(args)
	return rt.Run()
}
