package jetpack

import "fmt"
import "sort"
import "strconv"
import "strings"

import "github.com/appc/spec/schema"
import "github.com/appc/spec/schema/types"
import "github.com/juju/errors"
import "github.com/magiconair/properties"

import "github.com/3ofcoins/jetpack/ui"
import "github.com/3ofcoins/jetpack/zfs"

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

type SectionTable struct {
	Name     string
	Contents [][]string
}

func ShowSection(ui *ui.UI, name string, obj ...interface{}) error {
	return ui.Section(name, func() error { return Show(ui, obj...) })
}

func Show(ui *ui.UI, objs ...interface{}) error {
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
			if err := Show(ui, obj); err != nil {
				return errors.Trace(err)
			}
		}

	case [][]string:
		ui.Table(obj.([][]string))

	case *properties.Properties:
		p := obj.(*properties.Properties)
		tbl := make([][]string, p.Len())
		for i, k := range p.Keys() {
			v, _ := p.Get(k)
			tbl[i] = []string{k, v}
		}
		return Show(ui, SectionTable{"Configuration", tbl})

	case SectionTable:
		if st := obj.(SectionTable); len(st.Contents) > 0 {
			return ShowSection(ui, st.Name, st.Contents)
		}

	case *Host:
		h := obj.(*Host)
		isdev := ""
		if IsDevelopment {
			isdev = " (development)"
		}
		ui.Sayf("JetPack %v (%v), compiled on %v%v", Version, Revision, BuildTimestamp, isdev)
		return Show(ui, h.Dataset, h.Properties)

	case *zfs.Dataset:
		ds := obj.(*zfs.Dataset)
		tbl := [][]string{[]string{"Mountpoint", ds.Mountpoint}}
		if ds.Origin != "" {
			tbl = append(tbl, []string{"Origin", ds.Origin})
		}

		return ShowSection(ui, fmt.Sprintf("ZFS Dataset %v", ds.Name), tbl)

	case *Image:
		if img := obj.(*Image); img != nil {
			metadata := [][]string{}
			if img.Hash != nil {
				metadata = append(metadata, []string{"Hash", img.Hash.String()})
			}
			if img.Origin != "" {
				metadata = append(metadata, []string{"Origin", img.Origin})
			}
			metadata = append(metadata, []string{"Timestamp", img.Timestamp.String()})

			containersTbl := SectionTable{Name: "Containers"}
			if containers, err := img.Containers(); err != nil {
				return errors.Trace(err)
			} else if len(containers) > 0 {
				containersTbl.Contents = containers.Table()
			}

			return ShowSection(ui, fmt.Sprintf("Image %v", img.UUID),
				img.Dataset,
				metadata,
				img.Manifest,
				containersTbl,
			)
		}

	case *Container:
		c := obj.(*Container)
		img, err := c.GetImage()
		if err != nil {
			return errors.Trace(err)
		}

		return ShowSection(ui, fmt.Sprintf("Container %v", c.Manifest.UUID),
			c.Dataset,
			c.Manifest,
			img)

	case schema.ImageManifest:
		manifest := obj.(schema.ImageManifest)
		return ShowSection(ui, fmt.Sprintf("Manifest %v", manifest.Name),
			manifest.Labels,
			manifest.App,
			manifest.Annotations,
			manifest.Dependencies)

	case schema.ContainerRuntimeManifest:
		manifest := obj.(schema.ContainerRuntimeManifest)
		return ShowSection(ui, "Manifest",
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

			envKeys := make([]string, 0, len(app.Environment))
			for k := range app.Environment {
				envKeys = append(envKeys, k)
			}
			sort.Strings(envKeys)
			envTbl := make([][]string, len(app.Environment))
			for i, k := range envKeys {
				envTbl[i] = []string{k, app.Environment[k]}
			}

			return ShowSection(ui, "App",
				metaTbl,
				SectionTable{"Environment", envTbl},
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
			return ShowSection(ui, "Apps", appsI)
		}

	case schema.RuntimeApp:
		app := obj.(schema.RuntimeApp)
		return ShowSection(ui, string(app.Name),
			app.ImageID, app.Isolators, types.Annotations(app.Annotations))

	case []types.MountPoint:
		if mntpts := obj.([]types.MountPoint); len(mntpts) > 0 {
			tbl := make([][]string, len(mntpts))
			for i, mntpt := range mntpts {
				tbl[i] = []string{string(mntpt.Name), mntpt.Path}
				if mntpt.ReadOnly {
					tbl[i] = append(tbl[i], "ro")
				}
			}
			return ShowSection(ui, "Mount Points", tbl)
		}

	case []types.Volume:
		if vols := obj.([]types.Volume); len(vols) > 0 {
			tbl := make([]interface{}, len(vols))
			for i, vol := range vols {
				fulfills := make([]string, len(vol.Fulfills))
				for i, name := range vol.Fulfills {
					fulfills[i] = string(name)
				}
				hdr := strings.Join(fulfills, ", ")
				if vol.ReadOnly {
					hdr += " (ro)"
				}

				entry := SectionTable{hdr, [][]string{[]string{"Kind", vol.Kind}}}

				if vol.Source != "" {
					entry.Contents = append(entry.Contents, []string{"Source", vol.Source})
				}
				tbl[i] = entry
			}
			return ShowSection(ui, "Volumes", tbl)
		}

	case []types.EventHandler:
		if handlers := obj.([]types.EventHandler); len(handlers) > 0 {
			tbl := make([][]string, len(handlers))
			for i, handler := range handlers {
				tbl[i] = []string{handler.Name, showExec(handler.Exec)}
			}
			return ShowSection(ui, "Event Handlers", tbl)
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
			return ShowSection(ui, "Ports", tbl)
		}

	case []types.Isolator:
		if isolators := obj.([]types.Isolator); len(isolators) > 0 {
			tbl := make([][]string, len(isolators))
			for i, isolator := range isolators {
				tbl[i] = []string{string(isolator.Name), isolator.Val}
			}
			return ShowSection(ui, "Isolators", tbl)
		}

	case types.Labels:
		if labels := obj.(types.Labels); len(labels) > 0 {
			return ShowSection(ui, "Labels", labelsTbl(labels))
		}

	case types.Annotations:
		if annotations := obj.(types.Annotations); len(annotations) > 0 {
			tbl := make([][]string, 0, len(annotations))
			for name, value := range annotations {
				tbl = append(tbl, []string{string(name), value})
			}
			return ShowSection(ui, "Annotations", tbl)
		}

	case types.Dependencies:
		if dependencies := obj.(types.Dependencies); len(dependencies) > 0 {
			return ui.Section("Dependencies", func() error {
				for _, dependency := range dependencies {
					header := fmt.Sprintf("%v (%v)", dependency.App, dependency.ImageID)
					if err := ShowSection(ui, header, labelsTbl(dependency.Labels)); err != nil {
						return err
					}
				}
				return nil
			})
		}

	// Just render some types as string
	case *types.Hash, types.Hash, string:
		ui.Sayf("%v", obj)

	// Fallback
	default:
		return ui.Section(fmt.Sprintf("[%T %v]", obj, obj), func() error {
			ui.Sayf("%#v", obj)
			return nil
		})
	}

	return nil
}
