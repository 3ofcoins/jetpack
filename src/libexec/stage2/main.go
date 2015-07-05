package main

import "flag"

import "os"

import "syscall" // Exec() is only in syscall, wtf?

import "golang.org/x/sys/unix"

func JailAttach(jid int) error {
	if _, _, err := unix.Syscall(unix.SYS_JAIL_ATTACH, uintptr(jid), 0, 0); err == 0 {
		return nil
	} else {
		return err
	}
}

var JID, Uid, Gid int
var Chroot, AppName string
var Env = make(dictFlag)
var Exec []string
var WorkingDirectory, MetadataURL string

func main() {
	flag.IntVar(&JID, "jid", -1, "Jail ID")
	flag.IntVar(&Uid, "uid", 0, "User to run as")
	flag.IntVar(&Gid, "gid", 0, "Group to run as")
	flag.StringVar(&Chroot, "chroot", "/", "Chroot within jail")
	flag.StringVar(&AppName, "name", "", "Application name")
	flag.StringVar(&WorkingDirectory, "cwd", "/", "Working directory")
	flag.StringVar(&MetadataURL, "mds", "", "Metadata server URL")
	flag.Var(&Env, "setenv", "Environment variables")

	flag.Parse()
	Exec = flag.Args()

	// TODO: sanity check?

	if err := JailAttach(JID); err != nil {
		panic(err)
	}

	if Chroot != "/" {
		if err := unix.Chroot(Chroot); err != nil {
			panic(err)
		}
	}

	if err := os.Chdir(WorkingDirectory); err != nil {
		panic(err)
	}

	if _, ok := Env["PATH"]; !ok {
		Env["PATH"] = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	}

	if _, ok := Env["TERM"]; !ok {
		term := os.Getenv("TERM")
		if term == "" {
			term = "vt100"
		}
		Env["TERM"] = term
	}

	Env["AC_APP_NAME"] = AppName
	Env["AC_METADATA_URL"] = MetadataURL

	envv := make([]string, 0, len(Env))
	for k, v := range Env {
		envv = append(envv, k+"="+v)
	}

	if err := unix.Setgroups([]int{}); err != nil {
		panic(err)
	}

	if err := unix.Setregid(Gid, Gid); err != nil {
		panic(err)
	}

	if err := unix.Setreuid(Uid, Uid); err != nil {
		panic(err)
	}

	// FIXME: setusercontext()?
	// See https://github.com/freebsd/freebsd/blob/master/usr.sbin/jexec/jexec.c#L123-L126

	os.Clearenv()

	if err := syscall.Exec(Exec[0], Exec, envv); err != nil {
		panic(err)
	}
}
