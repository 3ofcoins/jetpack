package main

import (
	"flag"
	"fmt"
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
