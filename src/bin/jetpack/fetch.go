package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/juju/errors"

	"lib/fetch"
	"lib/jetpack"
)

func init() {
	AddCommand("fetch LOCATION", "Fetch an image to local store", cmdFetch, flFetch)
}

var SigLocation string

func flFetch(fl *flag.FlagSet) {
	fetch.AllowHTTPFlag(fl)
	jetpack.AllowNoSignatureFlag(fl)
	SaveIDFlag(fl)
	fl.StringVar(&SigLocation, "sig", "", "Provide explicit signature location")
}

func cmdFetch(args []string) error {
	var idf *os.File
	if SaveID != "" {
		if f, err := os.Create(SaveID); err != nil {
			return err
		} else {
			idf = f
			defer f.Close()
		}
	}

	for _, name := range args {
		if img, err := Host.FetchImage(name, SigLocation); err != nil {
			return errors.Trace(err)
		} else if err := cmdShowImage(img); err != nil {
			return errors.Trace(err)
		} else if idf != nil {
			fmt.Fprintln(idf, img.Hash)
		}
	}

	return nil
}
