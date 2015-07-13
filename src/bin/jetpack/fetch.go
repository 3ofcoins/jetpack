package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/appc/spec/schema/types"
	"github.com/juju/errors"

	"lib/fetch"
)

func init() {
	AddCommand("fetch LOCATION", "Fetch an image to local store", cmdFetch, flFetch)
}

func flFetch(fl *flag.FlagSet) {
	fetch.AllowHTTPFlag(fl)
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
		if name, labels, err := parseImageName(name); err != nil {
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
