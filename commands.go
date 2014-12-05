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

import "github.com/3ofcoins/zettajail/cli"

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

var flag struct {
	User     string
	Snapshot string
	Install  string
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

func cmdCtlJail(cmd string, args []string) error {
	flag := jailCmdSwitch[cmd]
	return ForEachJail(args, func(jail Jail) error {
		// FIXME: feedback
		return jail.RunJail(flag)
	})
}

func cmdInfo(_ string, args []string) error {
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
}

func cmdStatus(_ string, args []string) error {
	return ForEachJail(args, func(jail Jail) error {
		return jail.Status()
	})
}

func cmdConsole(_ string, args []string) error {
	if len(args) == 0 {
		return cli.ErrUsage
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
		args = []string{"login", "-f", flag.User}
		flag.User = ""
	}
	if flag.User == "root" {
		flag.User = ""
	}
	return jail.RunJexec(flag.User, args)
}

func cmdSet(_ string, args []string) error {
	// FIXME: modify if running, -f for force-modify
	if len(args) < 2 {
		return cli.ErrUsage
	}
	jail := GetJail(args[0])
	if !jail.Exists() {
		return fmt.Errorf("%s does not exist", args[0])
	}
	return jail.SetProperties(ParseProperties(args[1:]))
}

func cmdInit(_ string, args []string) error {
	return Host.Init(ParseProperties(args))
}

func cmdSnapshot(_ string, args []string) error {
	return ForEachJail(args, func(jail Jail) error {
		// FIXME: feedback
		_, err := jail.Snapshot(flag.Snapshot, false)
		return err
	})
}

func cmdCreate(_ string, args []string) error {
	jail, err := CreateJail(args[0], ParseProperties(args[1:]))
	if err != nil {
		return err
	}
	if flag.Install == "" {
		return nil
	}

	// Maybe just use fetch(1)'s copy/link behaviour here?
	switch fi, err := os.Stat(flag.Install); {
	case err == nil && fi.IsDir():
		flag.Install = filepath.Join(flag.Install, "base.txz")
		if _, err = os.Stat(flag.Install); err != nil {
			return err
		}
	case err == nil:
		// Pass. It is a file, so we assume it's base.txz
	case os.IsNotExist(err):
		if url, err := url.Parse(flag.Install); err != nil {
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
			flag.Install = distfile
		}
		// Check if it's an URL, fetch if yes, bomb if not
	default:
		// Weird error we can't handle
		return err
	}

	log.Println("Unpacking", flag.Install)
	if err := RunCommand("tar", "-C", jail.Mountpoint, "-xpf", flag.Install); err != nil {
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

var Cli = cli.NewCli("")

func init() {
	// Global vars
	Cli.StringVar(&ZFSRoot, "root", ZFSRoot, "Root ZFS filesystem")

	// Commands
	Cli.AddCommand("info", "[JAIL...] -- show global info or jail details", cmdInfo)
	Cli.AddCommand("status", "[JAIL...] -- show jail status", cmdStatus)

	// FIXME: less proliferation, nicer error handling, better feedback
	Cli.AddCommand("start", "[JAIL...] -- start some or all jails", cmdCtlJail)
	Cli.AddCommand("stop", "[JAIL...] -- stop some or all jails", cmdCtlJail)
	Cli.AddCommand("restart", "[JAIL...] -- restart some or all jails", cmdCtlJail)
	Cli.AddCommand("modify", "[JAIL...] -- modify some or all jails", cmdCtlJail)
	Cli.AddCommand("force-modify", "[JAIL...] -- modify, restarting if necessary, some or all jails", cmdCtlJail)
	Cli.AddCommand("start-modify", "[JAIL...] -- start some or all jails, modify if started", cmdCtlJail)
	Cli.AddCommand("start-force-modify", "[JAIL...] -- start some or all jails, force-modify if started", cmdCtlJail)

	Cli.AddCommand("console", "[-u=USER] JAIL [COMMAND...] -- execute COMMAND or login shell in JAIL", cmdConsole)
	Cli.AddCommand("set", "JAIL PROPERTY... -- set or modify jail properties", cmdSet)
	Cli.AddCommand("init", "[PROPERTY...] -- initialize or modify host (NFY)", cmdInit)
	Cli.AddCommand("snapshot", "[-s=SNAP] [JAIL...] -- snapshot existing jails", cmdSnapshot)
	Cli.AddCommand("create", "[-install=DIST] JAIL [PROPERTY...] -- create new jail", cmdCreate)

	Cli.Commands["console"].StringVar(&flag.User, "u", "root", "User to run command as")
	Cli.Commands["snapshot"].StringVar(&flag.Snapshot, "s", time.Now().UTC().Format("20060102T150405Z"), "Snapshot name")
	Cli.Commands["create"].StringVar(&flag.Install, "install", "", "Install base system from DIST (e.g. ftp://ftp2.freebsd.org/pub/FreeBSD/releases/amd64/amd64/10.1-RELEASE/, /path/to/base.txz)")
}
