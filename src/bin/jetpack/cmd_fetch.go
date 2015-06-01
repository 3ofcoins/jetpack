package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/appc/spec/discovery"
	"github.com/appc/spec/schema/types"
	"github.com/juju/errors"
)

func openACI(name string) (types.ACName, *os.File, *os.File, error) {
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

	eps, _, err := discovery.DiscoverEndpoints(*app, flagAllowHTTP)
	if err != nil {
		return app.Name, nil, nil, errors.Trace(err)
	}

	var aci, asc *os.File

	for _, endpoint := range eps.ACIEndpoints {
		if f, err_ := openLocation(endpoint.ASC); err_ != nil {
			err = err_
		} else {
			asc = f
			break
		}
	}

	if asc == nil {
		return app.Name, nil, nil, errors.Trace(err)
	}

	for _, endpoint := range eps.ACIEndpoints {
		if f, err_ := openLocation(endpoint.ACI); err_ != nil {
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
	fl := flag.NewFlagSet("fetch", flag.ExitOnError)
	fl.BoolVar(&flagAllowHTTP, "insecure-allow-http", false, "allow HTTP use for key discovery and/or retrieval")

	die(fl.Parse(args))
	args = fl.Args()

	for _, name := range args {
		if name, aci, asc, err := openACI(name); err != nil {
			return errors.Trace(err)
		} else {
			defer aci.Close()
			defer asc.Close()

			ks := Host.Keystore()
			if ety, err := ks.CheckSignature(name, aci, asc); err != nil {
				return errors.Trace(err)
			} else {
				fmt.Println("Valid signature for", name, "by:")
				fmt.Println(prettyKey(ety))
			}
		}
	}

	return nil
}
