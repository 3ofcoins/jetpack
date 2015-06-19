package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"

	"code.google.com/p/go-uuid/uuid"

	"github.com/juju/errors"

	"lib/jetpack"
	"lib/pod_constructor"
)

func init() {
	AddCommand("prepare ...", "Prepare a pod", cmdPrepare, flPrepare)
	AddCommand("run UUID|...", "Run a pod", cmdRun, flRun)
}

var flDryRun bool

func flPrepare(fl *flag.FlagSet) {
	flRunPrepareCommon(fl)
	fl.BoolVar(&flDryRun, "n", false, "Dry run (don't actually create pod, just show reified manifest)")
}

func flRun(fl *flag.FlagSet) {
	flRunPrepareCommon(fl)
	flAppName(fl)
}

func flRunPrepareCommon(fl *flag.FlagSet) {
	SaveIDFlag(fl)
}

func cmdPrepare(args []string) error {
	if pod, err := preparePod(args); err != nil {
		return errors.Trace(err)
	} else if pod == nil {
		// Dry run is on, manifest has been displayed, nothing to do for us
	} else {
		fmt.Println(pod.UUID) // TODO: pod show
	}
	return nil
}

func cmdRun(args []string) error {
	if pod, err := getOrPreparePod(args); err != nil {
		return errors.Trace(err)
	} else if err := guessAppNameFlag(pod); err != nil {
		return errors.Trace(err)
	} else {
		return errors.Trace(pod.RunApp(AppNameFlag))
	}
}

func getOrPreparePod(args []string) (*jetpack.Pod, error) {
	if len(args) == 1 {
		if id := uuid.Parse(args[0]); id != nil {
			return Host.GetPod(id)
		}
	}

	return preparePod(args)
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
