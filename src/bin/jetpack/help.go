package main

import (
	"flag"
	"fmt"
	"os"

	"lib/jetpack"
)

func init() {
	AddCommand("help [COMMAND]", "Show help", cmdHelp, nil)
	AddCommand("version", "Show jetpack version", cmdVersion, flVersion)
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

func flVersion(fl *flag.FlagSet) {
	QuietFlag(fl, "show only version number")
}

func cmdVersion([]string) error {
	if Quiet {
		fmt.Println(jetpack.Version)
	} else {
		isdev := ""
		if jetpack.IsDevelopment {
			isdev = " (development)"
		}
		fmt.Printf("JetPack %v (%v), compiled on %v%v\n",
			jetpack.Version, jetpack.Revision, jetpack.BuildTimestamp, isdev)
	}
	return nil
}

func Help() {
	fmt.Fprintf(os.Stderr, "Usage: %v [OPTION...] COMMAND [ARGS...]\nCommands:\n", AppName)

	cmds := make([][]string, len(Commands))
	i := 0
	for _, cmd := range Commands {
		cmds[i] = []string{"", AppName + " " + cmd.Usage, cmd.Synopsis}
		i++
	}
	doListF(os.Stderr, "", cmds)

	fmt.Fprintln(os.Stderr, "Global options:")
	flag.PrintDefaults()
}
