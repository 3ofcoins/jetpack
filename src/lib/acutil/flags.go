package acutil

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/url"
	"path"
	"runtime"
	"strconv"
	"strings"

	"github.com/appc/spec/discovery"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
)

type AnnotationsFlag types.Annotations

func (afl *AnnotationsFlag) String() string {
	vv := make([]string, len(*afl))
	for i, ann := range *afl {
		vv[i] = fmt.Sprintf("%v=%#v", ann.Name, ann.Value)
	}
	return fmt.Sprintf("[%v]", strings.Join(vv, ","))
}

func (afl *AnnotationsFlag) Set(val string) error {
	pieces := strings.SplitN(val, "=", 2)
	if len(pieces) != 2 {
		return errors.New("Annotations must be provided in NAME=VALUE format")
	} else if name, err := types.NewACIdentifier(pieces[0]); err != nil {
		return err
	} else {
		(*types.Annotations)(afl).Set(*name, pieces[1])
		return nil
	}
}

type ExposedPortsFlag []types.ExposedPort

func (epfl *ExposedPortsFlag) String() string {
	vv := make([]string, len(*epfl))
	for i, ep := range *epfl {
		if ep.HostPort == 0 {
			vv[i] = ep.Name.String()
		} else {
			vv[i] = fmt.Sprintf("%v=%v", ep.Name, ep.HostPort)
		}
	}
	return fmt.Sprintf("[%v]", strings.Join(vv, ","))
}

func (epfl *ExposedPortsFlag) Set(val string) error {
	ep := types.ExposedPort{}
	pieces := strings.SplitN(val, "=", 2)
	if name, err := types.NewACName(pieces[0]); err != nil {
		return err
	} else {
		ep.Name = *name
	}
	if len(pieces) == 2 {
		if hp, err := strconv.ParseUint(pieces[1], 10, 0); err != nil {
			return err
		} else {
			ep.HostPort = uint(hp)
		}
	}
	// TODO: check for duplicates? Or do we validate that later (by
	// serializing & reparsing JSON)?
	*epfl = append(*epfl, ep)
	return nil
}

type VolumesFlag []types.Volume

func (vfl *VolumesFlag) String() string {
	return fmt.Sprint(([]types.Volume)(*vfl))
}

func (vfl *VolumesFlag) Set(val string) error {
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
		return err
	} else {
		// TODO: check for duplicates?
		*vfl = append(*vfl, *v)
		return nil
	}
}

type PodManifestJSONFlag schema.PodManifest

func (pmjf *PodManifestJSONFlag) String() string {
	return "[PATH]"
}

func (pmjf *PodManifestJSONFlag) Set(val string) error {
	if bb, err := ioutil.ReadFile(val); err != nil {
		return err
	} else if err := json.Unmarshal(bb, (*schema.PodManifest)(pmjf)); err != nil {
		return err
	} else {
		return nil
	}
}

type MountsFlag []schema.Mount

func (mf *MountsFlag) String() string {
	return fmt.Sprint(*mf)
}

func (mf *MountsFlag) Set(val string) error {
	mnt := schema.Mount{}
	pieces := strings.SplitN(val, ":", 2)
	if name, err := types.NewACName(pieces[0]); err != nil {
		return err
	} else {
		mnt.Volume = *name
	}
	if len(pieces) == 1 {
		mnt.Path = mnt.Volume.String()
	} else {
		mnt.Path = pieces[1]
	}
	*mf = append(*mf, mnt)
	return nil
}

func PodManifestFlags(fl *flag.FlagSet, pm *schema.PodManifest) {
	fl.Var((*PodManifestJSONFlag)(pm), "f", "Read JSON pod manifest file")
	fl.Var((*AnnotationsFlag)(&pm.Annotations), "a", "Add annotation (NAME=VALUE)")
	fl.Var((*ExposedPortsFlag)(&pm.Ports), "p", "Expose port (NAME[=HOST_PORT])")
	fl.Var((*VolumesFlag)(&pm.Volumes), "v", "Define volume")
}

func ParseImageName(name string) (types.ACIdentifier, types.Labels, error) {
	app, err := discovery.NewAppFromString(name)
	if err != nil {
		return "", nil, err
	}

	if app.Labels["os"] == "" {
		app.Labels["os"] = runtime.GOOS
	}
	if app.Labels["arch"] == "" {
		app.Labels["arch"] = runtime.GOARCH
	}

	labels, err := types.LabelsFromMap(app.Labels)
	if err != nil {
		return "", nil, err
	}

	return app.Name, labels, nil
}

func parseApp(args []string) ([]string, *schema.RuntimeApp, error) {
	if len(args) == 0 {
		return nil, nil, nil
	}

	rtapp := schema.RuntimeApp{}

	// Parse first argument (image name)
	if h, err := types.NewHash(args[0]); err == nil {
		rtapp.Image.ID = *h
		rtapp.Name.Set(h.String()) // won't err
	} else if name, labels, err := ParseImageName(args[0]); err == nil {
		rtapp.Image.Name = &name
		rtapp.Name.Set(path.Base(name.String())) // won't err here
		rtapp.Image.Labels = labels
	} else {
		return args, nil, err
	}

	fl := flag.NewFlagSet(args[0], flag.ExitOnError)
	fl.Var(&rtapp.Name, "name", "App name")
	fl.Var((*AnnotationsFlag)(&rtapp.Annotations), "a", "Add annotation (NAME=VALUE)")
	fl.Var((*MountsFlag)(&rtapp.Mounts), "m", "Mount volume (VOLUME[:MOUNTPOINT])")
	// TODO: app override
	fl.Parse(args[1:])
	return fl.Args(), &rtapp, nil
}

func ParseApps(pm *schema.PodManifest, args []string) error {
	for len(args) > 0 {
		if rest, rtapp, err := parseApp(args); err != nil {
			return err
		} else {
			pm.Apps = append(pm.Apps, *rtapp)
			args = rest
		}
	}
	return nil
}
