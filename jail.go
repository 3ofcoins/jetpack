package zettajail

import "io"
import "log"
import "text/template"

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

type Jail struct {
	Dataset
	Host *Host
}

var ZeroJail = Jail{}

func (j Jail) String() string {
	return j.Name[len(j.Host.Name)+1:]
}

func (j Jail) Jid() int {
	return j.Host.Jid(j.String())
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
	return j.Host.WriteJailConf()
}
