package main

import (
	"flag"
	"fmt"
	"os"

	"lib/jetpack"
)

func init() {
	AddCommand("help [COMMAND]", "Show help", cmdHelp, nil)
	AddCommand("version", "Show jetpack version", cmdVersion, nil)
}

func cmdHelp(args []string) error {
	switch len(args) {
	case 0:
		Help()
		return nil
	case 1:
		if cmd := Commands[args[0]]; cmd != nil {
			cmd.Help()
			return nil
		}
		return ErrUsage
	default:
		return ErrUsage
	}
}

func cmdVersion([]string) error {
	fmt.Println(jetpack.Version())
	return nil
}

func Help() {
	fmt.Fprintf(os.Stderr, "Jetpack version %v\nUsage: %v [OPTION...] COMMAND [ARGS...]\nCommands:\n", jetpack.Version(), AppName)

	cmds := make([][]string, 0, len(Commands))
	for _, cmd := range Commands {
		cmds = append(cmds, []string{"", AppName + " " + cmd.Usage, cmd.Synopsis})
	}
	doListF(os.Stderr, "", cmds)

	fmt.Fprintln(os.Stderr, "Global options:")
	flag.PrintDefaults()
}
