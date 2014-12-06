package zettajail

import "os"
import "time"

import "github.com/augustoroman/multierror"

import "github.com/3ofcoins/zettajail/cli"

type Runtime struct {
	// Global switches
	ZFSRoot string

	// Commands' switches
	User               string
	Snapshot           string
	Install            string
	ModForce, ModStart bool

	// Global state
	host *Host

	// CLI stuff
	*cli.Cli
	Command string
	Args    []string
}

func (rt *Runtime) AddCommand(name, synopsis string, runner func() error) {
	rt.Cli.AddCommand(name, synopsis,
		func(command string, args []string) error {
			rt.Command = command
			rt.Args = args
			return runner()
		})
}

func (rt *Runtime) Shift() string {
	if len(rt.Args) == 0 {
		return ""
	}
	rv := rt.Args[0]
	rt.Args = rt.Args[1:]
	return rv
}

func (rt *Runtime) Properties() map[string]string {
	return ParseProperties(rt.Args)
}

func (rt *Runtime) Host() *Host {
	if rt.host == nil {
		rt.host = NewHost(rt.ZFSRoot)
	}
	return rt.host
}

func (rt *Runtime) Jails(args []string) ([]*Jail, error) {
	if len(args) == 0 {
		return rt.Host().Jails(), nil // FIXME: Jails() should return an error
	}
	jails := make([]*Jail, 0, len(args))
	var errs multierror.Accumulator
	for _, jailName := range args {
		if jail, err := rt.Host().GetJail(jailName); err != nil {
			errs.Push(err)
		} else {
			jails = append(jails, jail)
		}
	}
	return jails, errs.Error()
}

func (rt *Runtime) ForEachJail(fn func(*Jail) error) error {
	jails, err := rt.Jails(rt.Args)
	if err != nil {
		return err
	}
	var errs multierror.Accumulator
	for _, jail := range jails {
		errs.Push(fn(jail))
	}
	return errs.Error()
}

func NewRuntime(name string) *Runtime {
	rt := &Runtime{Cli: cli.NewCli(name)}

	// Global flags
	rt.StringVar(&rt.ZFSRoot, "root", os.Getenv("ZETTAJAIL_ROOT"), "Root ZFS filesystem")

	// Commands
	rt.AddCommand("clone", "SNAPSHOT JAIL [PROPERTY...] -- create new jail from existing snapshot", rt.CmdClone)
	rt.AddCommand("console", "[-u=USER] JAIL [COMMAND...] -- execute COMMAND or login shell in JAIL", rt.CmdConsole)
	rt.AddCommand("create", "[-install=DIST] JAIL [PROPERTY...] -- create new jail", rt.CmdCreate)
	rt.AddCommand("info", "[JAIL...] -- show global info or jail details", rt.CmdInfo)
	rt.AddCommand("init", "[PROPERTY...] -- initialize or modify host (NFY)", rt.CmdInit)
	rt.AddCommand("modify", "[-rc] [JAIL...] -- modify some or all jails", rt.CmdCtlJail)
	rt.AddCommand("ps", "JAIL [ps options...] -- show list of jail's processes", rt.CmdPs)
	rt.AddCommand("restart", "[JAIL...] -- restart some or all jails", rt.CmdCtlJail)
	rt.AddCommand("set", "JAIL PROPERTY... -- set or modify jail properties", rt.CmdSet)
	rt.AddCommand("snapshot", "[-s=SNAP] [JAIL...] -- snapshot some or all jails", rt.CmdSnapshot)
	rt.AddCommand("start", "[JAIL...] -- start (create) some or all jails", rt.CmdCtlJail)
	rt.AddCommand("status", "[JAIL...] -- show jail status", rt.CmdStatus)
	rt.AddCommand("stop", "[JAIL...] -- stop (remove) some or all jails", rt.CmdCtlJail)
	rt.AddCommand("tree", "-- show family tree of jails", rt.CmdTree)

	rt.Commands["console"].StringVar(&rt.User, "u", "root", "User to run command as")
	rt.Commands["create"].StringVar(&rt.Install, "install", "", "Install base system from DIST (e.g. ftp://ftp2.freebsd.org/pub/FreeBSD/releases/amd64/amd64/10.1-RELEASE/, /path/to/base.txz)")
	rt.Commands["modify"].BoolVar(&rt.ModForce, "r", false, "Restart jail if necessary")
	rt.Commands["modify"].BoolVar(&rt.ModStart, "c", false, "Start (create) jail if not started")
	rt.Commands["snapshot"].StringVar(&rt.Snapshot, "s", time.Now().UTC().Format("20060102T150405Z"), "Snapshot name")

	return rt
}

func Run(name string, args []string) error {
	rt := NewRuntime(name)
	rt.Parse(args)
	return rt.Run()
}
