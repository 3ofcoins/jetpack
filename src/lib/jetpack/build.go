package jetpack

import (
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"time"

	"code.google.com/p/go-uuid/uuid"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/juju/errors"

	"lib/run"
)

// Save image to an ACI file, return its hash.
//
// If path is an empty string, don't save the image, just return the
// hash. If path is "-", print image to stdout.
func (img *Image) saveACI(path string, packlist *os.File, perm os.FileMode) (*types.Hash, error) {
	tarArgs := []string{"-C", img.Path(), "-c", "--null", "-f", "-"}
	if packlist != nil {
		tarArgs = append(tarArgs, "-n", "-T", "-")
	} else {
		// no packlist -> flat ACI
		manifest := img.Manifest
		manifest.Dependencies = nil
		manifest.PathWhitelist = nil

		manifestF, err := ioutil.TempFile(img.Path(), "manifest.flat.")
		if err != nil {
			return nil, errors.Trace(err)
		}
		defer os.Remove(manifestF.Name())

		if manifestB, err := json.Marshal(manifest); err != nil {
			manifestF.Close()
			return nil, errors.Trace(err)
		} else {
			_, err := manifestF.Write(manifestB)
			manifestF.Close()
			if err != nil {
				return nil, errors.Trace(err)
			}
		}

		manifestN := filepath.Base(manifestF.Name())
		tarArgs = append(tarArgs, "-s", "/^"+manifestN+"$/manifest/", manifestN, "rootfs")
	}
	archiver := run.Command("tar", tarArgs...).ReadFrom(packlist)
	if archive, err := archiver.StdoutPipe(); err != nil {
		return nil, errors.Trace(err)
	} else {
		hash := sha512.New()
		faucet := io.TeeReader(archive, hash)
		sink := ioutil.Discard
		var compressor *run.Cmd = nil

		if path != "" {
			if path == "-" {
				sink = os.Stdout
			} else {
				if f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, perm); err != nil {
					return nil, errors.Trace(err)
				} else {
					defer f.Close()
					sink = f
				}
			}

			if compression := img.Host.Properties.GetString("images.aci.compression", "no"); compression != "none" {
				switch compression {
				case "xz":
					compressor = run.Command("xz", "-z", "-c")
				case "bzip2":
					compressor = run.Command("bzip2", "-z", "-c")
				case "gz":
				case "gzip":
					compressor = run.Command("gzip", "-c")
				default:
					return nil, errors.Errorf("Invalid setting images.aci.compression=%#v (allowed values: xz, bzip2, gzip, none)", compression)
				}

				compressor.Cmd.Stdout = sink
				if cin, err := compressor.StdinPipe(); err != nil {
					return nil, errors.Trace(err)
				} else {
					sink = cin
				}
			}
		}

		if err := archiver.Start(); err != nil {
			return nil, errors.Trace(err)
		}

		if compressor != nil {
			if err := compressor.Start(); err != nil {
				archiver.Cmd.Process.Kill()
				return nil, errors.Trace(err)
			}
		}

		if _, err := io.Copy(sink, faucet); err != nil {
			return nil, errors.Trace(err)
		}

		if hash, err := types.NewHash(fmt.Sprintf("sha512-%x", hash.Sum(nil))); err != nil {
			// CAN'T HAPPEN, srsly
			return nil, errors.Trace(err)
		} else {
			return hash, nil
		}
	}
}

func (img *Image) SaveFlatACI(path string, perm os.FileMode) (*types.Hash, error) {
	return img.saveACI(path, nil, perm)
}

func (img *Image) buildPodManifest(exec []string) *schema.PodManifest {
	bpm := schema.BlankPodManifest()

	// Figure out working path that doesn't exist in the image's rootfs
	workDir := ".jetpack.build."
	for {
		if _, err := os.Stat(img.getRootfs().Path(workDir)); err != nil {
			if os.IsNotExist(err) {
				break
			}
			panic(err)
		}
		workDir = fmt.Sprintf(".jetpack.build.%v", uuid.NewRandom())
	}

	bprta := img.RuntimeApp()
	bprta.Name.Set("jetpack/build")
	bprta.App = &types.App{
		Exec:             exec,
		WorkingDirectory: "/" + workDir,
		User:             "0",
		Group:            "0",
	}
	bpm.Apps = append(bpm.Apps, bprta)

	// This is needed by freebsd-update at least, should be okay to
	// allow this in builders.
	bpm.Annotations.Set("jetpack/jail.conf/allow.chflags", "true")

	return bpm
}

func (img *Image) Build(buildDir string, addFiles []string, buildExec []string) (*Image, error) {
	buildPod, err := img.Host.CreatePod(img.buildPodManifest(buildExec))
	if err != nil {
		return nil, errors.Trace(err)
	}

	ds, err := img.Host.Dataset.GetDataset(path.Join("pods", buildPod.UUID.String(), "rootfs.0"))
	if err != nil {
		return nil, errors.Trace(err)
	}

	parentSnap, err := ds.Snapshot("parent")
	if err != nil {
		return nil, errors.Trace(err)
	}

	fullWorkDir := buildPod.Path("rootfs/0", buildPod.Manifest.Apps[0].App.WorkingDirectory)
	if err := os.Mkdir(fullWorkDir, 0700); err != nil {
		return nil, errors.Trace(err)
	}

	if buildDir[len(buildDir)-1] != '/' {
		buildDir += "/"
	}

	cpArgs := []string{"-R", buildDir}
	if addFiles != nil {
		cpArgs = append(cpArgs, addFiles...)
	}
	cpArgs = append(cpArgs, fullWorkDir)

	if err := run.Command("cp", cpArgs...).Run(); err != nil {
		return nil, errors.Trace(err)
	}

	if err := buildPod.RunApp(buildPod.Manifest.Apps[0].Name); err != nil {
		return nil, errors.Trace(err)
	}

	if err := buildPod.Kill(); err != nil {
		return nil, errors.Trace(err)
	}

	manifestBytes, err := ioutil.ReadFile(filepath.Join(fullWorkDir, "manifest.json"))
	if err != nil {
		return nil, errors.Trace(err)
	}

	if err := os.RemoveAll(fullWorkDir); err != nil {
		return nil, errors.Trace(err)
	}

	if err := os.Remove(buildPod.Path("rootfs/0/etc/resolv.conf")); err != nil && !os.IsNotExist(err) {
		return nil, errors.Trace(err)
	}

	// Get packing list while parentSnap's name haven't changed

	packlist, err := ioutil.TempFile(buildPod.Path(), "aci.packlist.")
	if err != nil {
		return nil, errors.Trace(err)
	}
	os.Remove(packlist.Name())
	defer packlist.Close()

	haveDeletions := false
	if diffs, err := parentSnap.ZfsFields("diff"); err != nil {
		return nil, errors.Trace(err)
	} else {
		for _, diff := range diffs {
			switch diff[0] {
			case "+", "M":
				fmt.Fprintln(packlist, filepath.Join("rootfs", diff[1][len(ds.Mountpoint):]))
			case "R":
				fmt.Fprintln(packlist, filepath.Join("rootfs", diff[2][len(ds.Mountpoint):]))
				fallthrough
			case "-":
				haveDeletions = true
			default:
				return nil, errors.Errorf("Unknown `zfs diff` line: %v", diff)
			}
		}
	}
	packlist.Seek(0, os.SEEK_SET)

	// Pivot pod into an image
	childImage := NewImage(img.Host, buildPod.UUID)

	if err := ds.Set("mountpoint", childImage.Path("rootfs")); err != nil {
		return nil, errors.Trace(err)
	}

	if err := ds.Rename(img.Host.Dataset.ChildName(path.Join("images", childImage.UUID.String()))); err != nil {
		return nil, errors.Trace(err)
	}

	// Construct the child image's manifest

	if err := json.Unmarshal(manifestBytes, &childImage.Manifest); err != nil {
		return nil, errors.Trace(err)
	}

	// We don't need build pod anymore
	if err := buildPod.Destroy(); err != nil {
		return nil, errors.Trace(err)
	}
	buildPod = nil

	if _, ok := childImage.Manifest.Annotations.Get("timestamp"); !ok {
		if ts, err := time.Now().MarshalText(); err != nil {
			return nil, errors.Trace(err)
		} else {
			childImage.Manifest.Annotations.Set("timestamp", string(ts))
		}
	}

	for _, label := range []string{"os", "arch"} {
		if childValue, ok := childImage.Manifest.GetLabel(label); !ok {
			// if child has no os/arch, copy from parent
			if parentValue, ok := img.Manifest.GetLabel(label); ok {
				childImage.Manifest.Labels = append(childImage.Manifest.Labels,
					types.Label{Name: types.ACName(label), Value: parentValue})
			}
		} else if childValue == "" {
			// if child explicitly set to nil or empty string, remove the
			// label
			for i, l := range childImage.Manifest.Labels {
				if string(l.Name) == label {
					childImage.Manifest.Labels = append(
						childImage.Manifest.Labels[:i],
						childImage.Manifest.Labels[i+1:]...)
					break
				}
			}
		}
	}

	// TODO: option to create a flat image
	childImage.Manifest.Dependencies = append(types.Dependencies{
		types.Dependency{
			App:     img.Manifest.Name,
			ImageID: img.Hash,
			Labels:  img.Manifest.Labels,
		}}, childImage.Manifest.Dependencies...)

	if haveDeletions {
		prefixLen := len(ds.Mountpoint)
		if err := filepath.Walk(ds.Mountpoint, func(path string, _ os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if len(path) == prefixLen {
				// All paths are prefixed with ds.Mountpoint. Cheaper to compare lengths than whole string.
				return nil
			}
			childImage.Manifest.PathWhitelist = append(childImage.Manifest.PathWhitelist, path[prefixLen:])
			return nil
		}); err != nil {
			return nil, errors.Trace(err)
		}
		sort.Strings(childImage.Manifest.PathWhitelist)
	}

	if err := img.saveManifest(); err != nil {
		return nil, errors.Trace(err)
	}

	// Save the ACI
	if hash, err := childImage.saveACI(childImage.Path("aci"), packlist, 0440); err != nil {
		return nil, errors.Trace(err)
	} else {
		childImage.Hash = hash
	}

	if err := childImage.sealImage(); err != nil {
		return nil, errors.Trace(err)
	}

	return childImage, nil
}
