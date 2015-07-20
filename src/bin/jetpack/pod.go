package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"strconv"

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
	// AddCommand("console POD[:APP]", "Open a console in pod environment", cmdWrapMustApp0(cmdConsole), flConsole)
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
