package main

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/appc/spec/schema"
	"github.com/juju/errors"

	"lib/acutil"
)

func init() {
	AddCommand(".m ...", "Generate pod manifest", cmdDotM, flPod)
}

var flPodManifest = schema.BlankPodManifest()

func flPod(fl *flag.FlagSet) {
	acutil.PodManifestFlags(fl, flPodManifest)
}

func cmdDotM(args []string) error {
	if mb, err := json.MarshalIndent(flPodManifest, "", "  "); err != nil {
		return errors.Trace(err)
	} else {
		fmt.Println(string(mb))
	}
	return nil
}
