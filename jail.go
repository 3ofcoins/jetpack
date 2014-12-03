package main

import "io"
import "log"
import "path"
import "text/template"

var jailConfTmpl *template.Template

func init() {
	tmpl, err := template.New("jail.conf").Parse(`{{.}} {
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
	return Jail{GetDataset(Root.Name + "/" + name)}
}

func (j Jail) String() string {
	return path.Base(j.Name)
}

func (j Jail) WriteConfig(w io.Writer) error {
	return jailConfTmpl.Execute(w, j)
}
