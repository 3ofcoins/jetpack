package jetpack

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
	//DONE 	tmpl, err := template.New("jail.conf").Parse(`"{{.}}" {
	//DONE   path = "{{.Mountpoint}}";
	//DONE   exec.consolelog = "{{.Mountpoint}}.log";
	//DONE {{ range .JailParameters }}  {{.}};
	//DONE {{ end }}}
	//DONE `)
	//DONE 	jailConfTmpl = tmpl
	//DONE 	if err != nil {
	//DONE 		log.Fatalln(err)
	//DONE 	}
}

type Jail struct {
	Dataset
	Host *Host
}

func NewJail(host *Host, ds Dataset) *Jail {
	//DONE 	return &Jail{ds, host}
	return nil
}

func (j *Jail) String() string {
	//DONE 	return j.Name[len(j.Host.Name)+1:]
	return nil
}

func (j *Jail) Jid() int {
	//DONE 	cmd := exec.Command("jls", "-j", j.String(), "jid")
	//DONE 	out, err := cmd.Output()
	//DONE 	switch err.(type) {
	//DONE 	case nil:
	//DONE 		// Jail found
	//DONE 		jid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	//DONE 		if err != nil {
	//DONE 			panic(err)
	//DONE 		}
	//DONE 		return jid
	//DONE 	case *exec.ExitError:
	//DONE 		// Jail not found (or so we assume)
	//DONE 		return 0
	//DONE 	default:
	//DONE 		// Other error
	//DONE 		panic(err)
	//DONE 	}
	return -1
}

func (j *Jail) IsActive() bool {
	//DONE return j.Jid() > 0
	return nil
}

func (j *Jail) Status() error {
	//IRRELEVANT 	if j.IsActive() {
	//IRRELEVANT 		log.Printf("%v is active (%d).\n", j, j.Jid())
	//IRRELEVANT 	} else {
	//IRRELEVANT 		log.Printf("%v is not active.\n", j)
	//IRRELEVANT 	}
	return nil
}

func (j *Jail) Basedir() string {
	//DONE 	return filepath.Dir(j.Mountpoint)
	return ""
}

func (j *Jail) Path(elem ...string) string {
	//DONE 	return filepath.Join(append([]string{j.Basedir()}, elem...)...)
	return ""
}

func (j *Jail) RunJail(op string) error {
	//DONE 	if err := j.WriteConfig(); err != nil {
	//DONE 		return err
	//DONE 	}
	//DONE 	return RunCommand("jail", "-f", j.Path("jail.conf"), "-v", op, j.String())
	return nil
}

func (j *Jail) RunJexec(user string, jcmd []string) error {
	//DONE 	if len(jcmd) == 0 {
	//DONE 		jcmd = []string{"login", "-f", "root"}
	//DONE 	}
	//DONE
	//DONE 	args := []string{}
	//DONE 	if user != "" {
	//DONE 		args = append(args, "-U", user)
	//DONE 	}
	//DONE 	args = append(args, j.String())
	//DONE 	args = append(args, jcmd...)
	//DONE
	//DONE 	return RunCommand("jexec", args...)
	return nil
}

func (j *Jail) WriteConfigTo(w io.Writer) error {
	//DONE 	return jailConfTmpl.Execute(w, j)
	return nil
}

func (j *Jail) WriteConfig() error {
	//DONE 	jc, err := os.OpenFile(j.Path("jail.conf"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	//DONE 	if err != nil {
	//DONE 		return err
	//DONE 	}
	//DONE 	defer jc.Close()
	//DONE 	return j.WriteConfigTo(jc)
	return nil
}

func (j *Jail) SetProperties(properties map[string]string) error {
	//IRRELEVANT 	return j.Dataset.SetProperties(properties)
	return nil
}
