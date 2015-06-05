package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"

	"golang.org/x/sys/unix"

	"lib/jetpack"
)

func runMds(args []string) error {
	var pidfile, logfile string
	fl := flag.NewFlagSet("mds", flag.ExitOnError)
	fl.StringVar(&pidfile, "pid", "", "Save process ID to a file")
	fl.StringVar(&logfile, "log", "/var/log/jetpack.mds.log", "Save logs to a file")
	fl.Parse(args)
	args = fl.Args()

	u, err := user.Lookup(Host.Properties.MustGetString("mds.user"))
	if err != nil {
		return err
	}

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return err
	}

	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return err
	}

	lf, err := os.OpenFile(logfile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return err
	}

	var pf *os.File
	if pidfile != "" {
		pf, err := os.Create(pidfile)
		if err != nil {
			lf.Close()
			return err
		}
		defer pf.Close()
	}

	cmd := exec.Command(filepath.Join(jetpack.LibexecPath, "mds"), args...)
	cmd.Stdin = nil
	cmd.Stdout = lf
	cmd.Stderr = lf
	cmd.Dir = "/"

	if err := unix.Setgroups([]int{}); err != nil {
		return err
	}

	if err := unix.Setregid(gid, gid); err != nil {
		return err
	}

	if err := unix.Setreuid(uid, uid); err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	if pf != nil {
		fmt.Fprintln(pf, cmd.Process.Pid)
	}

	fmt.Printf("Metadata service started as PID %d, logging to %s.\n", cmd.Process.Pid, logfile)
	return nil
}
