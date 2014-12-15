package jetpack

import "log"
import "os"
import "path"

import "github.com/3ofcoins/go-zfs"

import "github.com/3ofcoins/jetpack/cli"

type Runtime struct {
	// CLI stuff
	*cli.Cli
	Command string
	Args    []string

	// Global switches
	ZFSRoot string

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
		pools, err := zfs.ListZpools()
		if err != nil {
			log.Fatalln(err)
		}
		if len(pools) == 0 {
			log.Fatalln("No ZFS pools found")
		}
		if len(pools) > 1 {
			log.Fatalln("Multiple ZFS pools found, please set JETPACK_ROOT environment variable or use -root flag")
		}
		rt.ZFSRoot = path.Join(pools[0].Name, "jetpack")
	}

	// Global flags
	rt.StringVar(&rt.ZFSRoot, "root", rt.ZFSRoot, "Root ZFS filesystem")

	// Commands
	rt.AddCommand("info", "-- show global info or jail details", rt.CmdInfo)
	rt.AddCommand("init", "[MOUNTPOINT] -- initialize or modify host (NFY)", rt.CmdInit)
	rt.AddCommand("import", "URI_OR_PATH -- import an image", rt.CmdImport)
	rt.AddCommand("images", "-- list images", rt.CmdImages)

	return rt
}

func Run(name string, args []string) error {
	rt := NewRuntime(name)
	rt.Parse(args)
	return rt.Run()
}
