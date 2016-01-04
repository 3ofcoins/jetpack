package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/juju/errors"

	"github.com/3ofcoins/jetpack/lib/jetpack"
)

func init() {
	AddCommand("init", "Initialize host", cmdWrapErr(cmdInit), nil)
	AddCommand("config [VAR...]", "Show configuration", cmdConfig, nil)
}

func cmdConfig(args []string) error {
	if len(args) == 0 {
		lines := strings.Split(jetpack.Config().String(), "\n")
		sort.Strings(lines)
		fmt.Println(strings.Join(lines[1:], "\n")) // first "line" is empty due to trailing newline
	} else {
		for _, propName := range args {
			if val, ok := jetpack.Config().Get(propName); ok {
				fmt.Println(val)
			} else {
				return errors.Errorf("No such property: %v", propName)
			}
		}
	}
	return nil
}

func cmdInit() error {
	return Host.Initialize()
}
