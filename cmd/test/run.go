package main

import (
	"os"
	"os/exec"
)

func iRun(command string) error {
	// TODO: proper output management
	out, err := exec.Command("/bin/sh", "-c", command).CombinedOutput()
	if err != nil {
		os.Stderr.Write(out)
	}
	return err
}
