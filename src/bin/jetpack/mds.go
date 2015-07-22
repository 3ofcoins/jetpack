package main

import (
	"fmt"
	"os"

	"github.com/juju/errors"

	"lib/jetpack"
)

// FIXME: THE WHOLE THING

func init() {
	AddCommand("mds [stop|restart]", "Manage metadata service process", cmdMds, nil)
}

func cmdMds(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "stop", "restart":
			jetpack.Config().Set("mds.autostart", "off")
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
				jetpack.Config().Set("mds.autostart", "on")
			}
		}
	}

	mdsi, err := Host.NeedMDS()
	if mdsi != nil {
		fmt.Println(mdsi)
	}

	return err
}
