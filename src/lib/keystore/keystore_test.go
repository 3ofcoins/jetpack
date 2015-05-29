package keystore

import "bytes"
import "fmt"
import "io/ioutil"
import "os"
import "path/filepath"

import "github.com/appc/spec/schema/types"
import "golang.org/x/crypto/openpgp"

import "testing"

var workDir string

func testImport(t *testing.T, prefix types.ACName, subdir string) {
	storePath, err := ioutil.TempDir(workDir, "store")
	if err != nil {
		panic(err)
	}

	ks := New(storePath)

	for i, key := range sampleKeys {
		fingerprint := sampleKeyFingerprints[i]
		keyPath, err := ks.StoreTrustedKey(prefix, bytes.NewReader([]byte(key)))

		if err != nil {
			t.Errorf("Error storing key #%d: %v\n", i, err)
		}

		expectedPath := filepath.Join(storePath, subdir, fingerprint)
		if keyPath != expectedPath {
			t.Errorf("Unexpected key path: %v, expected %v (key %d, store %v, prefix %v, fingerprint %v)\n",
				keyPath, expectedPath, i, storePath, prefix, fingerprint)
		}

		if keyBytes, err := ioutil.ReadFile(keyPath); err != nil {
			t.Errorf("Error reading back saved key %d: %v", i, err)
		} else if string(keyBytes) != key {
			t.Errorf("Saved key %d different than original", i)
		}
	}
}

func TestImportRoot(t *testing.T) {
	testImport(t, types.ACName(""), "_")
}

func TestImportPrefix(t *testing.T) {
	testImport(t, types.ACName("example.com"), "_example.com")
}

func TestImportPrefixEscaped(t *testing.T) {
	testImport(t, types.ACName("example.com/foo"), "_example.com%2Ffoo")
}

func TestGetKeyring(t *testing.T) {
	storePath, err := ioutil.TempDir(workDir, "store")
	if err != nil {
		panic(err)
	}

	ks := New(storePath)

	if _, err := ks.StoreTrustedKey(types.ACName("example.com/foo"), bytes.NewReader([]byte(sampleKeys[0]))); err != nil {
		t.Errorf("Error storing key: %v\n", err)
	}

	if _, err := ks.StoreTrustedKey(types.ACName("example.com/foo/bar"), bytes.NewReader([]byte(sampleKeys[1]))); err != nil {
		t.Errorf("Error storing key: %v\n", err)
	}

	for name, expectedKeys := range map[types.ACName]int{
		types.ACName("eggsample.com"):           0,
		types.ACName("eggsample.com/foo"):       0,
		types.ACName("eggsample.com/foo/bar"):   0,
		types.ACName("example.com"):             0,
		types.ACName("example.com/foo"):         1,
		types.ACName("example.com/foo/baz"):     1,
		types.ACName("example.com/foo/bar"):     2,
		types.ACName("example.com/foo/bar/baz"): 2,
		types.ACName("example.com/foobar"):      1,
		types.ACName("example.com/baz"):         0,
	} {
		if kr, err := ks.GetKeyring(name); err != nil {
			t.Errorf("Error getting keyring for %v: %v\n", name, err)

		} else if el, ok := kr.(openpgp.EntityList); !ok {
			t.Errorf("CAN'T HAPPEN: keyring for %v not an entity list", name)
		} else if actualKeys := len(el); actualKeys != expectedKeys {
			t.Errorf("Expected %d keys for %v, got %d instead\n", expectedKeys, name, actualKeys)
		}
	}

	if _, err := ks.StoreTrustedKey(types.ACName(""), bytes.NewReader([]byte(sampleKeys[2]))); err != nil {
		t.Errorf("Error storing key: %v\n", err)
	}

	for name, expectedKeys := range map[types.ACName]int{
		types.ACName("eggsample.com"):           1,
		types.ACName("eggsample.com/foo"):       1,
		types.ACName("eggsample.com/foo/bar"):   1,
		types.ACName("example.com"):             1,
		types.ACName("example.com/foo"):         2,
		types.ACName("example.com/foo/baz"):     2,
		types.ACName("example.com/foo/bar"):     3,
		types.ACName("example.com/foo/bar/baz"): 3,
		types.ACName("example.com/foobar"):      2,
		types.ACName("example.com/baz"):         1,
	} {
		if kr, err := ks.GetKeyring(name); err != nil {
			t.Errorf("Error getting keyring for %v: %v\n", name, err)

		} else if el, ok := kr.(openpgp.EntityList); !ok {
			t.Errorf("CAN'T HAPPEN: keyring for %v not an entity list", name)
		} else if actualKeys := len(el); actualKeys != expectedKeys {
			t.Errorf("Expected %d keys for %v, got %d instead\n", expectedKeys, name, actualKeys)
		}
	}
}

func TestMain(m *testing.M) {
	wd, err := ioutil.TempDir("", "jetpack.test.keystore.")
	if err != nil {
		panic(err)
	}
	workDir = wd

	if os.Getenv("JETPACK_TEST_KEEP_FILES") == "" {
		defer os.RemoveAll(wd)
	} else {
		defer fmt.Println("Leaving work directory:", wd)
	}

	os.Exit(m.Run())
}
