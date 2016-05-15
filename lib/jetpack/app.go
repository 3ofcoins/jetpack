package jetpack

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/appc/spec/schema/types"
	"github.com/juju/errors"

	"github.com/3ofcoins/jetpack/lib/passwd"
	"github.com/3ofcoins/jetpack/lib/run"
)

type App struct {
	Name   types.ACName
	Pod    *Pod
	app    *types.App
	cmd    *run.Cmd
	killed bool

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
			if app.killed {
				return errors.New("CAN'T HAPPEN: app killed, and Stage2 succeeded")
			}
		case "post-stop":
			defer func(exec []string) {
				// TODO: log
				if !app.killed {
					if err := app.Stage2(stdin, stdout, stderr, "0", "0", "", exec...); err != nil {
						if re != nil {
							re = errors.Trace(err)
						} // else? log?
					}
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
	return errors.Trace(app.Stage2(os.Stdin, os.Stdout, os.Stderr, "0", "0", "", "/usr/bin/login", "-p", "-f", username))
}

// IsRunning returns true if the app currently executes a stage2 command.
func (app *App) IsRunning() bool {
	return app.cmd != nil
}

func (app *App) Kill() error {
	if app.cmd != nil && app.cmd.Cmd.Process != nil {
		return app.cmd.Cmd.Process.Kill()
	}
	// Killing an app that's not alive is a nop
	return nil
}

func (app *App) Stage2(stdin io.Reader, stdout, stderr io.Writer, user, group string, cwd string, exec ...string) error {
	if app.IsRunning() {
		// One Jetpack process won't need to run multiple commands in the
		// same app at the same time. It's either sequential
		// hook-exec-hook, or an individual command, but not both in the
		// same binary. This assumption may change in the future.
		// FIXME: race condition between this place and setting app.cmd
		return errors.New("A stage2 command is already running for this app")
	}
	app.killed = false

	if strings.HasPrefix(user, "/") || strings.HasPrefix(group, "/") {
		return errors.New("Path-based user/group not supported yet, sorry")
	}

	if cwd == "" {
		cwd = app.app.WorkingDirectory
	}

	addSupplementaryGIDs := false

	if user == "" {
		user = app.app.User
		addSupplementaryGIDs = true
	}

	if group == "" {
		group = app.app.Group
		addSupplementaryGIDs = true
	}

	// Ensure jail is created
	jid := app.Pod.ensureJid()

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

	gids := strconv.Itoa(pwent.Gid)
	if addSupplementaryGIDs && len(app.app.SupplementaryGIDs) > 0 {
		gidsArr := make([]string, len(app.app.SupplementaryGIDs)+1)
		gidsArr[0] = gids
		for i, gid := range app.app.SupplementaryGIDs {
			gidsArr[i+1] = strconv.Itoa(gid)
		}
		gids = strings.Join(gidsArr, ",")
	}

	stage2 := filepath.Join(Config().MustGetString("path.libexec"), "stage2")
	args := []string{
		fmt.Sprintf("%d:%d:%s:%s:%s", jid, pwent.Uid, gids, app.Name, cwd),
		"AC_METADATA_URL=" + mds,
		"USER=" + pwent.Username,
		"LOGNAME=" + pwent.Username,
		"HOME=" + pwent.Home,
		"SHELL=" + pwent.Shell,
	}
	// TODO: move TERM= here if stdin (or stdout?) is a terminal
	args = append(args, app.env()...)
	args = append(args, exec...)
	app.cmd = run.Command(stage2, args...)
	app.cmd.Cmd.Stdin = stdin
	app.cmd.Cmd.Stdout = stdout
	app.cmd.Cmd.Stderr = stderr
	defer func() { app.cmd = nil }()

	return app.cmd.Run()
}
