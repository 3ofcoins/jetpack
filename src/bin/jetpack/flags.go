package main

import (
	"errors"
	"flag"
	"fmt"

	"github.com/appc/spec/schema/types"

	"lib/jetpack"
)

// Custom flag types

type sliceFlag []string

func (sf *sliceFlag) String() string {
	return fmt.Sprintf("%v", *sf)
}

func (sf *sliceFlag) Set(v string) error {
	*sf = append(*sf, v)
	return nil
}

// Command line flags used by different commands

var SaveID string

func SaveIDFlag(fl *flag.FlagSet) {
	fl.StringVar(&SaveID, "saveid", "", "Save ID to file")
}

var Quiet bool

func QuietFlag(fl *flag.FlagSet, desc string) {
	fl.BoolVar(&Quiet, "q", false, fmt.Sprintf("quiet (%v)", desc))
}

var AppNameFlag types.ACName

func guessAppNameFlag(pod *jetpack.Pod) error {
	if AppNameFlag.Empty() {
		if len(pod.Manifest.Apps) > 1 {
			return errors.New("This is a multi-app pod, and no name was given")
		}
		AppNameFlag = pod.Manifest.Apps[0].Name
	}
	return nil
}

func flAppName(fl *flag.FlagSet) {
	fl.Var(&AppNameFlag, "app", "Specify app to run (if none given, runs pods only app)")
}
