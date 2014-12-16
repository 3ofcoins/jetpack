package jetpack

import "log"
import "os"
import "path"

import "github.com/3ofcoins/jetpack/cli"

type Runtime struct {
	// CLI stuff
	*cli.Cli
	Command string
	Args    []string

	// Global switches
	ZFSRoot string

	// Per-command switches
	ImageName string
	Verbose   bool

	// Global runtime state
	host *Host
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
	rt := &Runtime{Cli: cli.NewCli(name)}

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
	rt.AddCommand("info", "[-i IMAGE] -- show global info or image details", rt.CmdInfo)
	rt.AddCommand("init", "[MOUNTPOINT] -- initialize or modify host (NFY)", rt.CmdInit)
	rt.AddCommand("import", "URI_OR_PATH -- import an image", rt.CmdImport)
	rt.AddCommand("images", "[-v] -- list images", rt.CmdImages)

	// Internal commands
	rt.AddCommand(".poke", "IMAGE [COMMAND] -- poke around in an image", rt.CmdPoke)

	// Switches
	rt.Commands["info"].StringVar(&rt.ImageName, "i", "", "Show info about an image")
	rt.Commands["images"].BoolVar(&rt.Verbose, "v", false, "Show detailed info")

	return rt
}

func Run(name string, args []string) error {
	rt := NewRuntime(name)
	rt.Parse(args)
	return rt.Run()
}
