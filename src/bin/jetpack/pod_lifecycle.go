package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/appc/spec/schema/types"
	"github.com/juju/errors"

	"lib/jetpack"
)

func init() {
	AddCommand("prepare ...", "Prepare a pod", cmdPrepare, flPrepare)
	AddCommand("run ...", "Run a pod", cmdWrapPodPrepare0(cmdRun), flRun)
}

var flDryRun bool

func flPrepare(fl *flag.FlagSet) {
	SaveIDFlag(fl)
	flPodManifest(fl)
	fl.BoolVar(&flDryRun, "n", false, "Dry run (don't actually create pod, just show reified manifest)")
}

func cmdPrepare(args []string) error {
	if pm, err := getPodManifest(args); err != nil {
		return errors.Trace(err)
	} else if flDryRun {
		if jb, err := json.MarshalIndent(pm, "", "  "); err != nil {
			return errors.Trace(err)
		} else {
			// TODO: is it a good place?
			fmt.Println(string(jb))
			return nil
		}
	} else if pod, err := Host.CreatePod(pm); err != nil {
		return errors.Trace(err)
	} else {
		if SaveID != "" {
			if err := ioutil.WriteFile(SaveID, []byte(pod.UUID.String()), 0644); err != nil {
				return errors.Trace(err)
			}
		}
		if !Quiet {
			// TODO: show pod
			fmt.Println(pod.UUID)
		}
		return nil
	}
}

var flAppName types.ACName
var flDestroy bool

func flRun(fl *flag.FlagSet) {
	flPodManifest(fl)
	SaveIDFlag(fl)
	fl.Var(&flAppName, "app", "Specify app to run for a multi-app pod")
	fl.BoolVar(&flDestroy, "destroy", false, "Destroy pod when done")
}

func cmdRun(pod *jetpack.Pod) (erv error) {
	if flAppName.Empty() {
		if len(pod.Manifest.Apps) != 1 {
			return errors.New("Multi-app pod! Please use -app=NAME to choose")
		} else {
			flAppName = pod.Manifest.Apps[0].Name
		}
	}
	if flDestroy {
		defer func() {
			if err := pod.Destroy(); err != nil {
				if erv == nil {
					erv = err
				} else {
					// TODO: UI
					fmt.Fprintln(os.Stderr, "ERROR destroying pod:", err)
				}
			}
		}()
	}
	return errors.Trace(pod.RunApp(flAppName))
}
