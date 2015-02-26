package main

import "flag"
import "fmt"
import "os"
import "path/filepath"
import "sort"
import "strconv"

import "github.com/juju/errors"

import "./jetpack"
import "./run"

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
  test                                    Run integration tests
  image list [QUERY]                      List images
  image import ARCHIVE [MANIFEST]         Import image from an archive
  image IMAGE build [OPTIONS] COMMAND...  Build new image from an existing one
                    -dir=.                Location on build directory on host
                    -cp=PATH...           Copy additional files from host
  image IMAGE show                        Display image details
  image IMAGE export [PATH]               Export image to an AMI file
                                          Output to stdout if no PATH given
  image IMAGE destroy                     Destroy image
  container list                          List containers
  container create IMAGE                  Create new container from image
  container CONTAINER show                Display container details
  container CONTAINER run                 Run container's application
  container CONTAINER console [USER]      Open console inside the container
  container CONTAINER ps|top|killall [OPTIONS...]
                                          Manage container's processes
  container CONTAINER kill                Kill running container
  container CONTAINER destroy             Destroy container
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
          CONTAINER  Has to be an UUID for now
Helpful Aliases:
  i ... -- image ...
  c ... -- container ...
  image, images -- image list
  container, containers -- container list
  image build|show|export|destroy IMAGE ... -- image IMAGE build|show|... ...
`)
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
	case "test":
		die(run.Command(filepath.Join(jetpack.LibexecPath, "test.integration"),
			append(args, "dataset="+Host.Dataset.Name)...).Run())
	case "images":
		command = "image"
		args = append([]string{"list"}, args...)
		fallthrough
	case "image", "i":
		switch command, args := subcommand("list", args); command {
		case "import":
			var archive, manifest string
			switch len(args) {
			case 2:
				manifest = args[1]
				fallthrough
			case 1:
				archive = args[0]
			default:
				die(errors.New("Usage: import ARCHIVE_URI [MANIFEST_URI]"))
			}
			image, err := Host.ImportImage(archive, manifest)
			die(err)
			show(image)
		case "list":
			images := Host.Images()

			if len(images) == 0 {
				show("No images")
			} else {
				sort.Sort(images)
				show(images.Table()) // FIXME: Table() doesn't really belong in images
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
				var buildDir string

				fs := flag.NewFlagSet("build", flag.ExitOnError)
				fs.Var(&copyFiles, "cp", "")
				fs.StringVar(&buildDir, "dir", ".", "")
				die(fs.Parse(args))

				newImage, err := img.Build(buildDir, copyFiles, fs.Args())
				die(err)
				show(newImage)
			case "show":
				show(img)
			case "export":
				path := "-"
				if len(args) > 0 {
					path = args[0]
				}
				if hash, err := img.SaveAMI(path, 0644); err != nil {
					die(err)
				} else {
					fmt.Fprintln(os.Stderr, hash)
				}
			case "destroy":
				die(img.Destroy())
			default:
				die(errors.Errorf("Unknown command %#v", command))
			}
		}
	case "containers":
		command = "container"
		args = append([]string{"list"}, args...)
		fallthrough
	case "container", "c":
		switch command, args := subcommand("list", args); command {
		case "create":
			container, err := Host.CloneContainer(image(args[0]))
			die(err)
			show(container)
		case "list":
			if containers := Host.Containers(); len(containers) == 0 {
				show("No containers")
			} else {
				sort.Sort(containers)
				show(containers.Table()) // FIXME: Table() doesn't really belong in containers
			}
		case "show", "run", "ps", "top", "killall", "kill", "destroy":
			// be nice to people who prefer to type UUID after command
			command, args[0] = args[0], command
			fallthrough
		default:
			container, err := Host.FindContainer(command)
			if err == jetpack.ErrNotFound {
				die(errors.Errorf("No such container: %#v", command))
			}
			die(err)
			switch command, args := subcommand("show", args); command {
			case "show":
				show(container)
			case "run":
				die(container.Run(nil))
			case "console":
				user := "root"
				if len(args) != 0 {
					user = args[0]
				}
				die(container.Run(jetpack.ConsoleApp(user)))
			case "ps", "top", "killall":
				jid := container.Jid()
				if jid == 0 {
					die(errors.New("Container is not running"))
				}

				flag := "-J"
				if command == "killall" {
					flag = "-j"
				}

				die(run.Command(command, append([]string{flag, strconv.Itoa(jid)}, args...)...).Run())
			case "kill":
				die(container.Kill())
			case "destroy":
				die(container.Destroy())
			default:
				die(errors.Errorf("Unknown command %#v", command))
			}
		}
	default:
		die(errors.Errorf("Unknown command %#v", command))
	}
}
