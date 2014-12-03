package main

import "io"
import "log"
import "text/template"

var jailConfTmpl *template.Template

func init() {
	tmpl, err := template.New("jail.conf").Parse(`"{{.JailName}}" {
  host.hostname = "{{.JailName}}";
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
	return Jail{GetDataset(Root.Name + "/" + name)}
}

func (j Jail) JailName() string {
	return j.Name[len(Root.Name)+1:]
}

func (j Jail) Jid() int {
	return Root.Jails()[j.JailName()]
}

func (j Jail) IsActive() bool {
	return j.Jid() > 0
}

func (j Jail) String() string {
	rv := j.JailName()
	if j.IsActive() {
		rv = "*" + rv
	}
	return rv
}

func (j Jail) WriteConfigTo(w io.Writer) error {
	return jailConfTmpl.Execute(w, j)
}
