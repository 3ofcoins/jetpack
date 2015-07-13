package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"

	"github.com/appc/spec/schema/types"
	"github.com/juju/errors"

	"lib/jetpack"
	"lib/pod_constructor"
)

func init() {
	AddCommand("prepare ...", "Prepare a pod", cmdPrepare, flPrepare)
	AddCommand("run UUID[:APP]", "Run a pod", cmdWrapMustApp0(cmdRun), nil)
}

var flDryRun, flRun, flDestroy bool

func flPrepare(fl *flag.FlagSet) {
	SaveIDFlag(fl)
	fl.BoolVar(&flDryRun, "n", false, "Dry run (don't actually create pod, just show reified manifest)")
	fl.BoolVar(&flRun, "run", false, "Run created pod")
	fl.BoolVar(&flDestroy, "destroy", false, "Destroy created pod")
}

func cmdPrepare(args []string) error {
	if pod, err := preparePod(args); err != nil {
		return errors.Trace(err)
	} else if pod == nil {
		// Dry run is on, manifest has been displayed, nothing to do for us
	} else {
		if flDestroy {
			defer pod.Destroy()
		}
		if flRun {
			if len(pod.Manifest.Apps) > 1 {
				return errors.New("Pod has more than one app")
			}
			return errors.Trace(pod.RunApp(pod.Manifest.Apps[0].Name))
		} else {
			fmt.Println(pod.UUID) // TODO: pod show
		}
	}
	return nil
}

func preparePod(args []string) (*jetpack.Pod, error) {
	if pm, err := pod_constructor.ConstructPodManifest(nil, args); err != nil {
		return nil, errors.Trace(err)
	} else if pm, err := Host.ReifyPodManifest(pm); err != nil {
		return nil, errors.Trace(err)
	} else {
		if flDryRun {
			if jb, err := json.MarshalIndent(pm, "", "  "); err != nil {
				return nil, errors.Trace(err)
			} else {
				// TODO: is it a good place?
				fmt.Println(string(jb))
				return nil, nil
			}
		}

		if pod, err := Host.CreatePod(pm); err != nil {
			return nil, errors.Trace(err)
		} else {
			if SaveID != "" {
				if err := ioutil.WriteFile(SaveID, []byte(pod.UUID.String()), 0644); err != nil {
					return nil, errors.Trace(err)
				}
			}
			return pod, nil
		}
	}
}

func cmdRun(pod *jetpack.Pod, appName types.ACName) error {
	return errors.Trace(pod.RunApp(appName))
}
