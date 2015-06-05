package main

import (
	"flag"

	"github.com/juju/errors"

	"lib/fetch"
	"lib/jetpack"
)

func runFetch(args []string) error {
	var sigLocation string
	fl := flag.NewFlagSet("fetch", flag.ExitOnError)
	fl.StringVar(&sigLocation, "sig", "", "Provide explicit signature location")
	fetch.AllowHTTPFlag(fl)
	jetpack.AllowNoSignatureFlag(fl)

	fl.Parse(args)
	args = fl.Args()

	for _, name := range args {
		if img, err := Host.FetchImage(name, sigLocation); err != nil {
			return errors.Trace(err)
		} else {
			show(img)
		}
	}

	return nil
}
