package jetpack_test

import "fmt"
import "io/ioutil"
import "os"
import "path"
import "path/filepath"
import "strings"
import "testing"

import "github.com/3ofcoins/jetpack/run"
import "github.com/3ofcoins/jetpack/zfs"

const datasetFile = "dataset.zfs"
const snapshotName = "initialized"

var RootDatadir, RootDatasetName string
var RootDataset, RootDatasetSnapshot *zfs.Dataset
var Flags = make(map[string]bool)

func initDataset() error {
	for _, cmd := range [][]string{
		[]string{"jetpack", "init", RootDatadir},
		[]string{"make", "-C", "../images/freebsd-base.release"},
		[]string{"make", "-C", "../images/freebsd-base"},
		[]string{"make", "-C", "../images/example.showenv"},
	} {
		if err := run.Command(cmd[0], cmd[1:]...).Run(); err != nil {
			return err
		}
	}

	if ds, err := zfs.GetDataset(RootDatasetName); err != nil {
		return err
	} else {
		RootDataset = ds
	}

	if snap, err := RootDataset.Snapshot(snapshotName, "-r"); err != nil {
		return err
	} else {
		RootDatasetSnapshot = snap
	}

	return nil
}

func restoreDataset() error {
	if zstream, err := os.Open(datasetFile); err != nil {
		return err
	} else {
		defer zstream.Close()
		if ds, err := zfs.ReceiveDataset(zstream, RootDatasetName, false); err != nil {
			return err
		} else {
			if err := ds.Set("mountpoint", RootDatadir); err != nil {
				return err
			}

			if children, err := ds.Children(-1, "-tfilesystem"); err != nil {
				return err
			} else {
				for _, child := range children {
					if err := child.Mount(); err != nil {
						return err
					}
				}
			}
			RootDataset = ds
		}

		if snap, err := RootDataset.GetSnapshot(snapshotName); err != nil {
			return err
		} else {
			RootDatasetSnapshot = snap
		}
	}
	return nil
}

func saveDataset() error {
	if zstream, err := os.Create(datasetFile); err != nil {
		return err
	} else {
		defer zstream.Close()
		return RootDatasetSnapshot.Send(zstream, "-R")
	}
}

func RollbackDataset(t *testing.T) {
	if err := RootDatasetSnapshot.Rollback("-r"); err != nil {
		t.Error("Cannot rollback dataset:", err)
	}
}

func prepareDataset() error {
	if Flags["quick"] {
		return restoreDataset()
	} else {
		if err := initDataset(); err != nil {
			return err
		}
		if Flags["save"] {
			return saveDataset()
		} else {
			return nil
		}
	}
}

func doRun(m *testing.M) int {
	// Parse flags and variables from command line
	for _, arg := range os.Args {
		if arg == "-test.v" {
			arg = "verbose"
		}
		if arg[0] == '-' {
			continue
		}
		if elts := strings.SplitN(arg, "=", 2); len(elts) == 1 {
			Flags[elts[0]] = true
		} else {
			if err := os.Setenv(elts[0], elts[1]); err != nil {
				panic(err)
			}
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	// Add ../bin to $PATH (FIXME: test could be actually compiled and
	// shipped with the binary, and $PATH setting would be up to the
	// caller; OTOH, this would require shipping also images/)
	if err = os.Setenv("PATH",
		strings.Join(
			[]string{
				filepath.Clean(filepath.Join(cwd, "..", "bin")),
				os.Getenv("PATH"),
			}, ":")); err != nil {
		panic(err)
	}

	var parentDataset *zfs.Dataset

	if parentName := os.Getenv("DATASET"); parentName == "" {
		panic("DATASET environment variable is not set")
	} else {
		if ds, err := zfs.GetDataset(parentName); err != nil {
			panic(err)
		} else {
			parentDataset = ds
		}
	}

	RootDatadir, err = ioutil.TempDir(parentDataset.Mountpoint, "jetpack.")
	if err != nil {
		panic(err)
	}

	if !Flags["keepfs"] {
		defer os.RemoveAll(RootDatadir)
	}

	RootDatasetName = path.Join(parentDataset.Name, path.Base(RootDatadir))

	if err := os.Setenv("JETPACK_ROOT", RootDatasetName); err != nil {
		panic(err)
	}

	defer func() {
		if !Flags["keepfs"] {
			if err := RootDataset.Destroy("-r"); err != nil {
				panic(err)
			}
		} else {
			fmt.Fprintln(os.Stderr, "*** Keeping", RootDataset)
		}
	}()

	// Fill dataset
	if err := prepareDataset(); err != nil {
		panic(err)
	} else if Flags["verbose"] {
		RootDataset.Zfs("list", "-r", "-tall", "-oname,mounted,mountpoint")
	}

	return m.Run()
}

func TestMain(m *testing.M) {
	os.Exit(doRun(m))
}

func TestForSmoke(t *testing.T) {
	RollbackDataset(t)

	if out, err := run.Command("jetpack", "list").OutputString(); err != nil {
		t.Error(err)
	} else {
		t.Logf("jetpack list =>\n%v\n", out)
		if out != "No containers" {
			t.Fatalf("Expected no containers")
		}
	}

	if out, err := run.Command("jetpack", "images").OutputLines(); err != nil {
		t.Error(err)
	} else {
		t.Logf("jetpack images =>\n%v\n", strings.Join(out, "\n"))
		if len(out) != 4 {
			t.Fatal("Expected four lines of output (header + 3 images), instead got", len(out))
		}
	}
}
