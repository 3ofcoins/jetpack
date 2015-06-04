package fetch

import (
	"flag"
	"os"

	"github.com/appc/spec/discovery"
	"github.com/appc/spec/schema/types"
)

var AllowHTTP bool

func AllowHTTPFlag(fl *flag.FlagSet) {
	if fl == nil {
		fl = flag.CommandLine
	}
	fl.BoolVar(&AllowHTTP, "insecure-allow-http", false, "Allow non-encrypted HTTP")
}

func OpenPubKey(location string) (types.ACName, *os.File, error) {
	if app, err := discovery.NewAppFromString(location); err == nil {
		// Proper ACName given, let's do the discovery
		if eps, _, err := discovery.DiscoverPublicKeys(*app, AllowHTTP); err != nil {
			return app.Name, nil, err
		} else {
			// We assume multiple returned keys are alternatives, not
			// multiple different valid keychains.
			var erv error
			for _, keyurl := range eps.Keys {
				if keyf, err := OpenLocation(keyurl); err != nil {
					erv = err
				} else {
					return app.Name, keyf, nil
				}
			}
			// All keys erred
			return app.Name, nil, erv
		}
	} else {
		// Not an ACName, let's open as raw location
		f, err := OpenLocation(location)
		return "", f, err
	}
}
