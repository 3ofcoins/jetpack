package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/juju/errors"

	"lib/jetpack"
	"lib/ui"
)

const AppName = "jetpack"

// Die on error
func Die(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, errors.ErrorStack(err))
		os.Exit(1)
	}
}

var Host *jetpack.Host

func main() {
	configPath := jetpack.DefaultConfigPath

	if cfg := os.Getenv("JETPACK_CONF"); cfg != "" {
		configPath = cfg
	}

	flag.StringVar(&configPath, "config", configPath, "Configuration file")
	flag.BoolVar(&ui.Debug, "debug", false, "Show debugging info")

	flag.Parse()

	if h, err := jetpack.NewHost(configPath); err != nil {
		Die(err)
	} else {
		Host = h
	}

	if args := flag.Args(); len(args) == 0 {
		Help()
	} else if cmd, ok := Commands[args[0]]; ok {
		Die(cmd.Run(args[1:]))
	} else {
		Help()
		os.Exit(1)
	}
}
