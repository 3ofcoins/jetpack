package main

import "log"
import "net"
import "os"
import "os/exec"

import "github.com/3ofcoins/go-zfs"

func (cli *Cli) CmdGlobalInfo() error {
	log.Println("Version:", Version)
	log.Println("Root ZFS dataset:", ZFSRoot)
	if !Host.Exists() {
		log.Println("Root ZFS dataset does not exist. Please run `zjail init`.")
	} else {
		log.Println("File system root:", Host.Mountpoint)
		log.Println("Jails:", Host.Jails())
		log.Println("Parameters:", Host.JailParameters())
		if iface, err := net.InterfaceByName(Host.Properties["jail:interface"]); err != nil {
			return err
		} else {
			addrs, _ := iface.Addrs()
			log.Println("Interface:", iface, addrs, addrs[0].Network())
			log.Printf("%#v %#v\n", iface, addrs[0])
		}
		if err := Host.WriteConfigTo(os.Stdout); err != nil {
			return err
		}
	}
	return nil
}

func (cli *Cli) CmdJailInfo(jail Jail) error {
	jail.Status()
	jail.WriteConfigTo(os.Stdout)
	return nil
}

// FIXME: implement own fetch+install
func (cli *Cli) CmdInstall() error {
	if fs, err := zfs.CreateFilesystem(Host.Name+"/"+cli.Jail, nil); err != nil {
		return err
	} else {
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
				"BSDINSTALL_CHROOT="+fs.Mountpoint,
				"BSDINSTALL_DISTSITE=ftp://ftp.freebsd.org/pub/FreeBSD/releases/amd64/amd64/10.1-RELEASE",
				"BSDINSTALL_LOG="+fs.Mountpoint+".log",
			)
			if err := cmd.Run(); err != nil {
				return err
			}
		}
	}
	return Host.WriteJailConf()
}

func (cli *Cli) CmdSnapshot(jail Jail) error {
	_, err := jail.Snapshot(cli.Snapshot, false)
	return err
}
