package main

import "log"
import "net"
import "os"
import "os/exec"

import "github.com/3ofcoins/go-zfs"

func bsdinstall(root string, args ...string) {
	cmd := exec.Command("bsdinstall", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"DISTRIBUTIONS=base.txz",
		"BSDINSTALL_CHROOT="+root,
		"BSDINSTALL_DISTSITE=ftp://ftp.freebsd.org/pub/FreeBSD/releases/amd64/amd64/10.1-RELEASE",
		"BSDINSTALL_LOG="+root+".log",
	)
	if err := cmd.Run(); err != nil {
		log.Fatalln(err)
	}
}

func main() {
	cli, err := ParseArgs()
	if err != nil {
		log.Fatalln(err)
	}

	switch {
	case cli.DoInfo:
		if cli.Jail == "" {
			log.Println("Version:", Version)
			log.Println("Root ZFS dataset:", ZFSRoot)
			if !Root.Exists() {
				log.Printf("Root ZFS dataset does not exist. Please run following or similar:\n  zfs create -o mountpoint=/srv/jail -o atime=off -o compress=lz4 -o dedup=on -p %v\n", ZFSRoot)
			} else {
				log.Println("File system root:", Root.Mountpoint)
				if children, err := Root.Children(); err != nil {
					log.Fatalf("ERROR: %#v", err.Error())
				} else {
					log.Println("Jails:", children)
					log.Println("Parameters:", Root.JailParameters())
					if iface, err := net.InterfaceByName(Root.Properties["jail:interface"]); err != nil {
						log.Fatalln(err)
					} else {
						addrs, _ := iface.Addrs()
						log.Println("Interface:", iface, addrs, addrs[0].Network())
						log.Printf("%#v %#v\n", iface, addrs[0])
					}
					if err := Root.WriteConfigTo(os.Stdout); err != nil {
						log.Fatalln("ERROR:", err)
					}
				}
			}
		} else {
			jail := cli.GetJail()
			log.Printf("Configuration for %v:\n", jail)
			jail.WriteConfigTo(os.Stdout)
		}
	case cli.DoInstall:
		if fs, err := zfs.CreateFilesystem(Root.Name+"/"+cli.Jail, nil); err != nil {
			log.Fatalf("ERROR: %#v", err.Error())
		} else {
			bsdinstall(fs.Mountpoint, "distfetch")
			bsdinstall(fs.Mountpoint, "checksum")
			bsdinstall(fs.Mountpoint, "distextract")
			bsdinstall(fs.Mountpoint, "config")
			bsdinstall(fs.Mountpoint, "entropy")
		}
		if err := Root.WriteJailConf(); err != nil {
			log.Fatalln("ERROR:", err)
		}
	case cli.DoSet:
		cli.GetJail().SetProperties(cli.ParseProperties())
		if err := Root.WriteJailConf(); err != nil {
			log.Fatalln("ERROR:", err)
		}
	case cli.DoStatus:
		if cli.Jail == "" {
			if children, err := Root.Children(); err != nil {
				log.Fatalf("ERROR: %#v", err.Error())
			} else {
				for _, child := range children {
					child.Status()
				}
			}
		} else {
			cli.GetJail().Status()
		}
	case cli.DoStart:
		if err := cli.GetJail().RunJail("-c"); err != nil {
			log.Fatalln("ERROR:", err)
		}
	case cli.DoStop:
		if err := cli.GetJail().RunJail("-r"); err != nil {
			log.Fatalln("ERROR:", err)
		}
	case cli.DoRestart:
		if err := cli.GetJail().RunJail("-rc"); err != nil {
			log.Fatalln("ERROR:", err)
		}
	case cli.DoConsole:
		if err := cli.GetJail().RunJexec("", cli.Command); err != nil {
			log.Fatalln("ERROR:", err)
		}
	case cli.DoInit:
		if err := Root.Init(); err != nil {
			log.Fatalln("ERROR:", err)
		} else {
			Root.SetProperties(cli.ParseProperties())
			if err := Root.WriteJailConf(); err != nil {
				log.Fatalln("ERROR:", err)
			}
		}
	default:
		log.Fatalln("Not There Yet")
	}
}
