package zettajail

import "fmt"
import "io"
import "log"
import "path"
import "text/template"

import "github.com/3ofcoins/go-zfs"

var jailConfTmpl *template.Template

func init() {
	tmpl, err := template.New("jail.conf").Parse(`"{{.}}" {
  path = "{{.Mountpoint}}";
  exec.consolelog = "{{.Mountpoint}}.log";
{{ range .JailParameters }}  {{.}};
{{ end }}}
`)
	jailConfTmpl = tmpl
	if err != nil {
		log.Fatalln(err)
	}
}

type Jail struct{ Dataset }

var ZeroJail = Jail{}

func GetJail(name string) (Jail, error) {
	ds, err := GetDataset(path.Join(Host.Name, name))
	if err != nil {
		return ZeroJail, err
	}
	if ds.Type == "filesystem" && ds.Properties["zettajail:jail"] == "on" {
		return Jail{ds}, nil
	} else {
		return ZeroJail, fmt.Errorf("Not a jail: %v", ds.Name)
	}
}

func newJailProperties(name string, properties map[string]string) map[string]string {
	if properties == nil {
		properties = make(map[string]string)
	}

	// Set mountpoint to have a reference path later on
	if _, hasMountpoint := properties["mountpoint"]; !hasMountpoint {
		properties["mountpoint"] = path.Join(Host.Mountpoint, name)
	}

	if _, hasHostname := properties["zettajail:jail:host.hostname"]; !hasHostname {
		properties["zettajail:jail:host.hostname"] = path.Base(name)
	}

	// Expand default console log
	switch properties["zettajail:jail:exec.consolelog"] {
	case "true":
		properties["zettajail:jail:exec.consolelog"] = properties["mountpoint"] + ".log"
	case "false":
		delete(properties, "zettajail:jail:exec.consolelog")
	}

	properties["zettajail:jail"] = "on"
	return properties
}

func CreateJail(name string, properties map[string]string) (Jail, error) {
	properties = newJailProperties(name, properties)

	ds, err := zfs.CreateFilesystem(path.Join(Host.Name, name), properties)
	if err != nil {
		return ZeroJail, err
	}
	return Jail{Dataset{ds}}, Host.WriteJailConf()
}

func CloneJail(snapshot, name string, properties map[string]string) (Jail, error) {
	// FIXME: base properties off snapshot's properties, at least for zettajail:*
	properties = newJailProperties(name, properties)
	snap, err := zfs.GetDataset(path.Join(Host.Name, snapshot))
	if err != nil {
		return ZeroJail, err
	}
	ds, err := snap.Clone(path.Join(Host.Name, name), properties)
	if err != nil {
		return ZeroJail, err
	}
	return Jail{Dataset{ds}}, Host.WriteJailConf()
}

func (j Jail) String() string {
	return j.Name[len(Host.Name)+1:]
}

func (j Jail) Jid() int {
	return Host.Jid(j.String())
}

func (j Jail) IsActive() bool {
	return j.Jid() > 0
}

func (j Jail) Status() error {
	if j.IsActive() {
		log.Printf("%v is active (%d).\n", j, j.Jid())
	} else {
		log.Printf("%v is not active.\n", j)
	}
	return nil
}

func (j Jail) RunJail(op string) error {
	return RunCommand("jail", "-v", op, j.String())
}

func (j Jail) RunJexec(user string, jcmd []string) error {
	if len(jcmd) == 0 {
		jcmd = []string{"login", "-f", "root"}
	}

	args := []string{}
	if user != "" {
		args = append(args, "-U", user)
	}
	args = append(args, j.String())
	args = append(args, jcmd...)

	return RunCommand("jexec", args...)
}

func (j Jail) WriteConfigTo(w io.Writer) error {
	return jailConfTmpl.Execute(w, j)
}

func (j Jail) SetProperties(properties map[string]string) error {
	if err := j.Dataset.SetProperties(properties); err != nil {
		return err
	}
	return Host.WriteJailConf()
}
