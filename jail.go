package zettajail

import "io"
import "log"
import "os"
import "os/exec"
import "path/filepath"
import "strconv"
import "strings"
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

func NewJail(host *Host, ds Dataset) *Jail {
	return &Jail{ds, host}
}

func (j *Jail) String() string {
	return j.Name[len(j.Host.Name)+1:]
}

func (j *Jail) Jid() int {
	cmd := exec.Command("jls", "-j", j.String(), "jid")
	out, err := cmd.Output()
	switch err.(type) {
	case nil:
		// Jail found
		jid, err := strconv.Atoi(strings.TrimSpace(string(out)))
		if err != nil {
			panic(err)
		}
		return jid
	case *exec.ExitError:
		// Jail not found (or so we assume)
		return 0
	default:
		// Other error
		panic(err)
	}
}

func (j *Jail) IsActive() bool {
	return j.Jid() > 0
}

func (j *Jail) Status() error {
	if j.IsActive() {
		log.Printf("%v is active (%d).\n", j, j.Jid())
	} else {
		log.Printf("%v is not active.\n", j)
	}
	return nil
}

func (j *Jail) Basedir() string {
	return filepath.Dir(j.Mountpoint)
}

func (j *Jail) Path(elem ...string) string {
	return filepath.Join(append([]string{j.Basedir()}, elem...)...)
}

func (j *Jail) RunJail(op string) error {
	if err := j.WriteConfig(); err != nil {
		return err
	}
	return RunCommand("jail", "-f", j.Path("jail.conf"), "-v", op, j.String())
}

func (j *Jail) RunJexec(user string, jcmd []string) error {
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

func (j *Jail) WriteConfigTo(w io.Writer) error {
	return jailConfTmpl.Execute(w, j)
}

func (j *Jail) WriteConfig() error {
	jc, err := os.OpenFile(j.Path("jail.conf"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer jc.Close()
	return j.WriteConfigTo(jc)
}

func (j *Jail) SetProperties(properties map[string]string) error {
	return j.Dataset.SetProperties(properties)
}
