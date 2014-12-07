package zettajail

import "fmt"
import "io/ioutil"
import "log"
import "net"
import "net/url"
import "os"
import "path"
import "path/filepath"
import "strconv"
import "strings"

import "github.com/3ofcoins/zettajail/cli"

const jailRcConf = `sendmail_submit_enable="NO"
sendmail_outbound_enable="NO"
sendmail_msp_queue_enable="NO"
cron_enable="NO"
devd_enable="NO"
syslogd_enable="NO"
`

func (rt *Runtime) CmdCtlJail() error {
	var op string
	switch rt.Command {
	case "start":
		op = "-c"
	case "stop":
		op = "-r"
	case "restart":
		op = "-rc"
	case "modify":
		switch {
		case rt.ModForce && rt.ModStart:
			op = "-cmr"
		case rt.ModForce:
			op = "-rm"
		case rt.ModStart:
			op = "-cm"
		default:
			op = "-m"
		}
	}
	return rt.ForEachJail(func(jail *Jail) error {
		// FIXME: feedback
		return jail.RunJail(op)
	})
}

func (rt *Runtime) CmdInfo() error {
	if len(rt.Args) == 0 {
		log.Println("Root ZFS dataset:", rt.Host().Name)
		if !rt.Host().Exists() {
			log.Println("Root ZFS dataset does not exist. Please run `zjail init`.")
			return nil
		}
		log.Println("File system root:", rt.Host().Mountpoint)
		iface, err := net.InterfaceByName(rt.Host().Properties["zettajail:jail:interface"])
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
	return rt.ForEachJail(func(jail *Jail) error {
		if err := jail.Status(); err != nil {
			return err
		}
		if jail.Origin != "" {
			origin := jail.Origin
			if strings.HasPrefix(origin, rt.Host().Name+"/") {
				origin = origin[len(rt.Host().Name)+1:]
			}
			log.Println("Origin:", origin)
		}
		log.Println("Snapshots:", jail.Snapshots())
		return jail.WriteConfigTo(os.Stdout)
	})
}

func printTree(allJails []*Jail, snap Dataset, indent string) {
	origin := ""
	if snap != ZeroDataset {
		origin = snap.Name
	}

	jails := []*Jail{}
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
			fmt.Printf("%s%s%s%s\n", indent, halfdent, item, snap.Name[strings.Index(snap.Name, "@"):])
			printTree(allJails, snap, indent+halfdent+halfdent2)
		}
	}
}

func (rt *Runtime) CmdTree() error {
	printTree(rt.Host().Jails(), ZeroDataset, "")
	return nil
}

func (rt *Runtime) CmdStatus() error {
	return rt.ForEachJail(func(jail *Jail) error {
		return jail.Status()
	})
}

func (rt *Runtime) CmdPs() error {
	jail, err := rt.Host().GetJail(rt.Shift())
	if err != nil {
		return err
	}
	jid := jail.Jid()
	if jid == 0 {
		return fmt.Errorf("%s is not running", jail)
	}
	psArgs := []string{"-J", strconv.Itoa(jid)}
	psArgs = append(psArgs, rt.Args...)
	return RunCommand("ps", psArgs...)
}

func (rt *Runtime) CmdConsole() error {
	if len(rt.Args) == 0 {
		return cli.ErrUsage
	}
	jail, err := rt.Host().GetJail(rt.Shift())
	if err != nil {
		return err
	}
	if !jail.IsActive() {
		return fmt.Errorf("%s is not started", jail)
	}

	args := rt.Args
	user := rt.User
	if len(args) == 0 {
		args = []string{"login", "-f", user}
		user = ""
	}
	if user == "root" {
		user = ""
	}
	log.Println(user, args)
	return jail.RunJexec(user, args)
}

func (rt *Runtime) CmdSet() error {
	// FIXME: modify if running, -f for force-modify
	if len(rt.Args) < 2 {
		return cli.ErrUsage
	}
	jail, err := rt.Host().GetJail(rt.Shift())
	if err != nil {
		return err
	}
	return jail.SetProperties(rt.Properties())
}

func (rt *Runtime) cmdInitDwim() (*Host, error) {
	if rt.Folder == "" {
		return CreateHost(rt.ZFSRoot, rt.Properties())
	} else {
		folder := rt.Folder
		rt.Folder = ""
		return rt.Host().CreateFolder(folder, rt.Properties())
	}
}

func (rt *Runtime) CmdInit() error {
	host, err := rt.cmdInitDwim()
	if err != nil {
		return err
	}
	rt.host = host
	return rt.CmdInfo()
}

func (rt *Runtime) CmdSnapshot() error {
	return rt.ForEachJail(func(jail *Jail) error {
		// FIXME: feedback
		_, err := jail.Snapshot(rt.Snapshot, false)
		return err
	})
}

func (rt *Runtime) CmdCreate() error {
	jailName := rt.Shift()
	jail, err := rt.Host().CreateJail(jailName, rt.Properties())
	if err != nil {
		return err
	}
	if rt.Install == "" {
		return nil
	}

	// Maybe just use fetch(1)'s copy/link behaviour here?
	switch fi, err := os.Stat(rt.Install); {
	case err == nil && fi.IsDir():
		rt.Install = filepath.Join(rt.Install, "base.txz")
		if _, err = os.Stat(rt.Install); err != nil {
			return err
		}
	case err == nil:
		// Pass. It is a file, so we assume it's base.txz
	case os.IsNotExist(err):
		if url, err := url.Parse(rt.Install); err != nil {
			return err
		} else {
			// FIXME: fetch MANIFEST, check checksum
			if path.Ext(url.Path) != "txz" {
				// Directory URL
				url.Path = path.Join(url.Path, "base.txz")
			}
			distdir := jail.Path("dist")
			if err := os.MkdirAll(distdir, 0755); err != nil {
				return err
			}
			distfile := filepath.Join(distdir, path.Base(url.Path))

			log.Println("Downloading", url)
			if err := RunCommand("fetch", "-o", distfile, "-m", "-l", url.String()); err != nil {
				return err
			}
			rt.Install = distfile
		}
		// Check if it's an URL, fetch if yes, bomb if not
	default:
		// Weird error we can't handle
		return err
	}

	log.Println("Unpacking", rt.Install)
	if err := RunCommand("tar", "-C", jail.Mountpoint, "-xpf", rt.Install); err != nil {
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

func (rt *Runtime) CmdClone() error {
	snapName := rt.Shift()
	jailName := rt.Shift()
	_, err := rt.Host().CloneJail(snapName, jailName, rt.Properties())
	return err
}
