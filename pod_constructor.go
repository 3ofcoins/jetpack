package main

import "encoding/json"
import "flag"
import "fmt"
import "io/ioutil"
import "os"
import "strings"

import "code.google.com/p/go-uuid/uuid"
import "github.com/appc/spec/schema"
import "github.com/appc/spec/schema/types"

type podFlag struct {
	v *schema.PodManifest
}

func (cf podFlag) String() string {
	return "PATH"
}

func (cf podFlag) Set(val string) error {
	if data, err := ioutil.ReadFile(val); err != nil {
		return err
	} else {
		if err := json.Unmarshal(data, cf.v); err != nil {
			return err
		}
	}
	return nil
}

type volumesFlag struct{ v *[]types.Volume }

func (vf volumesFlag) String() string {
	return "NAME,kind=host|empty[,source=PATH][,readOnly=true]"
}

func (vf volumesFlag) Set(val string) error {
	if v, err := types.VolumeFromString(val); err != nil {
		return err
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
		return err
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
		return err
	}

	if err := mnt.MountPoint.Set(splut[1%len(splut)]); err != nil {
		return err
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
	if newUUID, err := types.NewUUID(uuid.NewRandom().String()); err != nil {
		// CAN'T HAPPEN
		panic(err)
	} else {
		pm.UUID = *newUUID
	}

	fl.Var(podFlag{pm}, "f", "Load JSON with (partial or full) pod manifest")
	fl.Var(volumesFlag{&pm.Volumes}, "v", "Add volume")
	fl.Var(annotationsFlag{&pm.Annotations}, "a", "Add annotation")
	// TODO: isolatorsFlag

	if err := fl.Parse(args); err != nil {
		return nil, err
	}

	for args = fl.Args(); len(args) > 0; args = fl.Args() {
		if rapp, err := getRuntimeApp(args[0]); err != nil {
			return nil, err
		} else {
			fl = runtimeAppFlagSet(rapp)
			if err := fl.Parse(args[1:]); err != nil {
				return nil, err
			}
			pm.Apps = append(pm.Apps, *rapp)
		}
	}

	return pm, nil
}
