package main

import (
	"flag"
	"net/url"
	"os"
	"runtime"
	"strings"

	"github.com/appc/spec/discovery"
	"github.com/appc/spec/schema/types"
	"github.com/juju/errors"

	"lib/fetch"
	"lib/jetpack"
)

func isRawLocation(name string) bool {
	if name[0] == '/' || strings.HasPrefix(name, "./") {
		return true
	}
	if u, err := url.Parse(name); err == nil && u.Scheme != "" {
		return true
	}
	return false
}

func openACI(name, sig string) (types.ACName, *os.File, *os.File, error) {
	var aci, asc *os.File

	if sig != "" {
		if sf, err := fetch.OpenLocation(sig); err != nil {
			return jetpack.ACNoName, nil, nil, errors.Trace(err)
		} else {
			asc = sf
		}
	}

	if isRawLocation(name) {
		if af, err := fetch.OpenLocation(name); err != nil {
			asc.Close()
			return jetpack.ACNoName, nil, nil, errors.Trace(err)
		} else {
			return jetpack.ACNoName, af, asc, nil
		}
	}

	app, err := discovery.NewAppFromString(name)
	if err != nil {
		return app.Name, nil, nil, errors.Trace(err)
	}

	if app.Labels["os"] == "" {
		app.Labels["os"] = runtime.GOOS
	}

	if app.Labels["arch"] == "" {
		app.Labels["arch"] = runtime.GOARCH
	}

	eps, _, err := discovery.DiscoverEndpoints(*app, fetch.AllowHTTP)
	if err != nil {
		return app.Name, nil, nil, errors.Trace(err)
	}

	if asc == nil {
		err = nil
		for _, endpoint := range eps.ACIEndpoints {
			if f, err_ := fetch.OpenLocation(endpoint.ASC); err_ != nil {
				err = err_
			} else {
				asc = f
				break
			}
		}
		if asc == nil && err != nil {
			return app.Name, nil, nil, errors.Trace(err)
		}
	}

	for _, endpoint := range eps.ACIEndpoints {
		if f, err_ := fetch.OpenLocation(endpoint.ACI); err_ != nil {
			err = err_
		} else {
			aci = f
			break
		}
	}

	if aci == nil {
		asc.Close()
		return app.Name, nil, nil, errors.Trace(err)
	}

	return app.Name, aci, asc, nil
}

func runFetch(args []string) error {
	var sigLocation string
	var noSig bool
	fl := flag.NewFlagSet("fetch", flag.ExitOnError)
	fl.BoolVar(&noSig, "insecure-no-signature", false, "Skip signature checking")
	fl.StringVar(&sigLocation, "sig", "", "Provide explicit signature location")
	fetch.AllowHTTPFlag(fl)

	fl.Parse(args)
	args = fl.Args()

	for _, name := range args {
		if name, aci, asc, err := openACI(name, sigLocation); err != nil {
			return errors.Trace(err)
		} else {
			defer aci.Close()
			if asc == nil && !noSig {
				return errors.New("No signature")
			}
			defer asc.Close()

			if img, err := Host.ImportImageNG(name, aci, asc); err != nil {
				return errors.Trace(err)
			} else {
				show(img)
			}
		}
	}

	return nil
}
