package zettajail

import "errors"
import "flag"
import "fmt"
import "log"
import "os"
import "sort"
import "strings"

import "github.com/docopt/docopt-go"
import "github.com/mitchellh/mapstructure"

var (
	Version = "0.0.0-wip0"
	usage   = `Usage:
  zjail create <JAIL> [<PROPERTY>...]
  zjail console <JAIL> [<COMMAND>...]
  zjail set <JAIL> <PROPERTY>...
  zjail init [<PROPERTY>...]
  zjail snapshot <JAIL@SNAPSHOT>
  zjail -h | --help | --version

Options:
  -h, --help          Show help
  --version           Show version number
`
)

type OldCli struct {
	// commands
	DoCreate   bool `mapstructure:"create"`
	DoSet      bool `mapstructure:"set"`
	DoInit     bool `mapstructure:"init"`
	DoStart    bool `mapstructure:"start"`
	DoStop     bool `mapstructure:"stop"`
	DoRestart  bool `mapstructure:"restart"`
	DoConsole  bool `mapstructure:"console"`
	DoSnapshot bool `mapstructure:"snapshot"`

	Jail          string   `mapstructure:"<JAIL>"`
	Snapshot      string   `mapstructure:"<JAIL@SNAPSHOT>"`
	RawProperties []string `mapstructure:"<PROPERTY>"`
	Command       []string `mapstructure:"<COMMAND>"`

	raw map[string]interface{}
}

func ParseArgs() (cli *OldCli, err error) {
	rawArgs, derr := docopt.Parse(usage, nil, true, Version, true)
	if derr != nil {
		err = derr
		return
	}

	cli = &OldCli{raw: rawArgs}
	err = mapstructure.Decode(rawArgs, cli)
	if err != nil {
		log.Printf("%v -> %#v\n", err, rawArgs)
	}
	return
}

func (cli *OldCli) GetJail() Jail {
	if cli.Jail == "" && cli.Snapshot == "" {
		log.Fatalln("No jail given")
	}
	if cli.Jail == "" {
		splut := strings.Split(cli.Snapshot, "@")
		if len(splut) != 2 {
			log.Fatalf("Invalid JAIL@SNAPSHOT spec: %#v\n", cli.Snapshot)
		}
		cli.Jail = splut[0]
		cli.Snapshot = splut[1]
	}
	jail := GetJail(cli.Jail)
	if !jail.Exists() {
		log.Fatalln("Jail does not exist:", cli.Jail)
	}
	return jail
}

func (cli *OldCli) Properties() map[string]string {
	return ParseProperties(cli.RawProperties)
}

func (cli *OldCli) Dispatch() error {
	switch {
	case cli.DoCreate:
		log.Printf("%#v\n", cli)
		return nil
		// return cli.CmdCreate()
	default:
		return fmt.Errorf("CAN'T HAPPEN: %#v", cli)
	}
}

var ErrUsage = errors.New("Invalid usage")

type CommandRunner func(string, []string) error

type Command struct {
	*flag.FlagSet
	Name     string
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
	return cmd.Name + " " + cmd.Synopsis
}

type Cli struct {
	*flag.FlagSet
	Name     string
	Commands map[string]*Command
}

func (cli *Cli) Usage() {
	fmt.Fprintf(os.Stderr,
		"Usage: %s [flags] command [args...]\nKnown commands:\n", cli.Name)
	commands := make([]string, 0, len(cli.Commands))
	for cmd := range cli.Commands {
		commands = append(commands, cmd)
	}
	sort.Strings(commands)
	for _, cmd := range commands {
		fmt.Fprintf(os.Stderr, "  %s %s\n", cli.Name, cli.Commands[cmd])
	}
	fmt.Fprintf(os.Stderr, "  %s help [COMMAND]\nGlobal flags:\n", cli.Name)
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

func (cli *Cli) AddCommand(cmd *Command) {
	cli.Commands[cmd.Name] = cmd
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
