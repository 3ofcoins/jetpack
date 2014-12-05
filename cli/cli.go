package cli

import "errors"
import "flag"
import "fmt"
import "os"
import "sort"
import "strings"

var ErrUsage = errors.New("Invalid usage")

type CommandRunner func(string, []string) error

type Command struct {
	*flag.FlagSet
	Name     string
	CliName  string
	Synopsis string
	Runner   CommandRunner
}

func NewCommand(name, synopsis string, runner CommandRunner) *Command {
	cmd := &Command{
		FlagSet:  flag.NewFlagSet(name, flag.ContinueOnError),
		Name:     name,
		Synopsis: synopsis,
		Runner:   runner,
	}
	cmd.FlagSet.Usage = cmd.Usage
	return cmd
}

func (cmd *Command) Run() error {
	if !cmd.Parsed() {
		return errors.New("Cannot Run() before Parse()")
	}
	if err := cmd.Runner(cmd.Name, cmd.Args()); err == ErrUsage {
		cmd.Usage()
		os.Exit(2)
		return nil
	} else {
		return err
	}
}

func (cmd *Command) Usage() {
	fmt.Fprintln(os.Stderr, "Usage:", cmd)
	cmd.PrintDefaults()
}

func (cmd *Command) String() string {
	pieces := []string{}
	if cmd.CliName != "" {
		pieces = append(pieces, cmd.CliName)
	}
	pieces = append(pieces, cmd.Name, cmd.Synopsis)
	return strings.Join(pieces, " ")
}

type Cli struct {
	*flag.FlagSet
	Name     string
	Commands map[string]*Command
}

func (cli *Cli) Usage() {
	fmt.Fprintf(os.Stderr,
		"Usage: %s [flags] command [args...]\nKnown commands:\n  %s help [COMMAND] -- show help\n", cli.Name, cli.Name)
	commands := make([]string, 0, len(cli.Commands))
	for cmd := range cli.Commands {
		commands = append(commands, cmd)
	}
	sort.Strings(commands)
	for _, cmd := range commands {
		fmt.Fprintf(os.Stderr, "  %s\n", cli.Commands[cmd])
	}
	fmt.Fprintln(os.Stderr, "Global flags:")
	cli.PrintDefaults()
}

func NewCli(name string) *Cli {
	if name == "" {
		name = os.Args[0]
	}
	rv := &Cli{
		FlagSet:  flag.NewFlagSet(name, flag.ContinueOnError),
		Name:     name,
		Commands: make(map[string]*Command),
	}
	rv.FlagSet.Usage = rv.Usage
	return rv
}

func (cli *Cli) AddCommand(name, synopsis string, runner CommandRunner) {
	cmd := NewCommand(name, synopsis, runner)
	cmd.CliName = cli.Name
	cli.Commands[name] = cmd
}

func (cli *Cli) MustGetCommand(name string) *Command {
	if cmd, hasCommand := cli.Commands[name]; hasCommand {
		return cmd
	} else {
		fmt.Fprintln(os.Stderr, "Unknown command:", name)
		cli.Usage()
		os.Exit(2)
	}
	return nil
}

func (cli *Cli) Parse(args []string) {
	if args == nil {
		args = os.Args[1:]
	}

	if err := cli.FlagSet.Parse(args); err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		} else {
			os.Exit(2)
		}
	}

	if cli.NArg() == 0 || cli.Arg(0) == "help" {
		if cli.NArg() > 1 {
			cli.MustGetCommand(cli.Arg(1)).Usage()
		} else {
			cli.Usage()
		}
		os.Exit(0)
	}

	cli.MustGetCommand(cli.Arg(0))
}

func (cli *Cli) Run() error {
	cmd := cli.Commands[cli.Arg(0)]
	if err := cmd.Parse(cli.Args()[1:]); err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		} else {
			os.Exit(2)
		}
	}
	return cmd.Run()
}
