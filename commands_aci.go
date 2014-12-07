package zettajail

import "fmt"
import "io/ioutil"
import "os"
import "path/filepath"

import "github.com/3ofcoins/rocket/app-container/schema"

func (rt *Runtime) CmdExport() error {
	jail, err := rt.Host().GetJail(rt.Args[0])
	if err != nil {
		return err
	}

	manifest, err := schema.NewFilesetManifest(rt.Args[1])
	if err != nil {
		return err
	}

	filelist, err := ioutil.TempFile(jail.Basedir(), "aci.files.")
	if err != nil {
		return err
	}
	defer os.Remove(filelist.Name())

	err = filepath.Walk(jail.Mountpoint,
		func(path string, _ os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if path != jail.Mountpoint {
				relPath := path[len(jail.Mountpoint):]
				manifest.Files = append(manifest.Files, relPath)
				fmt.Fprintln(filelist, "rootfs"+relPath)
			}
			return nil
		})
	if err != nil {
		return err
	}

	manifestJSON, err := manifest.MarshalJSON()
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(jail.Path("fileset"), manifestJSON, 0644)
	if err != nil {
		return err
	}

	fmt.Fprintln(filelist, "fileset")

	return RunCommand("tar", "-cJ", "-C", jail.Basedir(), "-T", filelist.Name(), "-n", "-f", rt.Args[2])
}

func (rt *Runtime) CmdImport() error {
	aci, err := ReadACI(rt.Args[0])
	if err != nil {
		return err
	}

	fmt.Println(aci.Name, aci.Checksum(), aci.FSHash())

	images, err := rt.Host().GetFolder("images")
	if err != nil {
		return err
	}

	// We need to shorten the actual dataset name because of a stupid FreeBSD limit
	// https://lists.freebsd.org/pipermail/freebsd-hackers/2013-November/043798.html
	// 128 bits is good enough for UUID, so it's good enough for us as well
	jail, err := images.CreateJail(aci.FSHash(),
		map[string]string{
			"zettajail:aci:checksum": aci.Checksum(),
			"zettajail:aci:name":     aci.Name.String(),
		})
	if err != nil {
		return err
	}

	err = RunCommand("tar", "-C", jail.Basedir(), "-xf", rt.Args[0])
	if err != nil {
		return err
	}

	_, err = jail.Snapshot("aci", false)
	if err != nil {
		return err
	}

	return jail.SetProperties(map[string]string{
		"canmount": "off",
		"readonly": "on",
	})
}
