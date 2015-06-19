package main

import (
	stderrors "errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/juju/errors"
)

var ErrUsage = stderrors.New("Invalid usage")

type CommandHandler func([]string) error

type Command struct {
	Usage, Synopsis string
	handler         CommandHandler
	flag            *flag.FlagSet
}

var Commands = make(map[string]*Command)

func AddCommand(usage, synopsis string, handler CommandHandler, flHandler func(*flag.FlagSet)) *Command {
	space := strings.IndexFunc(usage, unicode.IsSpace)
	if space < 0 {
		space = len(usage)
	}
	name := usage[:space]

	cmd := &Command{
		Usage:    usage,
		Synopsis: synopsis,
		handler:  handler,
	}

	if flHandler != nil {
		cmd.flag = flag.NewFlagSet(name, flag.ExitOnError)
		flHandler(cmd.flag)
	}

	Commands[name] = cmd
	return cmd
}

func (cmd *Command) String() string {
	if cmd.Synopsis != "" {
		return fmt.Sprintf("%v -- %v", cmd.Usage, cmd.Synopsis)
	}
	return cmd.Usage
}

func (cmd *Command) Help() {
	fmt.Fprintf(os.Stderr, "%v\n\nUsage: %v %v\n",
		cmd.Synopsis, AppName, cmd.Usage)
	if cmd.flag != nil {
		fmt.Fprintln(os.Stderr, "Options:")
		cmd.flag.PrintDefaults()
	}
}

func (cmd *Command) Run(args []string) error {
	if cmd.flag != nil {
		cmd.flag.Parse(args)
		args = cmd.flag.Args()
	}
	err := cmd.handler(args)
	if err == ErrUsage {
		return fmt.Errorf("Usage: %v %v", AppName, cmd.Usage)
	}
	return err
}

func cmdWrap(fn func()) CommandHandler {
	return func([]string) error { fn(); return nil }
}

func cmdWrapErr(fn func() error) CommandHandler {
	return func([]string) error { return errors.Trace(fn()) }
}
