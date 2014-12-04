package zettajail

import "errors"
import "flag"
import "fmt"
import "log"
import "os"
import "sort"
import "strings"
import "strconv"

import "github.com/docopt/docopt-go"
import "github.com/mitchellh/mapstructure"

var (
	Version = "0.0.0-wip0"
	usage   = `Usage:
  zjail create <JAIL> [<PROPERTY>...]
  zjail info [<JAIL>]
  zjail status [<JAIL>]
  zjail (start|stop|restart) <JAIL>
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
	DoInfo     bool `mapstructure:"info"`
	DoCreate   bool `mapstructure:"create"`
	DoSet      bool `mapstructure:"set"`
	DoInit     bool `mapstructure:"init"`
	DoStatus   bool `mapstructure:"status"`
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

func parseProperties(properties []string) map[string]string {
	if properties == nil {
		return nil
	}
	pmap := make(map[string]string)
	for _, property := range properties {
		isJailProperty := false
		prefix := ""

		switch property[0] {
		case '+': // "+property" is raw ZFS property
			property = property[1:]
		case '@': // "@property" is zettajail: property
			property = property[1:]
			prefix = "zettajail:"
		default: // "property" is zettajail:jail: (jail property)
			prefix = "zettajail:jail:"
			isJailProperty = true
		}

		if splut := strings.SplitN(property, "=", 2); len(splut) == 1 {
			// No "=" in string -> a flag

			// Check for negation
			isTrue := true
			if strings.HasPrefix(property, "no") {
				property = property[2:]
				isTrue = false
			} else if strings.Contains(property, ".no") {
				property = strings.Replace(property, ".no", ".", 1)
				isTrue = false
			}

			if isJailProperty {
				if isTrue {
					pmap[prefix+property] = "true"
				} else {
					pmap[prefix+property] = "false"
				}
			} else {
				if isTrue {
					pmap[prefix+property] = "on"
				} else {
					pmap[prefix+property] = "off"
				}
			}
		} else {
			if isJailProperty {
				pmap[prefix+splut[0]] = strconv.Quote(splut[1])
			} else {
				pmap[prefix+splut[0]] = splut[1]
			}
		}
	}
	return pmap
}

func (cli *OldCli) Properties() map[string]string {
	return parseProperties(cli.RawProperties)
}

func (cli *OldCli) Dispatch() error {
	switch {
	case cli.DoInfo && cli.Jail == "":
		return cli.CmdGlobalInfo()
	case cli.DoInfo:
		return cli.CmdJailInfo(cli.GetJail())
	case cli.DoCreate:
		log.Printf("%#v\n", cli)
		return nil
		// return cli.CmdCreate()
	case cli.DoSet:
		return cli.GetJail().SetProperties(cli.Properties())
	case cli.DoStatus && cli.Jail == "":
		Host.Status()
		return nil
	case cli.DoStatus:
		cli.GetJail().Status()
		return nil
	case cli.DoStart:
		return cli.GetJail().RunJail("-c")
	case cli.DoStop:
		return cli.GetJail().RunJail("-r")
	case cli.DoRestart:
		return cli.GetJail().RunJail("-rc")
	case cli.DoConsole:
		return cli.GetJail().RunJexec("", cli.Command)
	case cli.DoInit:
		return Host.Init(cli.Properties())
	case cli.DoSnapshot:
		return cli.CmdSnapshot(cli.GetJail())
	default:
		return fmt.Errorf("CAN'T HAPPEN: %#v", cli)
	}
}

func oldRunCli() error {
	if cli, err := ParseArgs(); err != nil {
		return err
	} else {
		return cli.Dispatch()
	}
}

type CommandRunner func(string, []string, interface{}) error

type Command struct {
	*flag.FlagSet
	Name     string
	Synopsis string
	Runner   CommandRunner
	Sink     interface{}
}

func NewCommand(name, synopsis string, runner CommandRunner, sink interface{}) *Command {
	cmd := &Command{
		FlagSet:  flag.NewFlagSet(name, flag.ContinueOnError),
		Name:     name,
		Synopsis: synopsis,
		Runner:   runner,
		Sink:     sink,
	}
	cmd.FlagSet.Usage = cmd.Usage
	return cmd
}

func (cmd *Command) Run() error {
	if !cmd.Parsed() {
		return errors.New("Cannot Run() before Parse()")
	}
	return cmd.Runner(cmd.Name, cmd.Args(), cmd.Sink)
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

func (cli *Cli) AddCommand(name, synopsis string, runner CommandRunner, sink interface{}) *Command {
	cmd := NewCommand(name, synopsis, runner, sink)
	cli.Commands[name] = cmd
	return cmd
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
