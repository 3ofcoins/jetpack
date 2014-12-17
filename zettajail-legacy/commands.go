package jetpack

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

import "github.com/3ofcoins/jetpack/cli"

const jailRcConf = `sendmail_submit_enable="NO"
sendmail_outbound_enable="NO"
sendmail_msp_queue_enable="NO"
cron_enable="NO"
devd_enable="NO"
syslogd_enable="NO"
`

func (rt *Runtime) CmdCtlJail() error {
	//DONE 	var op string
	//DONE 	switch rt.Command {
	//DONE 	case "start":
	//DONE 		op = "-c"
	//DONE 	case "stop":
	//DONE 		op = "-r"
	//DONE 	case "restart":
	//DONE 		op = "-rc"
	//DONE 	case "modify":
	//DONE 		switch {
	//DONE 		case rt.ModForce && rt.ModStart:
	//DONE 			op = "-cmr"
	//DONE 		case rt.ModForce:
	//DONE 			op = "-rm"
	//DONE 		case rt.ModStart:
	//DONE 			op = "-cm"
	//DONE 		default:
	//DONE 			op = "-m"
	//DONE 		}
	//DONE 	}
	//DONE 	return rt.ForEachJail(func(jail *Jail) error {
	//DONE 		// FIXME: feedback
	//DONE 		return jail.RunJail(op)
	//DONE 	})
	return nil
}

func (rt *Runtime) CmdInfo() error {
	if len(rt.Args) == 0 {
		//DONE		log.Println("Root ZFS dataset:", rt.Host().Name)
		//DONE		if !rt.Host().Exists() {
		//DONE			log.Println("Root ZFS dataset does not exist. Please run `zjail init`.")
		//DONE			return nil
		//DONE		}
		//DONE		log.Println("File system root:", rt.Host().Mountpoint)
		//DONE		iface, err := net.InterfaceByName(rt.Host().Properties["jetpack:jail:interface"])
		//DONE		if err != nil {
		//DONE			return err
		//DONE		}
		//DONE		addrs, err := iface.Addrs()
		//DONE		if err != nil {
		//DONE			return err
		//DONE		}
		//DONE		log.Printf("Interface: %v (%v)\n", iface.Name, addrs[0])
		//DONE		return nil
	}
	return rt.ForEachJail(func(jail *Jail) error {
		//IRRELEVANT 		if err := jail.Status(); err != nil {
		//IRRELEVANT 			return err
		//IRRELEVANT 		}
		//IRRELEVANT 		if jail.Origin != "" {
		//IRRELEVANT 			origin := jail.Origin
		//IRRELEVANT 			if strings.HasPrefix(origin, rt.Host().Name+"/") {
		//IRRELEVANT 				origin = origin[len(rt.Host().Name)+1:]
		//IRRELEVANT 			}
		//IRRELEVANT 			log.Println("Origin:", origin)
		//IRRELEVANT 		}
		//IRRELEVANT 		log.Println("Snapshots:", jail.Snapshots())
		//IRRELEVANT 		return jail.WriteConfigTo(os.Stdout)
		//IRRELEVANT
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
	//DONE 	return rt.ForEachJail(func(jail *Jail) error {
	//DONE 		return jail.Status()
	//DONE 	})
	return nil
}

func (rt *Runtime) CmdPs() error {
	//DONE 	jail, err := rt.Host().GetJail(rt.Shift())
	//DONE 	if err != nil {
	//DONE 		return err
	//DONE 	}
	//DONE 	jid := jail.Jid()
	//DONE 	if jid == 0 {
	//DONE 		return fmt.Errorf("%s is not running", jail)
	//DONE 	}
	//DONE 	psArgs := []string{"-J", strconv.Itoa(jid)}
	//DONE 	psArgs = append(psArgs, rt.Args...)
	//DONE 	return RunCommand("ps", psArgs...)
	return nil
}

func (rt *Runtime) CmdConsole() error {
	//DONE 	if len(rt.Args) == 0 {
	//DONE 		return cli.ErrUsage
	//DONE 	}
	//DONE 	jail, err := rt.Host().GetJail(rt.Shift())
	//DONE 	if err != nil {
	//DONE 		return err
	//DONE 	}
	//DONE 	if !jail.IsActive() {
	//DONE 		return fmt.Errorf("%s is not started", jail)
	//DONE 	}
	//DONE
	//DONE 	args := rt.Args
	//DONE 	user := rt.User
	//DONE 	if len(args) == 0 {
	//DONE 		args = []string{"login", "-f", user}
	//DONE 		user = ""
	//DONE 	}
	//DONE 	if user == "root" {
	//DONE 		user = ""
	//DONE 	}
	//DONE 	log.Println(user, args)
	//DONE 	return jail.RunJexec(user, args)
	return nil
}

func (rt *Runtime) CmdSet() error {
	//IRRELEVANT 	// FIXME: modify if running, -f for force-modify
	//IRRELEVANT 	if len(rt.Args) < 2 {
	//IRRELEVANT 		return cli.ErrUsage
	//IRRELEVANT 	}
	//IRRELEVANT 	jail, err := rt.Host().GetJail(rt.Shift())
	//IRRELEVANT 	if err != nil {
	//IRRELEVANT 		return err
	//IRRELEVANT 	}
	//IRRELEVANT 	return jail.SetProperties(rt.Properties())
	return nil
}

//DONEfunc (rt *Runtime) cmdInitDwim() (*Host, error) {
//DONE	if rt.Folder == "" {
//DONE		return CreateHost(rt.ZFSRoot, rt.Properties())
//DONE	} else {
//DONE		folder := rt.Folder
//DONE		rt.Folder = ""
//DONE		return rt.Host().CreateFolder(folder, rt.Properties())
//DONE	}
//DONE}

func (rt *Runtime) CmdInit() error {
	//DONE	host, err := rt.cmdInitDwim()
	//DONE	if err != nil {
	//DONE		return err
	//DONE	}
	//DONE	rt.host = host
	return rt.CmdInfo()
}

func (rt *Runtime) CmdSnapshot() error {
	//IRRELEVANT 	return rt.ForEachJail(func(jail *Jail) error {
	//IRRELEVANT 		// FIXME: feedback
	//IRRELEVANT 		_, err := jail.Snapshot(rt.Snapshot, false)
	//IRRELEVANT 		return err
	//IRRELEVANT 	})
	return nil
}

func (rt *Runtime) CmdCreate() error {
	//DONE 	jailName := rt.Shift()
	//DONE 	jail, err := rt.Host().CreateJail(jailName, rt.Properties())
	//DONE 	if err != nil {
	//DONE 		return err
	//DONE 	}
	//DONE 	if rt.Install == "" {
	//DONE 		return nil
	//DONE 	}
	//DONE
	//DONE 	// Maybe just use fetch(1)'s copy/link behaviour here?
	//DONE 	switch fi, err := os.Stat(rt.Install); {
	//DONE 	case err == nil && fi.IsDir():
	//DONE 		rt.Install = filepath.Join(rt.Install, "base.txz")
	//DONE 		if _, err = os.Stat(rt.Install); err != nil {
	//DONE 			return err
	//DONE 		}
	//DONE 	case err == nil:
	//DONE 		// Pass. It is a file, so we assume it's base.txz
	//DONE 	case os.IsNotExist(err):
	//DONE 		if url, err := url.Parse(rt.Install); err != nil {
	//DONE 			return err
	//DONE 		} else {
	//DONE 			// FIXME: fetch MANIFEST, check checksum
	//DONE 			if path.Ext(url.Path) != "txz" {
	//DONE 				// Directory URL
	//DONE 				url.Path = path.Join(url.Path, "base.txz")
	//DONE 			}
	//DONE 			distdir := jail.Path("dist")
	//DONE 			if err := os.MkdirAll(distdir, 0755); err != nil {
	//DONE 				return err
	//DONE 			}
	//DONE 			distfile := filepath.Join(distdir, path.Base(url.Path))
	//DONE
	//DONE 			log.Println("Downloading", url)
	//DONE 			if err := RunCommand("fetch", "-o", distfile, "-m", "-l", url.String()); err != nil {
	//DONE 				return err
	//DONE 			}
	//DONE 			rt.Install = distfile
	//DONE 		}
	//DONE 		// Check if it's an URL, fetch if yes, bomb if not
	//DONE 	default:
	//DONE 		// Weird error we can't handle
	//DONE 		return err
	//DONE 	}
	//DONE
	//DONE 	log.Println("Unpacking", rt.Install)
	//DONE 	if err := RunCommand("tar", "-C", jail.Mountpoint, "-xpf", rt.Install); err != nil {
	//DONE 		return err
	//DONE 	}
	//DONE
	//DONE 	log.Println("Configuring", jail.Mountpoint)
	//DONE 	if err := ioutil.WriteFile(filepath.Join(jail.Mountpoint, "/etc/rc.conf"), []byte(jailRcConf), 0644); err != nil {
	//DONE 		return err
	//DONE 	}
	//DONE
	//DONE 	if bb, err := ioutil.ReadFile("/etc/resolv.conf"); err != nil {
	//DONE 		return err
	//DONE 	} else {
	//DONE 		if err := ioutil.WriteFile(filepath.Join(jail.Mountpoint, "/etc/resolv.conf"), bb, 0644); err != nil {
	//DONE 			return err
	//DONE 		}
	//DONE 	}
	//DONE
	//DONE 	rf, err := os.Open("/dev/random")
	//DONE 	if err != nil {
	//DONE 		return err
	//DONE 	}
	//DONE 	defer rf.Close()
	//DONE 	entropy := make([]byte, 4096)
	//DONE 	if _, err := rf.Read(entropy); err != nil {
	//DONE 		return err
	//DONE 	}
	//DONE 	return ioutil.WriteFile(filepath.Join(jail.Mountpoint, "/entropy"), entropy, 0600)
	return nil
}

func (rt *Runtime) CmdClone() error {
	//DONE 	snapName := rt.Shift()
	//DONE 	jailName := rt.Shift()
	//DONE 	_, err := rt.Host().CloneJail(snapName, jailName, rt.Properties())
	//DONE 	return err
	return nil
}
