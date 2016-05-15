package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/appc/spec/schema/types"
	"github.com/juju/errors"
	"github.com/pborman/uuid"

	"github.com/3ofcoins/jetpack/lib/jetpack"
	"github.com/3ofcoins/jetpack/lib/run"
)

func init() {
	AddCommand("prepare ...", "Prepare a pod", cmdPrepare, flPrepare)
	AddCommand("run ...", "Run a pod", cmdWrapPodPrepare0(cmdRun), flRun)
	AddCommand("manifest POD", "Show pod manifest", cmdWrapPod0(cmdPodManifest), nil)
	AddCommand("destroy POD", "Destroy a pod", cmdWrapPod0(cmdDestroyPod), nil)
	AddCommand("kill POD", "Kill a running pod", cmdWrapPod0(cmdKillPod), nil)
	AddCommand("ps POD [ARGS...]", "Show pod's process list (ps)", cmdWrapPod(cmdPodCmd("/bin/ps", "-J")), nil)
	AddCommand("top POD [ARGS...]", "Show pod's process list (top)", cmdWrapPod(cmdPodCmd("/usr/bin/top", "-J")), nil)
	AddCommand("killall POD [ARGS...]", "Kill pod's processes", cmdWrapPod(cmdPodCmd("/usr/bin/killall", "-j")), nil)
	AddCommand("console POD[:APP]", "Open a console in app", cmdWrapApp0(cmdConsole), flConsole)
	AddCommand("exec POD[:APP] COMMAND...", "Run a command in app", cmdWrapApp(cmdExec), nil)
	AddCommand("cp [FLAGS] ARGS...", "Copy files to/from pod (use POD:[APP|@VOL]:PATH for pod paths)", cmdCp, nil)
}

var flDryRun bool

func flPrepare(fl *flag.FlagSet) {
	SaveIDFlag(fl)
	flPodManifest(fl)
	fl.BoolVar(&flDryRun, "n", false, "Dry run (don't actually create pod, just show reified manifest)")
}

func cmdPrepare(args []string) error {
	if pm, err := getPodManifest(args); err != nil {
		return errors.Trace(err)
	} else if flDryRun {
		if jb, err := json.MarshalIndent(pm, "", "  "); err != nil {
			return errors.Trace(err)
		} else {
			// TODO: is it a good place?
			fmt.Println(string(jb))
			return nil
		}
	} else if pod, err := Host.CreatePod(pm); err != nil {
		return errors.Trace(err)
	} else {
		if SaveID != "" {
			if err := ioutil.WriteFile(SaveID, []byte(pod.UUID.String()), 0644); err != nil {
				return errors.Trace(err)
			}
		}
		if !Quiet {
			// TODO: show pod
			fmt.Println(pod.UUID)
		}
		return nil
	}
}

var flAppName types.ACName
var flDestroy, flTerminal bool

func flRun(fl *flag.FlagSet) {
	flPodManifest(fl)
	SaveIDFlag(fl)
	fl.Var(&flAppName, "app", "Specify app to run for a multi-app pod")
	fl.BoolVar(&flDestroy, "destroy", false, "Destroy pod when done")
	fl.BoolVar(&flTerminal, "t", false, "Attach app to the terminal (single-app containers only)")
}

func cmdRun(pod *jetpack.Pod) (erv error) {
	if flAppName.Empty() && flTerminal {
		if len(pod.Manifest.Apps) != 1 {
			return errors.New("Multi-app pod! Please use -app=NAME to choose")
		} else {
			flAppName = pod.Manifest.Apps[0].Name
		}
	}
	if flDestroy {
		defer func() {
			if err := pod.Destroy(); err != nil {
				if erv == nil {
					erv = err
				} else {
					// TODO: UI
					fmt.Fprintln(os.Stderr, "ERROR destroying pod:", err)
				}
			}
		}()
	}
	if !flAppName.Empty() {
		// Run one app on terminal
		if app := pod.App(flAppName); app == nil {
			return jetpack.ErrNotFound
		} else {
			return errors.Trace(app.Run(os.Stdin, os.Stdout, os.Stderr))
		}
	} else {
		return errors.Trace(pod.Run())
	}
}

func cmdPodManifest(pod *jetpack.Pod) error {
	if jsonManifest, err := json.MarshalIndent(pod.Manifest, "", "  "); err != nil {
		return errors.Trace(err)
	} else {
		_, err := fmt.Println(string(jsonManifest))
		return errors.Trace(err)
	}
}

func cmdDestroyPod(pod *jetpack.Pod) error {
	return errors.Trace(pod.Destroy())
}

func cmdKillPod(pod *jetpack.Pod) error {
	return errors.Trace(pod.Kill())
}

func cmdPodCmd(cmd string, baseArgs ...string) func(*jetpack.Pod, []string) error {
	return func(pod *jetpack.Pod, args []string) error {
		jid := pod.Jid()
		if jid == 0 {
			return errors.New("Pod is not running")
		} else {
			return errors.Trace(run.Command(cmd, append(append(baseArgs, strconv.Itoa(jid)), args...)...).Run())
		}
	}
}

var flConsoleUsername string

func flConsole(fl *flag.FlagSet) {
	fl.StringVar(&flConsoleUsername, "u", "root", "Username to run console as")
}

func cmdConsole(app *jetpack.App) error {
	return errors.Trace(app.Console(flConsoleUsername))
}

func cmdExec(app *jetpack.App, args []string) error {
	return errors.Trace(app.Stage2(os.Stdin, os.Stdout, os.Stderr, "", "", "", args...))
}

// Arguments for cp to leave unprocessed (switches and local paths):
//  - switches (starting with '-')
//  - absolute paths (starting with '/')
//  - current and parent dir (literal "." and "..")
//  - relative paths (starting with "./" or "../")
var cmdCpSwitchOrLocalFileRegexp = regexp.MustCompile(`^(?:[-/]|\.\.?(?:$|/))`)

func cmdCp(args []string) error {
	// caches
	pods := map[string]*jetpack.Pod{}

	for i, arg := range args {
		// Leave switches and local paths alone
		if cmdCpSwitchOrLocalFileRegexp.MatchString(arg) {
			continue
		}
		pieces := strings.SplitN(arg, ":", 3)
		if len(pieces) != 3 {
			return ErrUsage
		}

		podUUID := uuid.Parse(pieces[0])
		if podUUID == nil {
			return ErrUsage
		}

		pod := pods[podUUID.String()]
		if pod == nil {
			pod_, err := Host.GetPod(podUUID)
			if err != nil {
				return err
			}
			pod = pod_
			pods[podUUID.String()] = pod_
		}

		isVolume := false
		if pieces[1] == "" {
			if len(pod.Manifest.Apps) != 1 {
				return ErrUsage
			}
			pieces[1] = pod.Manifest.Apps[0].Name.String()
		} else if pieces[1][0] == '@' {
			isVolume = true
			pieces[1] = pieces[1][1:]
		}

		// TODO: sanity check on paths?
		if isVolume {
			args[i] = pod.Path("rootfs", "vol", pieces[1], pieces[2])
		} else {
			args[i] = pod.Path("rootfs", "app", pieces[1], "rootfs", pieces[2])
		}
	}

	// fmt.Printf("cp %#v\n", args)
	return errors.Trace(run.Command("/bin/cp", args...).Run())
}
