package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/juju/errors"

	"lib/fetch"
	"lib/jetpack"
)

func runFetch(args []string) error {
	var saveId, sigLocation string
	fl := flag.NewFlagSet("fetch", flag.ExitOnError)
	fl.StringVar(&sigLocation, "sig", "", "Provide explicit signature location")
	fl.StringVar(&saveId, "saveid", "", "Save ID of each fetched image to a file")
	fetch.AllowHTTPFlag(fl)
	jetpack.AllowNoSignatureFlag(fl)

	fl.Parse(args)
	args = fl.Args()

	var idf *os.File
	if saveId != "" {
		if f, err := os.Create(saveId); err != nil {
			return err
		} else {
			idf = f
			defer f.Close()
		}
	}

	for _, name := range args {
		if img, err := Host.FetchImage(name, sigLocation); err != nil {
			return errors.Trace(err)
		} else {
			show(img)
			if idf != nil {
				fmt.Fprintln(idf, img.Hash)
			}
		}
	}

	return nil
}
