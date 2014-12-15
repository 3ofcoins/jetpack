package jetpack

import "fmt"
import "io/ioutil"
import "os"
import "path/filepath"

func (rt *Runtime) CmdExport() error {
	jail, err := rt.Host().GetJail(rt.Args[0])
	if err != nil {
		return err
	}

	manifest := NewImageManifest(rt.Args[1])

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
				manifest.PathWhitelist = append(manifest.PathWhitelist, relPath)
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
	//DONE 	aci, err := ReadACI(rt.Args[0])
	//DONE 	if err != nil {
	//DONE 		return err
	//DONE 	}
	//DONE
	//DONE 	fmt.Println(aci.Name, aci.Checksum(), aci.FSHash())
	//DONE
	//DONE 	images, err := rt.Host().GetFolder("images")
	//DONE 	if err != nil {
	//DONE 		return err
	//DONE 	}
	//DONE
	//DONE 	// We need to shorten the actual dataset name because of a stupid FreeBSD limit
	//DONE 	// https://lists.freebsd.org/pipermail/freebsd-hackers/2013-November/043798.html
	//DONE 	// 128 bits is good enough for UUID, so it's good enough for us as well
	//DONE 	jail, err := images.CreateJail(aci.FSHash(),
	//DONE 		map[string]string{
	//DONE 			"jetpack:aci:checksum": aci.Checksum(),
	//DONE 			"jetpack:aci:name":     aci.Name.String(),
	//DONE 		})
	//DONE 	if err != nil {
	//DONE 		return err
	//DONE 	}
	//DONE
	//DONE 	err = RunCommand("tar", "-C", jail.Basedir(), "-xf", rt.Args[0])
	//DONE 	if err != nil {
	//DONE 		return err
	//DONE 	}
	//DONE
	//DONE 	_, err = jail.Snapshot("aci", false)
	//DONE 	if err != nil {
	//DONE 		return err
	//DONE 	}
	//DONE
	//DONE 	return jail.SetProperties(map[string]string{
	//DONE 		"canmount": "off",
	//DONE 		"readonly": "on",
	//DONE 	})
	return nil
}
