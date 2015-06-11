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
	"lib/ui"
)

//  Write ACI to `, return its hash. If packlist file is nil, writes
//  flat ACI (and manifest omits the dependencies).
func (img *Image) writeACI(w io.Writer, packlist *os.File) (*types.Hash, error) {
	var sink io.Writer
	var faucet io.Reader

	if sw := ui.NewSpinningWriter("Writing ACI", w); true {
		defer sw.Close()
		sink = sw
	}

	tarArgs := []string{"-C", img.Path(), "-c", "--null", "-f", "-"}
	if packlist != nil {
		img.ui.Debug("Writing an incremental ACI")
		tarArgs = append(tarArgs, "-n", "-T", "-")
	} else {
		img.ui.Debug("Writing a flat ACI")

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

	tar := run.Command("tar", tarArgs...).ReadFrom(packlist)
	if tarPipe, err := tar.StdoutPipe(); err != nil {
		return nil, errors.Trace(err)
	} else {
		faucet = tarPipe
	}

	hash := sha512.New()
	faucet = io.TeeReader(faucet, hash)

	var compressor *run.Cmd = nil
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

	if err := tar.Start(); err != nil {
		return nil, errors.Trace(err)
	}

	if compressor != nil {
		if err := compressor.Start(); err != nil {
			tar.Cmd.Process.Kill()
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
		img.ui.Debug("Saved", hash)
		return hash, nil
	}
}

func (img *Image) WriteFlatACI(w io.Writer) (*types.Hash, error) {
	return img.writeACI(w, nil)
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
	bpm.Annotations.Set("jetpack/jail.conf/securelevel", "0")

	return bpm
}

func (img *Image) Build(buildDir string, addFiles []string, buildExec []string) (*Image, error) {
	img.ui.Println("Preparing build pod")
	abuilddir, _ := filepath.Abs(buildDir)
	img.ui.Debug("Build dir:", abuilddir)
	img.ui.Debug("Extra files:", run.ShellEscape(addFiles...))
	img.ui.Debug("Build command:", run.ShellEscape(buildExec...))
	buildPod, err := img.Host.CreatePod(img.buildPodManifest(buildExec))
	if err != nil {
		return nil, errors.Trace(err)
	}

	ui := ui.NewUI("cyan", "build", buildPod.UUID.String())

	workDir := buildPod.Manifest.Apps[0].App.WorkingDirectory
	ui.Debugf("Preparing build environment in %v", workDir)

	ds, err := img.Host.Dataset.GetDataset(path.Join("pods", buildPod.UUID.String(), "rootfs.0"))
	if err != nil {
		return nil, errors.Trace(err)
	}

	fullWorkDir := ds.Path(workDir)
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

	ui.Println("Running the build")
	if err := buildPod.RunApp(buildPod.Manifest.Apps[0].Name); err != nil {
		return nil, errors.Trace(err)
	}

	if err := buildPod.Kill(); err != nil {
		return nil, errors.Trace(err)
	}

	ui.Debug("Reading new image manifest")
	manifestBytes, err := ioutil.ReadFile(filepath.Join(fullWorkDir, "manifest.json"))
	if err != nil {
		return nil, errors.Trace(err)
	}

	ui.Debug("Removing work dir")
	if err := os.RemoveAll(fullWorkDir); err != nil {
		return nil, errors.Trace(err)
	}

	if err := os.Remove(ds.Path("etc/resolv.conf")); err != nil && !os.IsNotExist(err) {
		return nil, errors.Trace(err)
	}

	ui.Println("Pivoting build pod into new image")

	// Pivot pod into an image
	childImage := NewImage(img.Host, buildPod.UUID)

	if err := ds.Set("mountpoint", childImage.Path("rootfs")); err != nil {
		return nil, errors.Trace(err)
	}

	if err := ds.Rename(img.Host.Dataset.ChildName(path.Join("images", childImage.UUID.String()))); err != nil {
		return nil, errors.Trace(err)
	}

	// Construct the child image's manifest

	ui.Debug("Constructing new image manifest")

	if err := json.Unmarshal(manifestBytes, &childImage.Manifest); err != nil {
		return nil, errors.Trace(err)
	}

	// We don't need build pod anymore
	if err := buildPod.Destroy(); err != nil {
		return nil, errors.Trace(err)
	}
	buildPod = nil

	if _, ok := childImage.Manifest.Annotations.Get("timestamp"); !ok {
		childImage.Manifest.Annotations.Set("timestamp", time.Now().Format(time.RFC3339))
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

	childImage.Manifest.Dependencies = append(types.Dependencies{
		types.Dependency{
			App:     img.Manifest.Name,
			ImageID: img.Hash,
			Labels:  img.Manifest.Labels,
		}}, childImage.Manifest.Dependencies...)

	// Get packing list out of `zfs diff`

	ui.Debug("Generating incremental packing list")

	packlist, err := ioutil.TempFile(childImage.Path(), "aci.packlist.")
	if err != nil {
		return nil, errors.Trace(err)
	}
	os.Remove(packlist.Name())
	defer packlist.Close()
	io.WriteString(packlist, "manifest")

	// To figure out whether a deleted file has been re-added (and
	// should be kept in PathWhitelist after all), we keep changes in a
	// map: a false value means there was an addition; true value means
	// a deletion. False overwrites true, true never overwrites false.
	deletionMap := make(map[string]bool)

	if snap, err := ds.GetSnapshot("parent"); err != nil {
		return nil, errors.Trace(err)
	} else if diffs, err := snap.ZfsFields("diff"); err != nil {
		return nil, errors.Trace(err)
	} else {
		for _, diff := range diffs {
			path1 := diff[1][len(ds.Mountpoint):]
			switch diff[0] {
			case "+", "M":
				io.WriteString(packlist, filepath.Join("\000rootfs", path1))
				deletionMap[path1] = false
			case "R":
				path2 := diff[2][len(ds.Mountpoint):]
				deletionMap[path2] = false
				io.WriteString(packlist, filepath.Join("\000rootfs", path2))
				fallthrough
			case "-":
				if _, ok := deletionMap[path1]; !ok {
					// if found in map, either already true (no need to set
					// again), or false (which should stay)
					deletionMap[path1] = true
				}
			default:
				return nil, errors.Errorf("Unknown `zfs diff` line: %v", diff)
			}
		}
	}
	packlist.Seek(0, os.SEEK_SET)

	// Check if there were any deletions. If there weren't any, we don't
	// need to prepare a path whitelist.
	haveDeletions := false
	for _, isDeletion := range deletionMap {
		if isDeletion {
			haveDeletions = true
			break
		}
	}

	// If any files from parent were deleted, fill in path whitelist
	if haveDeletions {
		ui.Debug("Some files were deleted, filling in path whitelist")
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

	if err := childImage.saveManifest(); err != nil {
		return nil, errors.Trace(err)
	}

	// Save the ACI
	if f, err := os.OpenFile(childImage.Path("aci"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0440); err != nil {
		return nil, errors.Trace(err)
	} else {
		defer f.Close()
		if hash, err := childImage.writeACI(f, packlist); err != nil {
			return nil, errors.Trace(err)
		} else {
			childImage.Hash = hash
		}
	}

	if err := childImage.sealImage(); err != nil {
		return nil, errors.Trace(err)
	}

	return childImage, nil
}
