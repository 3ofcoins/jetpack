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
				}
			}
		} else {
			if jail := cli.GetJail(); !jail.Exists() {
				log.Println("Could not find", cli.Jail)
			} else {
				log.Printf("Configuration for %v:\n", jail)
				jail.WriteConfig(os.Stdout)
			}
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
	case cli.DoSet:
		cli.GetJail().SetJailParameters(cli.GetProperties())
	// TODO: cli.DoInit create zfs root w/ default properties
	default:
		log.Fatalln("Not There Yet")
	}
}
