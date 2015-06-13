package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"strings"

	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/juju/errors"
)

type podFlag struct {
	v *schema.PodManifest
}

func (cf podFlag) String() string {
	return "PATH"
}

func (cf podFlag) Set(val string) error {
	if data, err := ioutil.ReadFile(val); err != nil {
		return errors.Trace(err)
	} else {
		if err := json.Unmarshal(data, cf.v); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

type volumesFlag struct{ v *[]types.Volume }

func (vf volumesFlag) String() string {
	return "[-]NAME[:PATH] | NAME,kind=host|empty[,source=PATH][,readOnly=true]"
}

func (vf volumesFlag) Set(val string) error {
	// Transform sanely formatted string to Rocket/appc format. Should
	// we even support it?
	if !strings.ContainsRune(val, ',') {
		if pieces := strings.SplitN(val, ":", 2); len(pieces) == 1 {
			val += ",kind=empty"
		} else {
			val = fmt.Sprintf("%v,kind=host,source=%v", pieces[0], url.QueryEscape(pieces[1]))
		}
	}
	if val[0] == '-' {
		val = val[1:] + ",readOnly=true"
	}
	if v, err := types.VolumeFromString(val); err != nil {
		return errors.Trace(err)
	} else {
		for _, vol := range *vf.v {
			if vol.Name == v.Name {
				return fmt.Errorf("Volume %v already defined: %v", v.Name, vol)
			}
		}
		*vf.v = append(*vf.v, *v)
		return nil
	}
}

type annotationsFlag struct{ v *types.Annotations }

// TODO: isolatorsFlag

func (af annotationsFlag) String() string {
	return "NAME=VALUE"
}

func (af annotationsFlag) Set(val string) error {
	splut := strings.SplitN(val, "=", 2)
	if len(splut) < 2 {
		return fmt.Errorf("Invalid annotation %#v", val)
	}
	if name, err := types.NewACName(splut[0]); err != nil {
		return errors.Trace(err)
	} else {
		af.v.Set(*name, splut[1])
		return nil
	}
}

type mountsFlag struct{ v *[]schema.Mount }

func (mf mountsFlag) String() string {
	return "VOLUME[:MOUNTPOINT]"
}

func (mf mountsFlag) Set(val string) error {
	mnt := schema.Mount{}
	splut := strings.SplitN(val, ":", 2)
	if len(splut) < 2 {
		splut = append(splut, splut[0])
	}

	if err := mnt.Volume.Set(splut[0]); err != nil {
		return errors.Trace(err)
	}

	if err := mnt.MountPoint.Set(splut[1%len(splut)]); err != nil {
		return errors.Trace(err)
	}

	*mf.v = append(*mf.v, mnt)
	return nil
}

func runtimeAppFlagSet(ra *schema.RuntimeApp) *flag.FlagSet {
	fl := flag.NewFlagSet("", flag.ContinueOnError)
	fl.Usage = func() {}
	fl.Var(&ra.Name, "n", "App name")
	fl.Var(annotationsFlag{&ra.Annotations}, "a", "Add annotation")
	fl.Var(mountsFlag{&ra.Mounts}, "m", "Mount volume")
	// TODO: app override
	return fl
}

func constructPodHelp(fl *flag.FlagSet) func() {
	return func() {
		fmt.Fprintln(os.Stderr, `Usage: jetpack pod create [FLAGS] IMAGE [IMAGE FLAGS] [IMAGE [IMAGE FLAGS] ...]

Flags: `)
		fl.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nImage flags:")
		runtimeAppFlagSet(&schema.RuntimeApp{Name: types.ACName("NAME")}).PrintDefaults()
	}
}

func ConstructPod(args []string, fl *flag.FlagSet, getRuntimeApp func(string) (*schema.RuntimeApp, error)) (*schema.PodManifest, error) {
	if fl == nil {
		fl = flag.NewFlagSet("ConstructPod", flag.ContinueOnError)
	}
	fl.Usage = constructPodHelp(fl)

	pm := schema.BlankPodManifest()

	fl.Var(podFlag{pm}, "f", "Load JSON with (partial or full) pod manifest")
	fl.Var(volumesFlag{&pm.Volumes}, "v", "Add volume")
	fl.Var(annotationsFlag{&pm.Annotations}, "a", "Add annotation")
	// TODO: isolatorsFlag

	if err := fl.Parse(args); err != nil {
		return nil, errors.Trace(err)
	}

	for args = fl.Args(); len(args) > 0; args = fl.Args() {
		if rapp, err := getRuntimeApp(args[0]); err != nil {
			return nil, errors.Trace(err)
		} else {
			fl = runtimeAppFlagSet(rapp)
			if err := fl.Parse(args[1:]); err != nil {
				return nil, errors.Trace(err)
			}
			pm.Apps = append(pm.Apps, *rapp)
		}
	}

	// Automatic volumes
	for i, app := range pm.Apps {
		// TODO: appc/spec PR unifying RuntimeImage & Dependency
		dep := types.Dependency{Labels: app.Image.Labels}
		if app.Image.Name != nil {
			dep.App = *app.Image.Name
		}

		if !app.Image.ID.Empty() {
			dep.ImageID = &app.Image.ID
		}

		img, err := Host.GetImageDependency(&dep)
		if err != nil {
			return nil, errors.Trace(err)
		}

		pm.Apps[i].Image.ID = *img.Hash

		if img.Manifest.App == nil {
			continue
		}

	mntpnts:
		for _, mntpnt := range img.Manifest.App.MountPoints {
			var mnt *schema.Mount
			for _, mntc := range app.Mounts {
				if mntc.MountPoint == mntpnt.Name {
					mnt = &mntc
				}
			}
			if mnt == nil {
				fmt.Printf("INFO: mount for %v:%v not found, inserting mount for volume %v\n", app.Name, mntpnt.Name, mntpnt.Name)
				mnt = &schema.Mount{MountPoint: mntpnt.Name, Volume: mntpnt.Name}
				pm.Apps[i].Mounts = append(pm.Apps[i].Mounts, *mnt)
			}
			for _, vol := range pm.Volumes {
				if vol.Name == mnt.Volume {
					continue mntpnts
				}
			}
			fmt.Printf("INFO: volume %v not found, inserting empty volume\n", mnt.Volume)
			pm.Volumes = append(pm.Volumes, types.Volume{Name: mnt.Volume, Kind: "empty"})
		}
	}

	return pm, nil
}
