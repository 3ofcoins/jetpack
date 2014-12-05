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
	jails := make([]Jail, 0, len(args))
	var errs multierror.Accumulator
	for _, jailName := range args {
		if jail, err := GetJail(jailName); err != nil {
			errs.Push(err)
		} else {
			jails = append(jails, jail)
		}
	}
	return jails, errs.Error()
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

var fl struct {
	User               string
	Snapshot           string
	Install            string
	ModForce, ModStart bool
}

func cmdCtlJail(command string, args []string) error {
	var op string
	switch command {
	case "start":
		op = "-c"
	case "stop":
		op = "-r"
	case "restart":
		op = "-rc"
	case "modify":
		switch {
		case fl.ModForce && fl.ModStart:
			op = "-cmr"
		case fl.ModForce:
			op = "-rm"
		case fl.ModStart:
			op = "-cm"
		default:
			op = "-m"
		}
	}
	return ForEachJail(args, func(jail Jail) error {
		// FIXME: feedback
		return jail.RunJail(op)
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
		if jail.Origin != "" {
			origin := jail.Origin
			if strings.HasPrefix(origin, Host.Name+"/") {
				origin = origin[len(Host.Name)+1:]
			}
			log.Println("Origin:", origin)
		}
		log.Println("Snapshots:", jail.Snapshots())
		return jail.WriteConfigTo(os.Stdout)
	})
}

func printTree(allJails []Jail, snap Dataset, indent string) {
	origin := ""
	if snap != ZeroDataset {
		origin = snap.Name
	}

	jails := []Jail{}
	for _, jail := range allJails {
		if jail.Origin == origin {
			jails = append(jails, jail)
		}
	}

	for i, jail := range jails {
		halfdent := "┃"
		item := "┠"
		if i == len(jails)-1 {
			halfdent = " "
			item = "┖"
		}
		fmt.Printf("%s%s%s\n", indent, item, jail)

		snaps := jail.Snapshots()
		for i, snap := range snaps {
			halfdent2 := "│"
			item := "├"
			if i == len(snaps)-1 {
				halfdent2 = " "
				item = "└"
			}
			fmt.Printf("%s%s%s%s\n", indent, halfdent, item, snap)
			printTree(allJails, snap, indent+halfdent+halfdent2)
		}
	}
}

func cmdTree(_ string, _ []string) error {
	printTree(Host.Jails(), ZeroDataset, "")
	return nil
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
	jail, err := GetJail(args[0])
	if err != nil {
		return err
	}
	if !jail.IsActive() {
		return fmt.Errorf("%s is not started", args[0])
	}
	args = args[1:]

	if len(args) == 0 {
		args = []string{"login", "-f", fl.User}
		fl.User = ""
	}
	if fl.User == "root" {
		fl.User = ""
	}
	return jail.RunJexec(fl.User, args)
}

func cmdSet(_ string, args []string) error {
	// FIXME: modify if running, -f for force-modify
	if len(args) < 2 {
		return cli.ErrUsage
	}
	jail, err := GetJail(args[0])
	if err != nil {
		return err
	}
	return jail.SetProperties(ParseProperties(args[1:]))
}

func cmdInit(_ string, args []string) error {
	return Host.Init(ParseProperties(args))
}

func cmdSnapshot(_ string, args []string) error {
	return ForEachJail(args, func(jail Jail) error {
		// FIXME: feedback
		_, err := jail.Snapshot(fl.Snapshot, false)
		return err
	})
}

func cmdCreate(_ string, args []string) error {
	jail, err := CreateJail(args[0], ParseProperties(args[1:]))
	if err != nil {
		return err
	}
	if fl.Install == "" {
		return nil
	}

	// Maybe just use fetch(1)'s copy/link behaviour here?
	switch fi, err := os.Stat(fl.Install); {
	case err == nil && fi.IsDir():
		fl.Install = filepath.Join(fl.Install, "base.txz")
		if _, err = os.Stat(fl.Install); err != nil {
			return err
		}
	case err == nil:
		// Pass. It is a file, so we assume it's base.txz
	case os.IsNotExist(err):
		if url, err := url.Parse(fl.Install); err != nil {
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
			fl.Install = distfile
		}
		// Check if it's an URL, fetch if yes, bomb if not
	default:
		// Weird error we can't handle
		return err
	}

	log.Println("Unpacking", fl.Install)
	if err := RunCommand("tar", "-C", jail.Mountpoint, "-xpf", fl.Install); err != nil {
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

func cmdClone(_ string, args []string) error {
	_, err := CloneJail(args[0], args[1], ParseProperties(args[2:]))
	return err
}

var Cli = cli.NewCli("")

func init() {
	// Global flags
	Cli.StringVar(&ZFSRoot, "root", ZFSRoot, "Root ZFS filesystem")

	// Commands
	Cli.AddCommand("clone", "SNAPSHOT JAIL [PROPERTY...] -- create new jail from existing snapshot", cmdClone)
	Cli.AddCommand("console", "[-u=USER] JAIL [COMMAND...] -- execute COMMAND or login shell in JAIL", cmdConsole)
	Cli.AddCommand("create", "[-install=DIST] JAIL [PROPERTY...] -- create new jail", cmdCreate)
	Cli.AddCommand("info", "[JAIL...] -- show global info or jail details", cmdInfo)
	Cli.AddCommand("init", "[PROPERTY...] -- initialize or modify host (NFY)", cmdInit)
	Cli.AddCommand("modify", "[-rc] [JAIL...] -- modify some or all jails", cmdCtlJail)
	Cli.AddCommand("restart", "[JAIL...] -- restart some or all jails", cmdCtlJail)
	Cli.AddCommand("set", "JAIL PROPERTY... -- set or modify jail properties", cmdSet)
	Cli.AddCommand("snapshot", "[-s=SNAP] [JAIL...] -- snapshot some or all jails", cmdSnapshot)
	Cli.AddCommand("start", "[JAIL...] -- start (create) some or all jails", cmdCtlJail)
	Cli.AddCommand("status", "[JAIL...] -- show jail status", cmdStatus)
	Cli.AddCommand("stop", "[JAIL...] -- stop (remove) some or all jails", cmdCtlJail)
	Cli.AddCommand("tree", "-- show family tree of jails", cmdTree)

	Cli.Commands["console"].StringVar(&fl.User, "u", "root", "User to run command as")
	Cli.Commands["create"].StringVar(&fl.Install, "install", "", "Install base system from DIST (e.g. ftp://ftp2.freebsd.org/pub/FreeBSD/releases/amd64/amd64/10.1-RELEASE/, /path/to/base.txz)")
	Cli.Commands["modify"].BoolVar(&fl.ModForce, "r", false, "Restart jail if necessary")
	Cli.Commands["modify"].BoolVar(&fl.ModStart, "c", false, "Start (create) jail if not started")
	Cli.Commands["snapshot"].StringVar(&fl.Snapshot, "s", time.Now().UTC().Format("20060102T150405Z"), "Snapshot name")
}
