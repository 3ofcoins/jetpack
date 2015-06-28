package main

import (
	"fmt"
	"sort"
	"strings"
)

func init() {
	AddCommand("init", "Initialize host", cmdWrapErr(cmdInit), nil)
	AddCommand("config", "Show configuration", cmdWrap(cmdConfig), nil)
}

func cmdConfig() {
	lines := strings.Split(Host.Properties.String(), "\n")
	sort.Strings(lines)
	fmt.Println(strings.Join(lines[1:], "\n")) // first "line" is empty due to trailing newline
}

func cmdInit() error {
	return Host.Initialize()
}
