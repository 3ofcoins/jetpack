package main

import "flag"
import "fmt"
import "os"
import "path/filepath"
import "sort"
import "strconv"

import "github.com/juju/errors"

import "github.com/3ofcoins/jetpack/jetpack"
import "github.com/3ofcoins/jetpack/run"

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

	if len(args) == 0 {
		help = true
	} else if args[0] == "help" {
		help = true
		args = args[1:]
	}

	if help {
		if len(args) > 0 {
			fmt.Fprintln(os.Stderr, "FIXME: should show help for", args)
		}
		fmt.Fprintln(os.Stderr, "Usage:")
		usecase := func(args ...interface{}) {
			args = append([]interface{}{"  ", os.Args[0]}, args...)
			fmt.Fprintln(os.Stderr, args...)
		}
		usecase("help -- show this help screen")
		usecase("init -- initialize host")
		usecase("info -- show global information")
		usecase("test -- run integration tests")
		usecase("image import ARCHIVE_URI_OR_PATH [MANIFEST_URI_OR_PATH] -- import image from ACI or rootfs tarball + manifest JSON")
		usecase("image list [QUERY] -- list images")
		usecase("image IMAGE build [-cp=PATH...] [-dir=PATH] COMMAND... -- build new image from existing one (PATH defaults to current directory)")
		usecase("image IMAGE show -- show image details")
		usecase("image IMAGE destroy -- destroy image")
		usecase("container create IMAGE -- create container from image")
		usecase("container list -- list containers")
		usecase("container CONTAINER show -- show container details")
		usecase("container CONTAINER run -- run container")
		usecase("container CONTAINER console [USER] -- open console inside the container")
		usecase("container CONTAINER ps|top|killall [OPTIONS...] -- manage container's processes")
		usecase("container CONTAINER kill -- kill running container")
		usecase("container CONTAINER destroy -- destroy container")
		fmt.Fprintln(os.Stderr, "Global flags:")
		flag.PrintDefaults()
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
	}

	if Host.Dataset == nil {
		die(errors.New("Host is not initialized"))
	}

	switch command {
	case "info":
		show(Host)
	case "test":
		die(run.Command(filepath.Join(jetpack.LibexecPath, "test.integration"),
			append(args[1:], "dataset="+Host.Dataset.Name)...).Run())
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
			image, err := Host.Images.Import(archive, manifest)
			die(err)
			show(image)
		case "list":
			images, err := Host.Images.All()
			die(err)

			if len(images) == 0 {
				show("No images")
			} else {
				sort.Sort(images)
				show(images.Table()) // FIXME: Table() doesn't really belong in images
			}
		case "build", "show", "destroy":
			// be nice to people who prefer to type UUID after command
			command, args[0] = args[0], command
			fallthrough
		default:
			image, err := Host.Images.Get(command)
			if err == jetpack.ErrNotFound {
				die(errors.Errorf("No such image: %#v", command))
			}
			die(err)

			switch command, args := subcommand("show", args); command {
			case "build":
				var copyFiles sliceFlag
				var buildDir string

				fs := flag.NewFlagSet("build", flag.ExitOnError)
				fs.Var(&copyFiles, "cp", "")
				fs.StringVar(&buildDir, "dir", ".", "")
				die(fs.Parse(args))

				newImage, err := image.Build(buildDir, copyFiles, fs.Args())
				die(err)
				show(newImage)
			case "show":
				show(image)
			case "destroy":
				die(image.Destroy())
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
			image, err := Host.Images.Get(args[0])
			if err == jetpack.ErrNotFound {
				die(errors.Errorf("No such image: %#v", command))
			}
			die(err)

			container, err := Host.Containers.Clone(image)
			die(err)
			show(container)
		case "list":
			if containers, err := Host.Containers.All(); err != nil {
				die(err)
			} else {
				if len(containers) == 0 {
					show("No containers")
				} else {
					sort.Sort(containers)
					show(containers.Table()) // FIXME: Table() doesn't really belong in containers
				}
			}
		case "show", "run", "ps", "top", "killall", "kill", "destroy":
			// be nice to people who prefer to type UUID after command
			command, args[0] = args[0], command
			fallthrough
		default:
			container, err := Host.Containers.Get(command)
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
