package zettajail

import "fmt"
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
  zjail set <JAIL> <PROPERTY>...
  zjail init [<PROPERTY>...]
  zjail snapshot <JAIL@SNAPSHOT>
  zjail -h | --help | --version
`
)

type Cli struct {
	// commands
	DoInfo     bool `mapstructure:"info"`
	DoInstall  bool `mapstructure:"install"`
	DoSet      bool `mapstructure:"set"`
	DoInit     bool `mapstructure:"init"`
	DoStatus   bool `mapstructure:"status"`
	DoStart    bool `mapstructure:"start"`
	DoStop     bool `mapstructure:"stop"`
	DoRestart  bool `mapstructure:"restart"`
	DoConsole  bool `mapstructure:"console"`
	DoSnapshot bool `mapstructure:"snapshot"`

	Jail       string   `mapstructure:"<JAIL>"`
	Snapshot   string   `mapstructure:"<JAIL@SNAPSHOT>"`
	properties []string `mapstructure:"<PROPERTY>"`
	Command    []string `mapstructure:"<COMMAND>"`

	raw map[string]interface{}
}

func ParseArgs() (cli *Cli, err error) {
	rawArgs, derr := docopt.Parse(usage, nil, true, Version, true)
	if derr != nil {
		err = derr
		return
	}

	cli = &Cli{raw: rawArgs}
	err = mapstructure.Decode(rawArgs, cli)
	if err != nil {
		log.Printf("%v -> %#v\n", err, rawArgs)
	}
	return
}

func (cli *Cli) GetJail() Jail {
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
		switch property[0] {
		case '+': // "+property" is raw ZFS property
			property = property[1:]
		case '@': // "@property" is zettajail: property
			property = "zettajail:" + property[1:]
		default: // "property" is zettajail:jail: (jail property)
			property = "zettajail:jail:" + property
			isJailProperty = true
		}

		splut := strings.SplitN(property, "=", 2)
		if len(splut) == 1 {
			if isJailProperty {
				// TODO: look for a "no"
				pmap[splut[0]] = "true"
			} else {
				pmap[splut[0]] = "on"
			}
		} else {
			if isJailProperty {
				pmap[splut[0]] = strconv.Quote(splut[1])
			} else {
				pmap[splut[0]] = splut[1]
			}
		}
	}
	return pmap
}

func (cli *Cli) Properties() map[string]string {
	return parseProperties(cli.properties)
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

func RunCli() error {
	if cli, err := ParseArgs(); err != nil {
		return err
	} else {
		return cli.Dispatch()
	}
}
