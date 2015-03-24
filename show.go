package main

import "bytes"
import "fmt"
import "sort"
import "strconv"
import "strings"
import "text/tabwriter"

import "github.com/appc/spec/schema"
import "github.com/appc/spec/schema/types"
import "github.com/juju/errors"
import "github.com/magiconair/properties"

import "./jetpack"
import "./zfs"

func showExec(strs []string) string {
	for i, str := range strs {
		strs[i] = strconv.QuoteToASCII(str)
	}
	return strings.Join(strs, " ")
}

func labelsTbl(labels types.Labels) [][]string {
	if len(labels) == 0 {
		return nil
	}

	tbl := make([][]string, len(labels))
	for i, label := range labels {
		tbl[i] = []string{string(label.Name), label.Value}
	}
	return tbl
}

type sectionObj struct {
	name string
	objs []interface{}
}

func Section(name string, objs ...interface{}) sectionObj {
	return sectionObj{name, objs}
}

func ShowSection(prefix, name string, obj ...interface{}) error {
	return errors.Trace(Show(prefix, Section(name, obj...)))
}

func Showf(prefix string, format string, objs ...interface{}) error {
	return Show(prefix, fmt.Sprintf(format, objs...))
}

func Show(prefix string, objs ...interface{}) error {
	var obj interface{}
	switch len(objs) {
	case 0:
		return nil
	case 1:
		obj = objs[0]
	default:
		obj = objs
	}

	switch obj.(type) {
	case nil:
		return nil

	case []interface{}:
		for _, obj := range obj.([]interface{}) {
			if err := Show(prefix, obj); err != nil {
				return errors.Trace(err)
			}
		}

	case [][]string:
		if data := obj.([][]string); len(data) > 0 {
			buf := new(bytes.Buffer)
			w := tabwriter.NewWriter(buf, 2, 8, 2, ' ', 0)

			lines := make([]string, len(data))
			for i, ln := range data {
				lines[i] = strings.Join(ln, "\t")
			}

			if _, err := w.Write([]byte(strings.Join(lines, "\n"))); err != nil {
				return errors.Trace(err)
			}

			if err := w.Flush(); err != nil {
				return errors.Trace(err)
			}

			return errors.Trace(Show(prefix, buf.String()))
		}

	case *properties.Properties:

		if p := obj.(*properties.Properties); p.Len() > 0 {
			tbl := make([][]string, p.Len())
			for i, k := range p.Keys() {
				v, _ := p.Get(k)
				tbl[i] = []string{k, v}
			}
			return errors.Trace(ShowSection(prefix, "Configuration:", tbl))
		}
		return errors.Trace(Show(prefix, "Configuration: (empty)"))

	case sectionObj:
		sec := obj.(sectionObj)
		if err := Show(prefix, sec.name); err != nil {
			return errors.Trace(err)
		}
		return errors.Trace(Show(prefix+"  ", sec.objs...))

	case *jetpack.Host:
		h := obj.(*jetpack.Host)
		isdev := ""
		if jetpack.IsDevelopment {
			isdev = " (development)"
		}
		Showf(prefix, "JetPack %v (%v), compiled on %v%v", jetpack.Version, jetpack.Revision, jetpack.BuildTimestamp, isdev)
		return Show(prefix, h.Dataset, h.Properties)

	case *zfs.Dataset:
		ds := obj.(*zfs.Dataset)
		tbl := [][]string{[]string{"Mountpoint", ds.Mountpoint}}
		if ds.Origin != "" {
			tbl = append(tbl, []string{"Origin", ds.Origin})
		}

		return ShowSection(prefix, fmt.Sprintf("ZFS Dataset %v", ds.Name), tbl)

	case *jetpack.Image:
		if img := obj.(*jetpack.Image); img != nil {
			metadata := [][]string{}
			if img.Hash != nil {
				metadata = append(metadata, []string{"Hash", img.Hash.String()})
			}
			if img.Origin != "" {
				metadata = append(metadata, []string{"Origin", img.Origin})
			}
			metadata = append(metadata, []string{"Timestamp", img.Timestamp.String()})

			items := []interface{}{metadata, img.Manifest}

			if cc := img.Pods(); len(cc) > 0 {
				sort.Sort(cc)
				items = append(items, Section("Pods:", cc.Table()))
			}

			return errors.Trace(ShowSection(prefix, fmt.Sprintf("Image %v", img.UUID), items...))
		}

	case *jetpack.Pod:
		c := obj.(*jetpack.Pod)
		items := []interface{}{c.Manifest}
		for _, app := range c.Manifest.Apps {
			if img, err := c.Host.GetImageByHash(app.Image.ID); err != nil {
				return errors.Trace(err)
			} else {
				items = append(items, img)
			}
		}

		return errors.Trace(ShowSection(prefix, fmt.Sprintf("Pod %v", c.Manifest.UUID), items...))

	case schema.ImageManifest:
		manifest := obj.(schema.ImageManifest)
		return ShowSection(prefix, fmt.Sprintf("Manifest %v", manifest.Name),
			manifest.Labels,
			manifest.App,
			manifest.Annotations,
			manifest.Dependencies)

	case schema.PodManifest:
		manifest := obj.(schema.PodManifest)
		return ShowSection(prefix, "Manifest",
			manifest.Apps,
			manifest.Volumes,
			manifest.Isolators,
			types.Annotations(manifest.Annotations))

	case *types.App:
		if app := obj.(*types.App); app != nil {
			metaTbl := [][]string{
				[]string{"Exec", showExec(app.Exec)},
				[]string{"User", app.User},
				[]string{"Group", app.Group},
			}

			envTbl := make([][]string, len(app.Environment))
			for i, ev := range app.Environment {
				envTbl[i] = []string{ev.Name, ev.Value}
			}

			return ShowSection(prefix, "App",
				metaTbl,
				Section("Environment:", envTbl),
				app.EventHandlers,
				app.MountPoints,
				app.Ports,
				app.Isolators)
		}

	case schema.AppList:
		if apps := obj.(schema.AppList); len(apps) > 0 {
			appsI := make([]interface{}, len(apps))
			for i, app := range apps {
				appsI[i] = app
			}
			return ShowSection(prefix, "Apps", appsI)
		}

	case schema.RuntimeApp:
		app := obj.(schema.RuntimeApp)
		return ShowSection(prefix, string(app.Name),
			app.Image.ID, types.Annotations(app.Annotations))

	case []types.MountPoint:
		if mntpts := obj.([]types.MountPoint); len(mntpts) > 0 {
			tbl := make([][]string, len(mntpts))
			for i, mntpt := range mntpts {
				tbl[i] = []string{string(mntpt.Name), mntpt.Path}
				if mntpt.ReadOnly {
					tbl[i] = append(tbl[i], "ro")
				}
			}
			return ShowSection(prefix, "Mount Points", tbl)
		}

	case []types.Volume:
		if vols := obj.([]types.Volume); len(vols) > 0 {
			tbl := make([]interface{}, len(vols))
			for i, vol := range vols {
				hdr := string(vol.Name)
				if vol.ReadOnly != nil && *vol.ReadOnly {
					hdr += " (ro)"
				}

				details := [][]string{[]string{"Kind", vol.Kind}}
				if vol.Source != "" {
					details = append(details, []string{"Source", vol.Source})
				}
				tbl[i] = Section(hdr, details)
			}
			return ShowSection(prefix, "Volumes", tbl)
		}

	case []types.EventHandler:
		if handlers := obj.([]types.EventHandler); len(handlers) > 0 {
			tbl := make([][]string, len(handlers))
			for i, handler := range handlers {
				tbl[i] = []string{handler.Name, showExec(handler.Exec)}
			}
			return ShowSection(prefix, "Event Handlers", tbl)
		}

	case []types.Port:
		if ports := obj.([]types.Port); len(ports) > 0 {
			tbl := make([][]string, len(ports))
			for i, port := range ports {
				tbl[i] = []string{string(port.Name), port.Protocol, fmt.Sprintf("%d", port.Port)}
				if port.SocketActivated {
					tbl[i] = append(tbl[i], "(socket activated)")
				}
			}
			return ShowSection(prefix, "Ports", tbl)
		}

	case []types.Isolator:
		if isolators := obj.([]types.Isolator); len(isolators) > 0 {
			tbl := make([][]string, len(isolators))
			for i, isolator := range isolators {
				tbl[i] = []string{string(isolator.Name), string(*isolator.ValueRaw)}
			}
			return ShowSection(prefix, "Isolators", tbl)
		}

	case types.Labels:
		if labels := obj.(types.Labels); len(labels) > 0 {
			return ShowSection(prefix, "Labels", labelsTbl(labels))
		}

	case types.Annotations:
		if annotations := obj.(types.Annotations); len(annotations) > 0 {
			tbl := make([][]string, 0, len(annotations))
			for _, antn := range annotations {
				tbl = append(tbl, []string{string(antn.Name), antn.Value})
			}
			return ShowSection(prefix, "Annotations", tbl)
		}

	case types.Dependency:
		dependency := obj.(types.Dependency)
		return errors.Trace(ShowSection(prefix,
			fmt.Sprintf("%v (%v)", dependency.App, dependency.ImageID),
			labelsTbl(dependency.Labels)))

	case types.Dependencies:
		for _, dependency := range obj.(types.Dependencies) {
			if err := Show(prefix, dependency); err != nil {
				return errors.Trace(err)
			}
		}

	// Just render some types as string
	case *types.Hash, types.Hash:
		return errors.Trace(Showf(prefix, "%v", obj))

	case string:
		for _, line := range strings.Split(strings.TrimRight(obj.(string), "\n"), "\n") {
			if _, err := fmt.Println(prefix + line); err != nil {
				return errors.Trace(err)
			}
		}

	// Fallback
	default:
		return errors.Trace(Showf(prefix, "[%T %#v]", obj, obj))
	}

	return nil
}
