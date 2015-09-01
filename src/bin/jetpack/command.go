package main

import (
	"crypto/sha512"
	stderrors "errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"unicode"

	"code.google.com/p/go-uuid/uuid"

	"github.com/appc/spec/discovery"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/juju/errors"

	"lib/acutil"
	"lib/jetpack"
)

var ErrUsage = stderrors.New("Invalid usage")

type CommandHandler func([]string) error

type Command struct {
	Usage, Synopsis string
	handler         CommandHandler
	flag            *flag.FlagSet
}

var Commands = make(map[string]*Command)

func AddCommand(usage, synopsis string, handler CommandHandler, flHandler func(*flag.FlagSet)) *Command {
	space := strings.IndexFunc(usage, unicode.IsSpace)
	if space < 0 {
		space = len(usage)
	}
	name := usage[:space]

	cmd := &Command{
		Usage:    usage,
		Synopsis: synopsis,
		handler:  handler,
	}

	if flHandler != nil {
		cmd.flag = flag.NewFlagSet(name, flag.ExitOnError)
		flHandler(cmd.flag)
	}

	Commands[name] = cmd
	return cmd
}

func (cmd *Command) String() string {
	if cmd.Synopsis != "" {
		return fmt.Sprintf("%v -- %v", cmd.Usage, cmd.Synopsis)
	}
	return cmd.Usage
}

func (cmd *Command) Help() {
	fmt.Fprintf(os.Stderr, "%v\n\nUsage: %v %v\n",
		cmd.Synopsis, AppName, cmd.Usage)
	if cmd.flag != nil {
		fmt.Fprintln(os.Stderr, "Options:")
		cmd.flag.PrintDefaults()
	}
}

func (cmd *Command) Run(args []string) error {
	if cmd.flag != nil {
		cmd.flag.Parse(args)
		args = cmd.flag.Args()
	}
	err := cmd.handler(args)
	if err == ErrUsage {
		return fmt.Errorf("Usage: %v %v", AppName, cmd.Usage)
	}
	return err
}

func cmdWrap(fn func()) CommandHandler {
	return func([]string) error { fn(); return nil }
}

func cmdWrapErr(fn func() error) CommandHandler {
	return func([]string) error { return errors.Trace(fn()) }
}

func cmdWrapImage(cmd func(*jetpack.Image, []string) error, localOnly bool) func([]string) error {
	return func(args []string) error {
		if len(args) == 0 {
			return ErrUsage
		}
		if img, err := getImage(args[0], localOnly); err != nil {
			return errors.Trace(err)
		} else {
			return errors.Trace(cmd(img, args[1:]))
		}
	}
}

func cmdWrapImage0(cmd func(*jetpack.Image) error, localOnly bool) func([]string) error {
	return cmdWrapImage(func(img *jetpack.Image, args []string) error {
		if len(args) > 0 {
			return ErrUsage
		}
		return cmd(img)
	}, localOnly)
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

func cmdWrapPodMaybeApp(cmd func(*jetpack.Pod, *jetpack.App, []string) error) func([]string) error {
	return func(args []string) error {
		if len(args) == 0 {
			return ErrUsage
		}
		pieces := strings.SplitN(args[0], ":", 2)
		if pod, err := getPod(args[0]); err != nil {
			return errors.Trace(err)
		} else {
			if len(pieces) == 1 {
				return cmd(pod, nil, args[1:])
			} else if appName, err := types.NewACName(pieces[1]); err != nil {
				return errors.Trace(err)
			} else if app := pod.App(*appName); app == nil {
				return jetpack.ErrNotFound
			} else {
				return cmd(pod, app, args)
			}
		}
	}
}

func cmdWrapApp(cmd func(*jetpack.App, []string) error) func([]string) error {
	return cmdWrapPodMaybeApp(func(pod *jetpack.Pod, app *jetpack.App, args []string) error {
		if app == nil {
			if len(pod.Manifest.Apps) == 1 {
				app = pod.Apps()[0]
			} else {
				return errors.New("You need to specify app name for a multi-app pod")
			}
		}
		return cmd(app, args)
	})
}

func cmdWrapApp0(cmd func(*jetpack.App) error) func([]string) error {
	return cmdWrapApp(
		func(app *jetpack.App, args []string) error {
			if len(args) > 0 {
				return ErrUsage
			}
			return cmd(app)
		})
}

func cmdWrapPodPrepare0(cmd func(*jetpack.Pod) error) func([]string) error {
	return func(args []string) error {
		if pod, err := getOrPreparePod(args); err != nil {
			return errors.Trace(err)
		} else {
			return cmd(pod)
		}
	}
}

func parseImageName(name string) (types.ACIdentifier, types.Labels, error) {
	app, err := discovery.NewAppFromString(name)
	if err != nil {
		return "", nil, errors.Trace(err)
	}

	if app.Labels["os"] == "" {
		app.Labels["os"] = runtime.GOOS
	}
	if app.Labels["arch"] == "" {
		app.Labels["arch"] = runtime.GOARCH
	}

	labels, err := types.LabelsFromMap(app.Labels)
	if err != nil {
		return "", nil, errors.Trace(err)
	}

	return app.Name, labels, nil
}

const hashSize = sha512.Size*2 + len("sha512-")

func getImage(name string, localOnly bool) (*jetpack.Image, error) {
	if h, err := types.NewHash(name); err == nil {
		if len(name) < hashSize {
			// Short hash. Iterate over images, return first prefix match.
			// FIXME: what about multiple matches?
			name = strings.ToLower(name)
			if imgs, err := Host.Images(); err != nil {
				return nil, errors.Trace(err)
			} else {
				for _, img := range imgs {
					if strings.HasPrefix(img.Hash.String(), name) {
						return img, nil
					}
				}
				return nil, jetpack.ErrNotFound
			}
		}
		return Host.GetImage(*h, "", nil)
	}
	if name, labels, err := parseImageName(name); err != nil {
		return nil, errors.Trace(err)
	} else if localOnly {
		return Host.GetLocalImage(types.Hash{}, name, labels)
	} else {
		return Host.GetImage(types.Hash{}, name, labels)
	}
}

func getPod(name string) (*jetpack.Pod, error) {
	if id := uuid.Parse(name); id != nil {
		// Pod UUID
		return Host.GetPod(id)
	}
	// TODO: pod name
	return nil, ErrUsage
}

func getPodManifest(args []string) (*schema.PodManifest, error) {
	if err := acutil.ParseApps(thePodManifest, args); err != nil {
		return nil, errors.Trace(err)
	} else if acutil.IsPodManifestEmpty(thePodManifest) {
		return nil, ErrUsage
	} else if pm, err := Host.ReifyPodManifest(thePodManifest); err != nil {
		return nil, errors.Trace(err)
	} else {
		return pm, nil
	}
}

func getOrPreparePod(args []string) (*jetpack.Pod, error) {
	switch len(args) {
	case 0:
		return nil, ErrUsage
	case 1:
		if id := uuid.Parse(args[0]); id != nil {
			// Pod UUID
			return Host.GetPod(id)
		}
		fallthrough
	default:
		if pm, err := getPodManifest(args); err != nil {
			return nil, err
		} else if pod, err := Host.CreatePod(pm); err != nil {
			return nil, err
		} else {
			if SaveID != "" {
				if err := ioutil.WriteFile(SaveID, []byte(pod.UUID.String()), 0644); err != nil {
					return nil, err
				}
			}
			return pod, nil
		}
	}
}
