package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/appc/spec/schema/types"
	"github.com/juju/errors"

	"github.com/3ofcoins/jetpack/lib/acutil"
	"github.com/3ofcoins/jetpack/lib/fetch"
)

func init() {
	AddCommand("fetch NAME", "Discover and fetch an image", cmdFetch, flFetch)
	AddCommand("import LOCATION", "Import an image directly from location", cmdImport, flImport)
}

func flFetch(fl *flag.FlagSet) {
	SaveIDFlag(fl)
}

func cmdFetch(args []string) error {
	var idf *os.File
	if SaveID != "" {
		if f, err := os.Create(SaveID); err != nil {
			return err
		} else {
			idf = f
			defer idf.Close()
		}
	}

	for _, name := range args {
		if name, labels, err := acutil.ParseImageName(name); err != nil {
			return errors.Trace(err)
		} else if img, err := Host.FetchImage(types.Hash{}, name, labels); err != nil {
			return errors.Trace(err)
		} else {
			if idf != nil {
				fmt.Fprintln(idf, img.Hash)
			}
			if err := cmdShowImage(img); err != nil {
				return errors.Trace(err)
			}
		}
	}

	return nil
}

var flImportName types.ACIdentifier
var flImportSignature string

func flImport(fl *flag.FlagSet) {
	SaveIDFlag(fl)
	fl.Var(&flImportName, "name", "Name of imported image (for signature check)")
	fl.StringVar(&flImportSignature, "sig", "", "Location of signature")
}

func cmdImport(args []string) error {
	if len(args) != 1 {
		return ErrUsage
	}

	var idf *os.File
	if SaveID != "" {
		if f, err := os.Create(SaveID); err != nil {
			return err
		} else {
			idf = f
			defer idf.Close()
		}
	}

	aci, err := fetch.OpenLocation(args[0])
	if err != nil {
		return errors.Trace(err)
	}

	var asc *os.File
	if flImportSignature != "" {
		if asc_, err := fetch.OpenLocation(flImportSignature); err != nil {
			return errors.Trace(err)
		} else {
			asc = asc_
		}
	}

	if img, err := Host.ImportImage(flImportName, aci, asc); err != nil {
		return errors.Trace(err)
	} else {
		if idf != nil {
			fmt.Fprintln(idf, img.Hash)
		}
		return cmdShowImage(img)
	}
}
