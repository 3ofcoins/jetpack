package jetpack

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/appc/spec/schema/types"
	"github.com/juju/errors"

	"lib/passwd"
	"lib/run"
)

type App struct {
	Name types.ACName
	Pod  *Pod
	app  *types.App

	// cache
	_env []string
}

func (app *App) Path(elem ...string) string {
	return app.Pod.Path(append(
		[]string{"rootfs", "app", app.Name.String(), "rootfs"},
		elem...)...)
}

func (app *App) env() []string {
	if app._env == nil {
		env := make([]string, len(app.app.Environment))
		hasPath := false
		hasTerm := false
		for i, ev := range app.app.Environment {
			env[i] = ev.Name + "=" + ev.Value
			if ev.Name == "PATH" {
				hasPath = true
			}
			if ev.Name == "TERM" {
				hasTerm = true
			}
		}
		if !hasPath {
			env = append(env, "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
		}

		if !hasTerm {
			// TODO: TERM= only if we're attached to a terminal
			term := os.Getenv("TERM")
			if term == "" {
				term = "vt100"
			}
			env = append(env, "TERM="+term)
		}

		app._env = env
	}
	return app._env
}

func (app *App) Run(stdin io.Reader, stdout, stderr io.Writer) (re error) {
	if _, err := app.Pod.Host.CheckMDS(); err != nil {
		return errors.Trace(err)
	}

	for _, eh := range app.app.EventHandlers {
		switch eh.Name {
		case "pre-start":
			// TODO: log
			if err := app.Stage2(stdin, stdout, stderr, "0", "0", "", eh.Exec...); err != nil {
				return errors.Trace(err)
			}
		case "post-stop":
			defer func(exec []string) {
				// TODO: log
				if err := app.Stage2(stdin, stdout, stderr, "0", "0", "", exec...); err != nil {
					if re != nil {
						re = errors.Trace(err)
					} // else? log?
				}
			}(eh.Exec)
		default:
			return errors.Errorf("Unrecognized eventHandler: %v", eh.Name)
		}
	}

	return errors.Trace(app.Stage2(stdin, stdout, stderr, "", "", "", app.app.Exec...))
}

func (app *App) Console(username string) error {
	if username == "" {
		username = "root"
	}
	return errors.Trace(app.Stage2(os.Stdin, os.Stdout, os.Stderr, "0", "0", "", "/usr/bin/login", "-fp", username))
}

func (app *App) Stage2(stdin io.Reader, stdout, stderr io.Writer, user, group, cwd string, exec ...string) error {
	if strings.HasPrefix(user, "/") || strings.HasPrefix(group, "/") {
		return errors.New("Path-based user/group not supported yet, sorry")
	}

	if cwd == "" {
		cwd = app.app.WorkingDirectory
	}

	if user == "" {
		user = app.app.User
	}

	if group == "" {
		group = app.app.Group
	}

	// Ensure jail is created
	jid := app.Pod.Jid()
	if jid == 0 {
		if err := errors.Trace(app.Pod.runJail("-c")); err != nil {
			return errors.Trace(err)
		}
		jid = app.Pod.Jid()
		if jid == 0 {
			panic("Could not start jail")
		}
	}

	mds, err := app.Pod.MetadataURL()
	if err != nil {
		return errors.Trace(err)
	}

	pwf, err := passwd.ReadPasswd(app.Path("etc", "passwd"))
	if err != nil {
		return errors.Trace(err)
	}

	pwent := pwf.Find(user)
	if pwent == nil {
		return errors.Errorf("Cannot find user: %#v", user)
	}

	if group != "" {
		grf, err := passwd.ReadGroup(app.Path("etc", "group"))
		if err != nil {
			return errors.Trace(err)
		}
		pwent.Gid = grf.FindGid(group)
		if pwent.Gid < 0 {
			return errors.Errorf("Cannot find group: %#v", group)
		}
	}

	if cwd == "" {
		cwd = "/"
	}

	stage2 := filepath.Join(Config().MustGetString("path.libexec"), "stage2")
	args := []string{
		fmt.Sprintf("%d:%d:%d:%s:%s", jid, pwent.Uid, pwent.Gid, app.Name, cwd),
		"AC_METADATA_URL=" + mds,
		"USER=" + pwent.Username,
		"LOGNAME=" + pwent.Username,
		"HOME=" + pwent.Home,
		"SHELL=" + pwent.Shell,
	}
	// TODO: move TERM= here if stdin (or stdout?) is a terminal
	args = append(args, app.env()...)
	args = append(args, exec...)
	cmd := run.Command(stage2, args...)
	cmd.Cmd.Stdin = stdin
	cmd.Cmd.Stdout = stdout
	cmd.Cmd.Stderr = stderr
	return cmd.Run()
}
