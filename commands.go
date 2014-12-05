package zettajail

import "fmt"
import "io/ioutil"
import "log"
import "net"
import "net/url"
import "os"
import "path"
import "path/filepath"
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

const jailRcConf = `sendmail_submit_enable="NO"
sendmail_outbound_enable="NO"
sendmail_msp_queue_enable="NO"
cron_enable="NO"
devd_enable="NO"
syslogd_enable="NO"
`

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

	func() *Command {
		var install string
		cmd := NewCommand("create", "[-install=DIST] JAIL [PROPERTY...] -- create new jail",
			func(_ string, args []string) error {
				jail, err := CreateJail(args[0], ParseProperties(args[1:]))
				if err != nil {
					return err
				}
				if install != "" {
					// Maybe just use fetch(1)'s copy/link behaviour here?
					switch fi, err := os.Stat(install); {
					case err == nil && fi.IsDir():
						install = filepath.Join(install, "base.txz")
						if _, err = os.Stat(install); err != nil {
							return err
						}
					case err == nil:
						// Pass. It is a file, so we assume it's base.txz
					case os.IsNotExist(err):
						if url, err := url.Parse(install); err != nil {
							return err
						} else {
							// FIXME: fetch MANIFEST, check checksum
							if path.Ext(url.Path) != "txz" {
								// Directory URL
								url.Path = path.Join(url.Path, "base.txz")
							}
							distdir := jail.Mountpoint + ".dist"
							if err := os.MkdirAll(distdir, 0755); err != nil {
								return err
							}
							distfile := filepath.Join(distdir, path.Base(url.Path))

							log.Println("Downloading", url)
							if err := RunCommand("fetch", "-o", distfile, "-m", "-l", url.String()); err != nil {
								return err
							}
							install = distfile
						}
						// Check if it's an URL, fetch if yes, bomb if not
					default:
						// Weird error we can't handle
						return err
					}

					log.Println("Unpacking", install)
					if err := RunCommand("tar", "-C", jail.Mountpoint, "-xpf", install); err != nil {
						return err
					}

					log.Println("Configuring", jail.Mountpoint)
					if err := ioutil.WriteFile(filepath.Join(jail.Mountpoint, "/etc/rc.conf"), []byte(jailRcConf), 0644); err != nil {
						return err
					}

					if bb, err := ioutil.ReadFile("/etc/resolv.conf"); err != nil {
						return err
					} else {
						if err := ioutil.WriteFile(filepath.Join(jail.Mountpoint, "/etc/resolv.conf"), bb, 0644); err != nil {
							return err
						}
					}

					rf, err := os.Open("/dev/random")
					if err != nil {
						return err
					}
					defer rf.Close()
					entropy := make([]byte, 4096)
					if _, err := rf.Read(entropy); err != nil {
						return err
					}
					return ioutil.WriteFile(filepath.Join(jail.Mountpoint, "/entropy"), entropy, 0600)
				}
				return nil
			})
		cmd.StringVar(&install, "install", "", "Install base system from DIST (e.g. ftp://ftp2.freebsd.org/pub/FreeBSD/releases/amd64/amd64/10.1-RELEASE/, /path/to/base.txz)")
		return cmd
	}(),
}

func RunZettajail() error {
	cli := NewCli("")
	cli.StringVar(&ZFSRoot, "root", ZFSRoot, "Root ZFS filesystem")
	for _, cmd := range Commands {
		cli.AddCommand(cmd)
	}
	cli.Parse(nil)
	return cli.Run()
}
