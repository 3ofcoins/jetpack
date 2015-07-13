package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"code.google.com/p/go-uuid/uuid"

	"github.com/appc/spec/schema/types"
	"github.com/juju/errors"

	"lib/jetpack"
	"lib/run"
)

func init() {
	AddCommand("manifest POD", "Show pod manifest", cmdWrapPod0(cmdPodManifest), nil)
	AddCommand("destroy POD", "Destroy a pod", cmdWrapPod0(cmdDestroyPod), nil)
	AddCommand("kill POD", "Kill a running pod", cmdWrapPod0(cmdKillPod), nil)
	AddCommand("ps POD [ARGS...]", "Show pod's process list (ps)", cmdWrapPod(cmdPodCmd("/bin/ps", "-J")), nil)
	AddCommand("top POD [ARGS...]", "Show pod's process list (top)", cmdWrapPod(cmdPodCmd("/usr/bin/top", "-J")), nil)
	AddCommand("killall POD [ARGS...]", "Kill pod's processes", cmdWrapPod(cmdPodCmd("/usr/bin/killall", "-j")), nil)
	AddCommand("console POD[:APP]", "Open a console in pod environment", cmdWrapMustApp0(cmdConsole), flConsole)
}

func getPod(name string) (*jetpack.Pod, error) {
	if id := uuid.Parse(name); id != nil {
		// Pod UUID
		return Host.GetPod(id)
	}
	// TODO: pod name
	return nil, ErrUsage
}

func cmdWrapPod(cmd func(*jetpack.Pod, []string) error) func([]string) error {
	return func(args []string) error {
		if len(args) == 0 {
			return ErrUsage
		}
		if pod, err := getPod(args[0]); err != nil {
			return errors.Trace(err)
		} else {
			return errors.Trace(cmd(pod, args[1:]))
		}
	}
}

func cmdWrapPod0(cmd func(*jetpack.Pod) error) func([]string) error {
	return cmdWrapPod(func(pod *jetpack.Pod, args []string) error {
		if len(args) > 0 {
			return ErrUsage
		}
		return cmd(pod)
	})
}

func cmdWrapApp(cmd func(*jetpack.Pod, types.ACName, []string) error) func([]string) error {
	return func(args []string) error {
		if len(args) == 0 {
			return ErrUsage
		}
		pieces := strings.SplitN(args[0], ":", 2)
		if pod, err := getPod(pieces[0]); err != nil {
			return errors.Trace(err)
		} else if len(pieces) == 1 {
			if len(pod.Manifest.Apps) == 1 {
				return errors.Trace(cmd(pod, pod.Manifest.Apps[0].Name, args[1:]))
			} else {
				return errors.Trace(cmd(pod, types.ACName(""), args[1:]))
			}
		} else if appName, err := types.NewACName(pieces[1]); err != nil {
			return errors.Trace(err)
		} else if rta := pod.Manifest.Apps.Get(*appName); rta == nil {
			return errors.Errorf("Pod %v has no app %v", pod.UUID, appName)
		} else {
			return errors.Trace(cmd(pod, *appName, args[1:]))
		}
	}
}

func cmdWrapApp0(cmd func(*jetpack.Pod, types.ACName) error) func([]string) error {
	return cmdWrapApp(func(pod *jetpack.Pod, appName types.ACName, args []string) error {
		if len(args) > 0 {
			return ErrUsage
		}
		return cmd(pod, appName)
	})
}

func cmdWrapMustApp(cmd func(*jetpack.Pod, types.ACName, []string) error) func([]string) error {
	return cmdWrapApp(func(pod *jetpack.Pod, appName types.ACName, args []string) error {
		if appName.Empty() {
			return errors.Errorf("No app name provided, and pod %v has multiple apps", pod.UUID)
		}
		return cmd(pod, appName, args)
	})
}

func cmdWrapMustApp0(cmd func(*jetpack.Pod, types.ACName) error) func([]string) error {
	return cmdWrapMustApp(func(pod *jetpack.Pod, appName types.ACName, args []string) error {
		if len(args) > 0 {
			return ErrUsage
		}
		return cmd(pod, appName)
	})
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

func cmdConsole(pod *jetpack.Pod, appName types.ACName) error {
	return errors.Trace(pod.Console(appName, flConsoleUsername))
}
