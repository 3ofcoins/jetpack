package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/appc/spec/schema/types"
	"github.com/juju/errors"

	"github.com/3ofcoins/jetpack/lib/jetpack"
	"github.com/3ofcoins/jetpack/lib/run"
)

func init() {
	AddCommand("show-image IMAGE", "Show image info", cmdWrapImage0(cmdShowImage, true), nil)
	AddCommand("image-manifest IMAGE", "Show image manifest", cmdWrapImage0(cmdImageManifest, true), nil)
	AddCommand("destroy-image IMAGE", "Destroy an image", cmdWrapImage0(cmdDestroyImage, true), nil)
	AddCommand("export IMAGE [FILE]", "Export image to an ACI file", cmdWrapImage(cmdExportImage, true), flExport)
	AddCommand("build IMAGE COMMAND ARGS...", "Build a new image", cmdWrapImage(cmdBuild, false), flBuild)
}

var flExportFlat bool

func flExport(fl *flag.FlagSet) {
	fl.BoolVar(&flExportFlat, "flat", false, "Export flattened image without dependencies")
}

func cmdImageManifest(img *jetpack.Image) error {
	if jsonManifest, err := json.MarshalIndent(img.Manifest, "", "  "); err != nil {
		return errors.Trace(err)
	} else {
		_, err := fmt.Println(string(jsonManifest))
		return errors.Trace(err)
	}
}

func cmdShowImage(img *jetpack.Image) error {
	output := fmt.Sprintf("ID\t%v\nName\t%v\nTimestamp\t%v\n",
		img.Hash,
		img,
		img.Timestamp.Format(time.RFC3339),
	)

	if len(img.Manifest.Dependencies) > 0 {
		output += "Dependencies"
		for _, dep := range img.Manifest.Dependencies {
			if dimg, err := Host.GetImage(*dep.ImageID, "", nil); err != nil {
				// FIXME: export GetImageDependency?
				return errors.Trace(err)
			} else {
				output += fmt.Sprintf("\t%v %v\n", types.ShortHash(dimg.Hash.String()), dimg)
			}
		}
	}

	if app := img.Manifest.App; app != nil {
		output += "App\t\n" + appDetails(app)
	}

	tw := tabwriter.NewWriter(os.Stdout, 2, 8, 2, ' ', 0)
	fmt.Fprint(tw, output)
	return tw.Flush()
}

func cmdDestroyImage(img *jetpack.Image) error {
	return errors.Trace(img.Destroy())
}

func cmdExportImage(img *jetpack.Image, args []string) error {
	var output *os.File

	if len(args) == 0 || args[0] == "-" {
		output = os.Stdout
	} else {
		if of, err := os.Create(args[0]); err != nil {
			return errors.Trace(err)
		} else {
			output = of
			defer output.Close()
		}
	}

	if flExportFlat {
		if hash, err := img.WriteFlatACI(output); err != nil {
			return errors.Trace(err)
		} else {
			fmt.Println(hash)
			return nil
		}
	} else {
		if aci, err := os.Open(img.Path("aci")); err != nil {
			return errors.Trace(err)
		} else {
			defer aci.Close()
			_, err = io.Copy(output, aci)
			return errors.Trace(err)
		}
	}
}

func appDetails(app *types.App) string {
	u := app.User
	if u == "" {
		u = "0"
	}

	g := app.Group
	if g == "" {
		g = "0"
	}

	rv := fmt.Sprintf("  Exec\t[%v:%v] %v\n",
		u, g, run.ShellEscape(app.Exec...))

	if len(app.Ports) != 0 {
		ports := make([]string, len(app.Ports))
		nameLen := 0
		for _, port := range app.Ports {
			if nameLen < len(port.Name) {
				nameLen = len(port.Name)
			}
		}
		format := fmt.Sprintf("%%%dv:%%v/%%v", nameLen)
		for i, port := range app.Ports {
			ports[i] = fmt.Sprintf(format, port.Name, port.Protocol, port.Port)
			if port.Count > 1 {
				ports[i] += fmt.Sprintf("+%d", port.Count)
			}
		}
		rv += "  Ports\t" + strings.Join(ports, "\n\t") + "\n"
	}

	if len(app.MountPoints) != 0 {
		mps := make([]string, len(app.MountPoints))
		nameLen := 0
		for _, mp := range app.MountPoints {
			if len(mp.Name) > nameLen {
				nameLen = len(mp.Name)
			}
		}
		format := fmt.Sprintf("%%%dv:%%v", nameLen)
		for i, mp := range app.MountPoints {
			mps[i] = fmt.Sprintf(format, mp.Name, mp.Path)
			if mp.ReadOnly {
				mps[i] += " (ro)"
			}
		}

		rv += "  Mount Points\t" + strings.Join(mps, "\n\t") + "\n"
	}

	return rv
}

var flBuildDir string
var flBuildCp sliceFlag

func flBuild(fl *flag.FlagSet) {
	SaveIDFlag(fl)
	fl.Var(&flBuildCp, "cp", "Copy additional files to the build dir")
	fl.StringVar(&flBuildDir, "dir", ".", "Source build directory")
}

func cmdBuild(img *jetpack.Image, args []string) error {
	if nimg, err := img.Build(flBuildDir, flBuildCp, args); err != nil {
		return errors.Trace(err)
	} else {
		if err := cmdShowImage(nimg); err != nil {
			return errors.Trace(err)
		}
		if SaveID != "" {
			return errors.Trace(ioutil.WriteFile(SaveID,
				[]byte(nimg.Hash.String()+"\n"), 0644))
		}
		return nil
	}
}
