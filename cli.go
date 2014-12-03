package main

import "errors"
import "log"
import "strings"
import "strconv"

import "github.com/docopt/docopt-go"
import "github.com/mitchellh/mapstructure"

var (
	Version = "0.0.0-wip0"
	usage   = `Usage:
  zjail install <JAIL>
  zjail info [<JAIL>]
  zjail status [<JAIL>]
  zjail (start|stop|restart) <JAIL>
  zjail console <JAIL> [<COMMAND>...]
  zjail set <JAIL> [-o ZFSPROP]... <PROPERTY>...
  zjail init [-o ZFSPROP]... [<PROPERTY>...]
  zjail -h | --help | --version

Options:
  -o ZFSPROP   Literal ZFS property
`
)

type Cli struct {
	// commands
	DoInfo    bool `mapstructure:"info"`
	DoInstall bool `mapstructure:"install"`
	DoSet     bool `mapstructure:"set"`
	DoInit    bool `mapstructure:"init"`
	DoStatus  bool `mapstructure:"status"`
	DoStart   bool `mapstructure:"start"`
	DoStop    bool `mapstructure:"stop"`
	DoRestart bool `mapstructure:"restart"`
	DoConsole bool `mapstructure:"console"`

	Jail           string   `mapstructure:"<JAIL>"`
	JailProperties []string `mapstructure:"<PROPERTY>"`
	ZfsProperties  []string `mapstructure:"-o"`
	Command        []string `mapstructure:"<COMMAND>"`
}

func ParseArgs() (cli Cli, err error) {
	rawArgs, derr := docopt.Parse(usage, nil, true, Version, true)
	if derr != nil {
		err = derr
		return
	}

	err = mapstructure.Decode(rawArgs, &cli)
	if err != nil {
		log.Printf("%v -> %#v\n", err, rawArgs)
	}
	return
}

func (cli Cli) GetJail() Jail {
	jail := GetJail(cli.Jail)
	if !jail.Exists() {
		log.Fatalln("Jail does not exist:", cli.Jail)
	}
	return jail
}

func (cli Cli) ParseProperties() map[string]string {
	if cli.JailProperties == nil && cli.ZfsProperties == nil {
		return nil
	}
	rv := make(map[string]string)
	for _, property := range cli.JailProperties {
		splut := strings.SplitN(property, "=", 2)
		if len(splut) == 1 {
			if strings.HasPrefix(splut[0], "no") {
				rv["jail:"+splut[0][2:]] = "false"
			} else {
				rv["jail:"+splut[0]] = "true"
			}
		} else {
			rv["jail:"+splut[0]] = strconv.Quote(splut[1])
		}
	}

	for _, property := range cli.ZfsProperties {
		splut := strings.SplitN(property, "=", 2)
		if len(splut) == 1 {
			rv[splut[0]] = "on"
		} else {
			rv[splut[0]] = splut[1]
		}
	}

	return rv
}

func (cli *Cli) Dispatch() error {
	switch {
	case cli.DoInfo && cli.Jail == "":
		return cli.CmdGlobalInfo()
	case cli.DoInfo:
		return cli.CmdJailInfo(cli.GetJail())
	case cli.DoInstall:
		return cli.CmdInstall()
	case cli.DoSet:
		return cli.GetJail().SetProperties(cli.ParseProperties())
	case cli.DoStatus && cli.Jail == "":
		return Host.Status()
	case cli.DoStatus:
		return cli.GetJail().Status()
	case cli.DoStart:
		return cli.GetJail().RunJail("-c")
	case cli.DoStop:
		return cli.GetJail().RunJail("-r")
	case cli.DoRestart:
		return cli.GetJail().RunJail("-rc")
	case cli.DoConsole:
		return cli.GetJail().RunJexec("", cli.Command)
	case cli.DoInit:
		return Host.Init(cli.ParseProperties())
	default:
		return errors.New("CAN'T HAPPEN")
	}
}

func RunCli() error {
	if cli, err := ParseArgs(); err != nil {
		return err
	} else {
		return cli.Dispatch()
	}
}
