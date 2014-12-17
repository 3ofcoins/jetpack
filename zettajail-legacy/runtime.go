package jetpack

import "log"
import "os"
import "path"
import "time"

import "github.com/augustoroman/multierror"

import "github.com/3ofcoins/jetpack/cli"

type Runtime struct {
	// Global switches
	ZFSRoot string

	// Commands' switches
	Folder             string
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
	//DONE	rt.Cli.AddCommand(name, synopsis,
	//DONE		func(command string, args []string) error {
	//DONE			rt.Command = command
	//DONE			rt.Args = args
	//DONE			return runner()
	//DONE		})
}

func (rt *Runtime) Shift() string {
	//DONE	if len(rt.Args) == 0 {
	//DONE		return ""
	//DONE	}
	//DONE	rv := rt.Args[0]
	//DONE	rt.Args = rt.Args[1:]
	//DONE	return rv
	return ""
}

func (rt *Runtime) Properties() map[string]string {
	//IRRELEVANT 	return ParseProperties(rt.Args)
	return nil
}

func (rt *Runtime) Host() *Host {
	//DONE	if rt.host == nil {
	//DONE		dsName := rt.ZFSRoot
	//DONE		if rt.Folder != "" {
	//DONE			dsName = path.Join(dsName, rt.Folder)
	//DONE		}
	//DONE		rt.host = NewHost(dsName)
	//DONE		if rt.host == nil {
	//DONE			log.Printf("Root ZFS dataset %v does not exist\n", rt.ZFSRoot)
	//DONE			log.Printf("Run `%v init' to create data set\n", rt.Name)
	//DONE			os.Exit(1)
	//DONE		}
	//DONE	}
	return rt.host
}

func (rt *Runtime) Jails(args []string) ([]*Jail, error) {
	//IRRELEVANT 	if len(args) == 0 {
	//IRRELEVANT 		return rt.Host().Jails(), nil // FIXME: Jails() should return an error
	//IRRELEVANT 	}
	//IRRELEVANT 	jails := make([]*Jail, 0, len(args))
	//IRRELEVANT 	var errs multierror.Accumulator
	//IRRELEVANT 	for _, jailName := range args {
	//IRRELEVANT 		if jail, err := rt.Host().GetJail(jailName); err != nil {
	//IRRELEVANT 			errs.Push(err)
	//IRRELEVANT 		} else {
	//IRRELEVANT 			jails = append(jails, jail)
	//IRRELEVANT 		}
	//IRRELEVANT 	}
	//IRRELEVANT 	return jails, errs.Error()
	return nil, nil
}

func (rt *Runtime) ForEachJail(fn func(*Jail) error) error {
	//IRRELEVANT 	jails, err := rt.Jails(rt.Args)
	//IRRELEVANT 	if err != nil {
	//IRRELEVANT 		return err
	//IRRELEVANT 	}
	//IRRELEVANT 	var errs multierror.Accumulator
	//IRRELEVANT 	for _, jail := range jails {
	//IRRELEVANT 		errs.Push(fn(jail))
	//IRRELEVANT 	}
	//IRRELEVANT 	return errs.Error()
	return nil
}

func NewRuntime(name string) *Runtime {
	rt := &Runtime{Cli: cli.NewCli(name)}

	//DONE	rt.ZFSRoot = os.Getenv("ZETTAJAIL_ROOT")
	//DONE	if rt.ZFSRoot == "" {
	//DONE		rt.ZFSRoot = ElucidateDefaultRootDataset()
	//DONE	}
	//DONE
	//DONE	// Global flags
	//DONE	rt.StringVar(&rt.ZFSRoot, "root", rt.ZFSRoot, "Root ZFS filesystem")
	//DONE
	//DONE	// Commands
	//DONE	rt.AddCommand("clone", "SNAPSHOT JAIL [PROPERTY...] -- create new jail from existing snapshot", rt.CmdClone)
	//DONE	rt.AddCommand("console", "[-u=USER] JAIL [COMMAND...] -- execute COMMAND or login shell in JAIL", rt.CmdConsole)
	//DONE	rt.AddCommand("create", "[-i=DIST] JAIL [PROPERTY...] -- create new jail", rt.CmdCreate)
	//DONE	rt.AddCommand("export", "JAIL NAME PATH.aci", rt.CmdExport)
	//DONE	rt.AddCommand("import", "PATH.aci", rt.CmdImport)
	//DONE	rt.AddCommand("info", "[-p=FOLDER] [JAIL...] -- show global info or jail details", rt.CmdInfo)
	//DONE	rt.AddCommand("init", "[-p=FOLDER] [PROPERTY...] -- initialize or modify host (NFY)", rt.CmdInit)
	//DONE	rt.AddCommand("modify", "[-rc] [JAIL...] -- modify some or all jails", rt.CmdCtlJail)
	//DONE	rt.AddCommand("ps", "JAIL [ps options...] -- show list of jail's processes", rt.CmdPs)
	//DONE	rt.AddCommand("restart", "[JAIL...] -- restart some or all jails", rt.CmdCtlJail)
	//DONE	rt.AddCommand("set", "JAIL PROPERTY... -- set or modify jail properties", rt.CmdSet)
	//DONE	rt.AddCommand("snapshot", "[-s=SNAP] [JAIL...] -- snapshot some or all jails", rt.CmdSnapshot)
	//DONE	rt.AddCommand("start", "[JAIL...] -- start (create) some or all jails", rt.CmdCtlJail)
	//DONE	rt.AddCommand("status", "[JAIL...] -- show jail status", rt.CmdStatus)
	//DONE	rt.AddCommand("stop", "[JAIL...] -- stop (remove) some or all jails", rt.CmdCtlJail)
	//DONE	rt.AddCommand("tree", "-- show family tree of jails", rt.CmdTree)

	//DONE 	rt.Commands["console"].StringVar(&rt.User, "u", "root", "User to run command as")
	//DONE 	rt.Commands["create"].StringVar(&rt.Install, "i", "", "Install base system from DIST (e.g. ftp://ftp2.freebsd.org/pub/FreeBSD/releases/amd64/amd64/10.1-RELEASE/, /path/to/base.txz)")
	//DONE 	rt.Commands["info"].StringVar(&rt.Folder, "p", "", "Limit to subfolder")
	//DONE 	rt.Commands["init"].StringVar(&rt.Folder, "p", "", "Initialize subfolder")
	//DONE 	rt.Commands["modify"].BoolVar(&rt.ModForce, "r", false, "Restart jail if necessary")
	//DONE 	rt.Commands["modify"].BoolVar(&rt.ModStart, "c", false, "Start (create) jail if not started")
	//DONE 	rt.Commands["snapshot"].StringVar(&rt.Snapshot, "s", time.Now().UTC().Format("20060102T150405Z"), "Snapshot name")

	return rt
}

func Run(name string, args []string) error {
	//DONE	rt := NewRuntime(name)
	//DONE	rt.Parse(args)
	//DONE	return rt.Run()
	return nil
}
