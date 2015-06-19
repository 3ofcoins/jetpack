package main

import (
	"fmt"
	"os"

	"github.com/juju/errors"
)

func init() {
	AddCommand("mds [stop|restart]", "Manage metadata service process", cmdMds, nil)
}

// FIXME: logic here, in lib/jetpack/mds.go, and the line in
// lib/jetpack/pod.go looks like it was written on jetlag. Which it
// was. Needs rewrite, but works.

func boolProp(val bool) string {
	if val {
		return "on"
	} else {
		return "off"
	}
}

const doAutostart = true

func cmdMds(args []string) error {
	Host.Properties.Set("mds.autostart", boolProp(doAutostart))

	if len(args) > 0 {
		switch args[0] {
		case "stop", "restart":
			Host.Properties.Set("mds.autostart", "off")
			if mdsi, _ := Host.NeedMDS(); mdsi == nil {
				// Already down
				if args[0] != "restart" {
					return nil
				}
			} else {
				// Ignore errors. If we can find any MDS, kill it.
				fmt.Println("Killing:", mdsi)
				if p, err := os.FindProcess(mdsi.Pid); err != nil {
					return errors.Trace(err)
				} else if err := p.Kill(); err != nil {
					return errors.Trace(err)
				}
			}
			if args[0] == "restart" {
				Host.Properties.Set("mds.autostart", "on")
			}
		}
	}

	mdsi, err := Host.NeedMDS()
	if mdsi != nil {
		fmt.Println(mdsi)
	}

	return err
}
