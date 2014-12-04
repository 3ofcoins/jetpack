package main

import "io"
import "log"
import "os"
import "os/exec"
import "text/template"

var jailConfTmpl *template.Template

func init() {
	tmpl, err := template.New("jail.conf").Parse(`"{{.}}" {
  host.hostname = "{{.}}";
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

func GetJail(name string) Jail {
	return Jail{GetDataset(Host.Name + "/" + name)}
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
	cmd := exec.Command("jail", op, j.String())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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

	log.Println("JEXEC:", args)
	cmd := exec.Command("jexec", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
