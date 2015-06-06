package fetch

import (
	"flag"
	"os"
	"runtime"

	"github.com/appc/spec/discovery"
	"github.com/appc/spec/schema/types"
	multierror "github.com/hashicorp/go-multierror"
)

var AllowHTTP bool

func AllowHTTPFlag(fl *flag.FlagSet) {
	if fl == nil {
		fl = flag.CommandLine
	}
	fl.BoolVar(&AllowHTTP, "insecure-allow-http", false, "Allow non-encrypted HTTP")
}

func tryAppFromString(location string) *discovery.App {
	if app, err := discovery.NewAppFromString(location); err != nil {
		return nil
	} else {
		if app.Labels["os"] == "" {
			app.Labels["os"] = runtime.GOOS
		}

		if app.Labels["arch"] == "" {
			app.Labels["arch"] = runtime.GOARCH
		}
		return app
	}
}

func OpenPubKey(location string) (types.ACName, *os.File, error) {
	if app := tryAppFromString(location); app != nil {
		// Proper ACName given, let's do the discovery
		if eps, _, err := discovery.DiscoverPublicKeys(*app, AllowHTTP); err != nil {
			return app.Name, nil, err
		} else {
			// We assume multiple returned keys are alternatives, not
			// multiple different valid keychains.
			var err error
			for _, keyurl := range eps.Keys {
				if keyf, er1 := OpenLocation(keyurl); er1 != nil {
					err = multierror.Append(err, er1)
				} else {
					return app.Name, keyf, nil
				}
			}
			// All keys erred
			return app.Name, nil, err
		}
	} else {
		// Not an ACName, let's open as raw location
		f, err := OpenLocation(location)
		return "", f, err
	}
}

func DiscoverACI(app discovery.App) (*os.File, *os.File, error) {
	return discoverACI(app, nil)
}

func discoverACI(app discovery.App, asc *os.File) (*os.File, *os.File, error) {
	var aci *os.File
	if eps, _, err := discovery.DiscoverEndpoints(app, AllowHTTP); err != nil {
		return nil, nil, err
	} else {
		var err error

		if asc == nil {
			err = nil
			for _, ep := range eps.ACIEndpoints {
				if af, er1 := OpenLocation(ep.ASC); er1 != nil {
					err = multierror.Append(err, er1)
				} else {
					asc = af
					break
				}
			}
			if asc == nil {
				return nil, nil, err
			}
		}

		err = nil
		for _, ep := range eps.ACIEndpoints {
			if af, er1 := OpenLocation(ep.ACI); er1 != nil {
				err = multierror.Append(err, er1)
			} else {
				aci = af
				break
			}
			if aci == nil {
				if asc != nil {
					asc.Close()
				}
				return nil, nil, err
			}
		}

		return aci, asc, nil
	}
}

func OpenACI(location, sigLocation string) (types.ACName, *os.File, *os.File, error) {
	var asc *os.File

	// Signature override
	if sigLocation != "" {
		if sf, err := OpenLocation(sigLocation); err != nil {
			return "", nil, nil, err
		} else {
			asc = sf
		}
	}

	if app := tryAppFromString(location); app != nil {
		// Proper ACName given, let's do discovery
		if aci, asc, err := discoverACI(*app, asc); err != nil {
			return app.Name, nil, nil, err
		} else {
			return app.Name, aci, asc, nil
		}
	} else {
		if aci, err := OpenLocation(location); err != nil {
			if asc != nil {
				asc.Close()
			}
			return "", nil, nil, err
		} else {
			return "", aci, asc, nil
		}
	}
}
