package main

import (
	"encoding/json"
	"fmt"

	"github.com/juju/errors"

	"lib/acutil"
)

func init() {
	AddCommand(".m ...", "Generate pod manifest", cmdDotM, flPodManifest)
}

func cmdDotM(args []string) error {
	// 1. Check if UUID
	// 2. Try to construct a new manifest

	if err := acutil.ParseApps(thePodManifest, args); err != nil {
		return errors.Trace(err)
	}

	for i, app := range thePodManifest.Apps {
		// fmt.Println(app.Image.ID, app.Image.ID.Empty())
		if app.Image.ID.Empty() {
			// app is a copy; we need to use index to write
			thePodManifest.Apps[i].Image.ID.Set("sha512-0")
		}
	}

	if mb, err := json.MarshalIndent(thePodManifest, "", "  "); err != nil {
		return errors.Trace(err)
	} else {
		fmt.Println(string(mb))
	}
	return nil
}
