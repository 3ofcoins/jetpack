package main

import "strings"

import "github.com/docopt/docopt-go"
import "github.com/mitchellh/mapstructure"

var (
	Version = "0.0.0-wip0"
	usage   = `Usage:
  zjail list
  zjail install <jail>
  zjail info [<jail>]
  zjail set <jail> <property>...
  zjail -h | --help | --version
`
)

type Cli struct {
	// commands
	DoInfo    bool `mapstructure:"info"`
	DoInstall bool `mapstructure:"install"`
	DoSet     bool `mapstructure:"set"`

	Jail       string   `mapstructure:"<jail>"`
	Properties []string `mapstructure:"<property>"`
}

func ParseArgs() (cli Cli, err error) {
	rawArgs, derr := docopt.Parse(usage, nil, true, Version, true)
	if derr != nil {
		err = derr
		return
	}

	err = mapstructure.Decode(rawArgs, &cli)
	return
}

func (cli Cli) GetJail() Jail {
	return GetJail(cli.Jail)
}

func (cli Cli) GetProperties() map[string]string {
	if cli.Properties == nil {
		return nil
	}
	rv := make(map[string]string)
	for _, property := range cli.Properties {
		splut := strings.SplitN(property, "=", 2)
		if len(splut) == 1 {
			rv[splut[0]] = ""
		} else {
			rv[splut[0]] = splut[1]
		}
	}
	return rv
}
