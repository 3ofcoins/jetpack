package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/juju/errors"

	"lib/jetpack"
	"lib/run"
)

var Host *jetpack.Host

func die(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, errors.ErrorStack(err))
		os.Exit(1)
	}
}

func show(obj ...interface{}) {
	die(Show("", obj...))
}

func subcommand(def string, args []string) (string, []string) {
	if len(args) == 0 {
		return def, args
	}
	return args[0], args[1:]
}

func image(name string) *jetpack.Image {
	img, err := Host.FindImage(name)
	if err == jetpack.ErrNotFound {
		die(errors.Errorf("No such image: %#v", name))
	}
	die(err)
	return img
}

func getRuntimeApp(name string) (*schema.RuntimeApp, error) {
	if img, err := Host.FindImage(name); err != nil {
		return nil, err
	} else {
		rta := img.RuntimeApp()
		return &rta, nil
	}
}

func main() {
	configPath := jetpack.DefaultConfigPath
	help := false

	if cfg := os.Getenv("JETPACK_CONF"); cfg != "" {
		configPath = cfg
	}

	flag.StringVar(&configPath, "config", configPath, "Configuration file")
	flag.BoolVar(&help, "h", false, "Show help")
	flag.BoolVar(&help, "help", false, "Show help")

	flag.Parse()
	args := flag.Args()

	if help || len(args) == 0 || args[0] == "help" {
		fmt.Fprintf(os.Stderr, `Usage: %s [OPTIONS] COMMAND...
Options:
  -config=PATH  Configuration file (%s)
  -help, -h     Display this help screen
Commands:
  help                                    Display this help screen
  init                                    Initialize host
  info                                    Show global information
  trust [-l|-list]                        List trusted ACI signign keys
  trust [-prefix PREFIX] [-root] [KEY]    Trust ACI signing key
  trust -d FINGERPRINT                    Untrust ACI signing key
  fetch NAME...                           Fetch ACI
  image list [QUERY]                      List images
  image IMAGE build [OPTIONS] COMMAND...  Build new image from an existing one
                    -dir=.                Location on build directory on host
                    -cp=PATH...           Copy additional files from host
  image IMAGE show                        Display image details
  image IMAGE export [PATH]               Export image to an ACI file
                                          Output to stdout if no PATH given
  image IMAGE destroy                     Destroy image
  pod list                                List pods
  pod create [FLAGS] IMAGE [IMAGE FLAGS] [IMAGE [IMAGE FLAGS] ...]
                                          Create new pod from image
             -help                        Show detailed help
  pod POD show                            Display pod details
  pod POD run [APP]                       Run pod's application
  pod POD console [APP]                   Open console inside the pod
  pod POD ps|top|killall [OPTIONS...]
                                          Manage pod's processes
  pod POD kill                            Kill running pod
  pod POD destroy                         Destroy pod
  mds [FLAGS]                             Run metadata server as a daemon
Needs Explanation:
  ARCHIVE, MANIFEST  May be filesystem paths or URLs.
            cp=PATH  This option can be given multiple times
              QUERY  Is an expression that looks like this:
                      - NAME[,LABEL=VALUE[,LABEL=VALUE[,...]]]
                      - NAME:VERSION (alias for NAME:version=VERSION)
              IMAGE  Can be:
                      - an UUID (XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXX),
                      - a checksum (sha512-...), or
                      - a QUERY (which can't be ambiguous).
          POD  Has to be an UUID for now
Helpful Aliases:
  i|img ... -- image ...
  p ... -- pod ...
  image, images -- image list
  pod, pods -- pod list
  image build|show|export|destroy IMAGE ... -- image IMAGE build|show|... ...
`,
			filepath.Base(os.Args[0]), configPath)
		return
	}

	command := args[0]
	args = args[1:]

	if host, err := jetpack.NewHost(configPath); err != nil {
		die(err)
	} else {
		Host = host
	}

	if command == "init" {
		// Init is special: it doesn't need an initialized host
		die(Host.Initialize())
		show(Host)
		return
	}

	if Host.Dataset == nil {
		die(errors.New("Host is not initialized"))
	}

	switch command {
	case "info":
		show(Host)
	case "trust":
		die(runTrust(args))
	case "fetch":
		die(runFetch(args))
	case "images":
		command = "image"
		args = append([]string{"list"}, args...)
		fallthrough
	case "image", "img", "i":
		switch command, args := subcommand("list", args); command {
		case "list":
			var machineFriendly, showHash, idOnly bool
			fl := flag.NewFlagSet("image list", flag.ExitOnError)
			fl.BoolVar(&machineFriendly, "H", false, "Machine-friendly output")
			fl.BoolVar(&showHash, "hash", false, "Show image hash instead of UUID")
			fl.BoolVar(&idOnly, "q", false, "Show only ID")
			die(fl.Parse(args))

			images := Host.Images()

			if idOnly {
				for _, img := range images {
					if showHash {
						fmt.Println(img.Hash)
					} else {
						fmt.Println(img.UUID)
					}
				}
			} else if len(images) == 0 {
				if !machineFriendly {
					show("No images")
				}
			} else {
				lines := make([]string, len(images))
				for i, img := range images {
					labels := make([]string, len(img.Manifest.Labels))
					for j, label := range img.Manifest.Labels {
						labels[j] = fmt.Sprintf("%v=%#v", label.Name, label.Value)
					}
					sort.Strings(labels)
					first := img.UUID.String()
					if showHash {
						first = img.Hash.String()
					}
					lines[i] = fmt.Sprintf("%v\t%v\t%v",
						first,
						img.Manifest.Name,
						strings.Join(labels, ","))
				}
				sort.Strings(lines)
				output := strings.Join(lines, "\n")

				if machineFriendly {
					fmt.Println(output)
				} else {
					first := "UUID"
					if showHash {
						first = "HASH"
					}
					w := tabwriter.NewWriter(os.Stdout, 2, 8, 2, ' ', 0)
					fmt.Fprintf(w, "%v\tNAME\tLABELS\n%v\n", first, output)
					die(w.Flush())
				}
			}
		case "build", "show", "export", "destroy":
			// be nice to people who prefer to type UUID after command
			command, args[0] = args[0], command
			fallthrough
		default:
			img := image(command)

			switch command, args := subcommand("show", args); command {
			case "build":
				var copyFiles sliceFlag
				var buildDir, saveId string

				fl := flag.NewFlagSet("build", flag.ExitOnError)
				fl.Var(&copyFiles, "cp", "")
				fl.StringVar(&buildDir, "dir", ".", "")
				fl.StringVar(&saveId, "saveid", "", "Save ID of built image to a file")
				die(fl.Parse(args))

				newImage, err := img.Build(buildDir, copyFiles, fl.Args())
				die(err)
				show(newImage)
				if saveId != "" {
					die(ioutil.WriteFile(saveId, []byte(newImage.Hash.String()+"\n"), 0644))
				}
			case "show":
				show(img)
			case "export":
				var isFlat bool
				fl := flag.NewFlagSet("export", flag.ExitOnError)
				fl.BoolVar(&isFlat, "flat", false, "Export flattened image without dependencies")
				die(fl.Parse(args))
				args = fl.Args()

				if isFlat {
					if len(args) == 0 {
						args[0] = "-"
					}
					hash, err := img.SaveFlatACI(args[0], 0644)
					if hash != nil {
						fmt.Println(hash)
					}
					die(err)
				} else {
					aci, err := os.Open(img.Path("aci"))
					die(err)
					defer aci.Close()

					var output *os.File
					if len(args) == 0 || args[0] == "-" {
						output = os.Stdout
					} else {
						output, err = os.Create(args[0])
						die(err)
						defer output.Close()
					}

					_, err = io.Copy(output, aci)
					die(err)
				}
			case "destroy":
				die(img.Destroy())
			default:
				die(errors.Errorf("Unknown command %#v", command))
			}
		}
	case "pods":
		command = "pod"
		args = append([]string{"list"}, args...)
		fallthrough
	case "pod", "p":
		switch command, args := subcommand("list", args); command {
		case "create":
			var dryRun, doRun, doDestroy bool
			var saveId string
			fl := flag.NewFlagSet("jetpack pod create", flag.ContinueOnError)
			fl.BoolVar(&dryRun, "n", false, "Dry run (don't actually create pod, just show manifest)")
			fl.BoolVar(&doRun, "run", false, "Run pod immediately")
			fl.BoolVar(&doDestroy, "destroy", false, "Destroy pod after running (meaningless without -run)")
			fl.StringVar(&saveId, "saveid", "", "Save pod UUID to file")

			if pm, err := ConstructPod(args, fl, getRuntimeApp); err == flag.ErrHelp {
				// It's all right. Help has been shown.
			} else if err != nil {
				panic(err)
			} else if dryRun {
				if jb, err := json.MarshalIndent(pm, "", "  "); err != nil {
					panic(err)
				} else {
					fmt.Println(string(jb))
				}
			} else {
				pod, err := Host.CreatePod(pm)
				die(err)
				if saveId != "" {
					die(ioutil.WriteFile(saveId, []byte(pod.UUID.String()), 0644))
				}
				if doRun {
					if len(pod.Manifest.Apps) > 1 {
						die(errors.New("Pod has multiple apps, cannot run"))
					}
					err := pod.RunApp(pod.Manifest.Apps[0].Name)
					if doDestroy {
						err1 := pod.Destroy()
						if err == nil {
							err = err1
						}
					}
					die(err)
				} else {
					show(pod)
				}
			}
		case "list":
			var machineFriendly, idOnly bool
			fl := flag.NewFlagSet("pod list", flag.ExitOnError)
			fl.BoolVar(&machineFriendly, "H", false, "Machine-friendly output")
			fl.BoolVar(&idOnly, "q", false, "Show only ID")
			fl.Parse(args)

			pods := Host.Pods()

			if idOnly {
				for _, pod := range pods {
					fmt.Println(pod.UUID)
				}
			} else if len(pods) == 0 {
				if !machineFriendly {
					show("No pods")
				}
			} else {
				lines := make([]string, len(pods))
				for i, pod := range pods {
					apps := make([]string, len(pod.Manifest.Apps))
					for j, app := range pod.Manifest.Apps {
						apps[j] = app.Name.String()
					}
					ipAddress, _ := pod.Manifest.Annotations.Get("ip-address")
					lines[i] = fmt.Sprintf("%v\t%v\t%v\t%v",
						pod.UUID,
						pod.Status().String(),
						ipAddress,
						strings.Join(apps, " "))
				}
				sort.Strings(lines)
				output := strings.Join(lines, "\n")

				if machineFriendly {
					fmt.Println(output)
				} else {
					w := tabwriter.NewWriter(os.Stdout, 2, 8, 2, ' ', 0)
					fmt.Fprintf(w, "UUID\tSTATUS\tIP\tAPPS\n%v\n", output)
					die(w.Flush())
				}
			}
		case "show", "run", "ps", "top", "killall", "kill", "destroy":
			// be nice to people who prefer to type UUID after command
			command, args[0] = args[0], command
			fallthrough
		default:
			pod, err := Host.FindPod(command)
			if err == jetpack.ErrNotFound {
				die(errors.Errorf("No such pod: %#v", command))
			}
			die(err)
			switch command, args := subcommand("show", args); command {
			case "show":
				show(pod)
			case "run":
				switch len(args) {
				case 0:
					if len(pod.Manifest.Apps) > 1 {
						die(errors.New("Pod has multiple apps, you need to specify one"))
					}
					die(pod.RunApp(pod.Manifest.Apps[0].Name))
				case 1:
					die(pod.RunApp(types.ACName(args[0])))
				default:
					die(errors.New("Command `run' takes at most one argument"))
				}
			case "console":
				switch len(args) {
				case 0:
					if len(pod.Manifest.Apps) > 1 {
						die(errors.New("Pod has multiple apps, you need to specify one"))
					}
					die(pod.Console(pod.Manifest.Apps[0].Name, "root"))
				case 1:
					die(pod.Console(types.ACName(args[0]), "root"))
				default:
					die(errors.New("Command `console' takes at most one argument"))
				}
			case "ps", "top", "killall":
				jid := pod.Jid()
				if jid == 0 {
					die(errors.New("Pod is not running"))
				}

				flag := "-J"
				if command == "killall" {
					flag = "-j"
				}

				die(run.Command(command, append([]string{flag, strconv.Itoa(jid)}, args...)...).Run())
			case "kill":
				die(pod.Kill())
			case "destroy":
				die(pod.Destroy())
			default:
				die(errors.Errorf("Unknown command %#v", command))
			}
		}
	case "mds":
		die(runMds(args))
	default:
		die(errors.Errorf("Unknown command %#v", command))
	}
}
