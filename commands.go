package zettajail

import "fmt"
import "log"
import "net"
import "os"
import "os/exec"
import "strings"
import "time"

import "github.com/augustoroman/multierror"

func ParseJails(args []string) ([]Jail, error) {
	if len(args) == 0 {
		return Host.Jails(), nil // FIXME: Jails() should return an error
	}
	jails := make([]Jail, len(args))
	notFound := []string{}
	for i, jailName := range args {
		jails[i] = GetJail(jailName)
		if !jails[i].Exists() {
			notFound = append(notFound, jailName)
		}
	}
	if len(notFound) > 0 {
		return nil, fmt.Errorf("Could not find: %s", strings.Join(notFound, ", "))
	}
	return jails, nil
}

func ForEachJail(jailNames []string, fn func(Jail) error) error {
	jails, err := ParseJails(jailNames)
	if err != nil {
		return err
	}
	var errs multierror.Accumulator
	for _, jail := range jails {
		errs.Push(fn(jail))
	}
	return errs.Error()

}

var jailCmdSwitch = map[string]string{
	"start":              "-c",
	"stop":               "-r",
	"restart":            "-rc",
	"modify":             "-m",
	"force-modify":       "-rm",
	"start-modify":       "-cm",
	"start-force-modify": "-cmr",
}

func runJail(cmd string, args []string) error {
	flag := jailCmdSwitch[cmd]
	return ForEachJail(args, func(jail Jail) error {
		// FIXME: feedback
		return jail.RunJail(flag)
	})
}

var Commands = []*Command{
	NewCommand("info", "[JAIL...] -- show global info or jail details",
		func(_ string, args []string) error {
			if len(args) == 0 {
				log.Println("Version:", Version)
				log.Println("Root ZFS dataset:", ZFSRoot)
				if !Host.Exists() {
					log.Println("Root ZFS dataset does not exist. Please run `zjail init`.")
					return nil
				}
				log.Println("File system root:", Host.Mountpoint)
				iface, err := net.InterfaceByName(Host.Properties["zettajail:jail:interface"])
				if err != nil {
					return err
				}
				addrs, err := iface.Addrs()
				if err != nil {
					return err
				}
				log.Printf("Interface: %v (%v)\n", iface.Name, addrs[0])
				return nil
			}
			return ForEachJail(args, func(jail Jail) error {
				if err := jail.Status(); err != nil {
					return err
				}
				log.Println("Snapshots:", jail.Snapshots())
				return jail.WriteConfigTo(os.Stdout)
			})
		}),
	NewCommand("status", "[JAIL...] -- show jail status",
		func(_ string, args []string) error {
			return ForEachJail(args, func(jail Jail) error {
				return jail.Status()
			})
		}),

	// FIXME: lesser proliferation, nicer error handling, better feedback
	NewCommand("start", "[JAIL...] -- start some or all jails", runJail),
	NewCommand("stop", "[JAIL...] -- stop some or all jails", runJail),
	NewCommand("restart", "[JAIL...] -- restart some or all jails", runJail),
	NewCommand("modify", "[JAIL...] -- modify some or all jails", runJail),
	NewCommand("force-modify", "[JAIL...] -- modify, restarting if necessary, some or all jails", runJail),
	NewCommand("start-modify", "[JAIL...] -- start some or all jails, modify if started", runJail),
	NewCommand("start-force-modify", "[JAIL...] -- start some or all jails, force-modify if started", runJail),

	func() *Command {
		var user string

		cmd := NewCommand("console",
			"[flags] JAIL [COMMAND...] -- execute COMMAND in JAIL [default: login shell]",
			func(_ string, args []string) error {
				if len(args) == 0 {
					return ErrUsage
				}
				jail := GetJail(args[0])
				if !jail.Exists() {
					return fmt.Errorf("%s does not exist", args[0])
				}
				if !jail.IsActive() {
					return fmt.Errorf("%s is not started", args[0])
				}
				args = args[1:]

				if len(args) == 0 {
					args = []string{"login", "-f", user}
					user = ""
				}
				if user == "root" {
					user = ""
				}
				log.Println("jexec", user, args)
				return jail.RunJexec(user, args)
			})
		cmd.StringVar(&user, "u", "root", "User to run command as")
		return cmd
	}(),

	// FIXME: modify if running, -f for force-modify
	NewCommand("set", "JAIL PROPERTY... -- set or modify jail properties",
		func(_ string, args []string) error {
			if len(args) < 2 {
				return ErrUsage
			}
			jail := GetJail(args[0])
			if !jail.Exists() {
				return fmt.Errorf("%s does not exist", args[0])
			}
			return jail.SetProperties(ParseProperties(args[1:]))
		}),

	NewCommand("init", "[PROPERTY...] -- initialize or modify host (NFY)",
		func(_ string, args []string) error {
			return Host.Init(ParseProperties(args))
		}),

	func() *Command {
		var snap string
		cmd := NewCommand("snapshot", "[-s=SNAP] [JAIL...] -- snapshot existing jails",
			func(_ string, args []string) error {
				return ForEachJail(args, func(jail Jail) error {
					// FIXME: feedback
					_, err := jail.Snapshot(snap, false)
					return err
				})
			})
		cmd.StringVar(&snap, "s", time.Now().UTC().Format("20060102T150405Z"), "Snapshot name")
		return cmd
	}(),
}

func (cli *OldCli) CmdCreate() error {
	log.Printf("%v\n%#v\n", cli.Properties(), cli)
	jail, err := CreateJail(cli.Jail, cli.Properties())
	if err != nil {
		return err
	}

	// FIXME: implement own fetch+install
	for _, subcmd := range []string{
		"distfetch",
		"checksum",
		"distextract",
		"config",
		"entropy",
	} {
		cmd := exec.Command("bsdinstall", subcmd)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = append(os.Environ(),
			"DISTRIBUTIONS=base.txz",
			"BSDINSTALL_CHROOT="+jail.Mountpoint,
			"BSDINSTALL_DISTSITE=ftp://ftp.freebsd.org/pub/FreeBSD/releases/amd64/amd64/10.1-RELEASE",
		)
		if log, hasLog := jail.Properties["zettajail:jail:exec.consolelog"]; hasLog {
			cmd.Env = append(cmd.Env, "BSDINSTALL_LOG="+log)
		}

		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return Host.WriteJailConf()
}
