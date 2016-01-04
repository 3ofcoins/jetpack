package keystore

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/appc/spec/schema/types"
)

func newStore() *Keystore {
	storePath, err := ioutil.TempDir(".", "test.store.")
	if err != nil {
		panic(err)
	}

	return New(storePath)
}

func testImport(t *testing.T, prefix types.ACIdentifier, subdir string) {
	// Redirect stdout (how to DRY?)
	origStdout := os.Stdout
	defer func() { os.Stdout = origStdout }()
	if devnull, err := os.Create("/dev/null"); err != nil {
		panic(err)
	} else {
		os.Stdout = devnull
		defer devnull.Close()
	}

	defer closeSampleKeys()

	ks := newStore()
	defer os.RemoveAll(ks.Path)

	for i, key := range sampleKeys {
		fingerprint := sampleKeyFingerprints[i]
		keyPath, err := ks.StoreTrustedKey(prefix, openSampleKey(i), fingerprint)

		if err != nil {
			t.Errorf("Error storing key #%d: %v\n", i, err)
		}

		expectedPath := filepath.Join(ks.Path, subdir, fingerprint)
		if keyPath != expectedPath {
			t.Errorf("Unexpected key path: %v, expected %v (key %d, store %v, prefix %v, fingerprint %v)\n",
				keyPath, expectedPath, i, ks.Path, prefix, fingerprint)
		}

		if keyBytes, err := ioutil.ReadFile(keyPath); err != nil {
			t.Errorf("Error reading back saved key %d: %v", i, err)
		} else if string(keyBytes) != key {
			t.Errorf("Saved key %d different than original", i)
		}
	}
}

func TestImportRoot(t *testing.T) {
	testImport(t, Root, "@")
}

func TestImportPrefix(t *testing.T) {
	testImport(t, types.ACIdentifier("example.com"), "example.com")
}

func TestImportPrefixEscaped(t *testing.T) {
	testImport(t, types.ACIdentifier("example.com/foo"), "example.com,foo")
}

func checkKeyCount(t *testing.T, ks *Keystore, expected map[types.ACIdentifier]int) {
	for name, expectedKeys := range expected {
		if kr, err := ks.GetKeysFor(name); err != nil {
			t.Errorf("Error getting keyring for %v: %v\n", name, err)
		} else if actualKeys := kr.Len(); actualKeys != expectedKeys {
			t.Errorf("Expected %d keys for %v, got %d instead\n", expectedKeys, name, actualKeys)
		}
	}
}

func TestGetKeyring(t *testing.T) {
	// Redirect stdout (how to DRY?)
	origStdout := os.Stdout
	defer func() { os.Stdout = origStdout }()
	if devnull, err := os.Create("/dev/null"); err != nil {
		panic(err)
	} else {
		os.Stdout = devnull
		defer devnull.Close()
	}

	ks := newStore()
	defer os.RemoveAll(ks.Path)
	defer closeSampleKeys()

	if _, err := ks.StoreTrustedKey(types.ACIdentifier("example.com/foo"), openSampleKey(0), sampleKeyFingerprints[0]); err != nil {
		t.Errorf("Error storing key: %v\n", err)
	}

	if _, err := ks.StoreTrustedKey(types.ACIdentifier("example.com/foo/bar"), openSampleKey(1), sampleKeyFingerprints[1]); err != nil {
		t.Errorf("Error storing key: %v\n", err)
	}

	checkKeyCount(t, ks, map[types.ACIdentifier]int{
		types.ACIdentifier("eggsample.com"):           0,
		types.ACIdentifier("eggsample.com/foo"):       0,
		types.ACIdentifier("eggsample.com/foo/bar"):   0,
		types.ACIdentifier("example.com"):             0,
		types.ACIdentifier("example.com/foo"):         1,
		types.ACIdentifier("example.com/foo/baz"):     1,
		types.ACIdentifier("example.com/foo/bar"):     2,
		types.ACIdentifier("example.com/foo/bar/baz"): 2,
		types.ACIdentifier("example.com/foobar"):      1,
		types.ACIdentifier("example.com/baz"):         0,
	})

	if _, err := ks.StoreTrustedKey(Root, openSampleKey(2), sampleKeyFingerprints[2]); err != nil {
		t.Errorf("Error storing key: %v\n", err)
	}

	checkKeyCount(t, ks, map[types.ACIdentifier]int{
		types.ACIdentifier("eggsample.com"):           1,
		types.ACIdentifier("eggsample.com/foo"):       1,
		types.ACIdentifier("eggsample.com/foo/bar"):   1,
		types.ACIdentifier("example.com"):             1,
		types.ACIdentifier("example.com/foo"):         2,
		types.ACIdentifier("example.com/foo/baz"):     2,
		types.ACIdentifier("example.com/foo/bar"):     3,
		types.ACIdentifier("example.com/foo/bar/baz"): 3,
		types.ACIdentifier("example.com/foobar"):      2,
		types.ACIdentifier("example.com/baz"):         1,
	})
}

func countKeys(kr *Keyring) map[types.ACIdentifier]int {
	rv := make(map[types.ACIdentifier]int)
	for _, prefix := range kr.prefixes {
		rv[prefix] = rv[prefix] + 1
	}
	return rv
}

func TestGetAllKeyrings(t *testing.T) {
	// Redirect stdout (how to DRY?)
	origStdout := os.Stdout
	defer func() { os.Stdout = origStdout }()
	if devnull, err := os.Create("/dev/null"); err != nil {
		panic(err)
	} else {
		os.Stdout = devnull
		defer devnull.Close()
	}

	ks := newStore()
	defer os.RemoveAll(ks.Path)
	defer closeSampleKeys()

	prefix := types.ACIdentifier("example.com/foo")

	if _, err := ks.StoreTrustedKey(prefix, openSampleKey(0), sampleKeyFingerprints[0]); err != nil {
		t.Errorf("Error storing key: %v\n", err)
	}

	if _, err := ks.StoreTrustedKey(prefix, openSampleKey(1), sampleKeyFingerprints[1]); err != nil {
		t.Errorf("Error storing key: %v\n", err)
	}

	if _, err := ks.StoreTrustedKey(Root, openSampleKey(2), sampleKeyFingerprints[2]); err != nil {
		t.Errorf("Error storing key: %v\n", err)
	}

	kr, err := ks.GetAllKeys()
	if err != nil {
		t.Errorf("Error getting all keyrings: %v\n", err)
		t.FailNow()
	}

	kc := countKeys(kr)

	if len(kc) != 2 {
		t.Errorf("Got %d keyrings, expected 2: %v\n", len(kc), kc)
	}

	if rkc, ok := kc[Root]; !ok {
		t.Error("No root keyring")
	} else if rkc != 1 {
		t.Error("Root keyring %d long, expected 1\n", rkc)
	}

	if pkc, ok := kc[prefix]; !ok {
		t.Error("No prefix keyring")
	} else if pkc != 2 {
		t.Error("Prefix keyring %d long, expected 2\n", kc)
	}
}

type acNames []types.ACIdentifier

// sort.Interface
func (acn acNames) Len() int           { return len(acn) }
func (acn acNames) Less(i, j int) bool { return acn[i].String() < acn[j].String() }
func (acn acNames) Swap(i, j int)      { acn[i], acn[j] = acn[j], acn[i] }

func TestUntrustKey(t *testing.T) {
	// Redirect stdout (how to DRY?)
	origStdout := os.Stdout
	defer func() { os.Stdout = origStdout }()
	if devnull, err := os.Create("/dev/null"); err != nil {
		panic(err)
	} else {
		os.Stdout = devnull
		defer devnull.Close()
	}

	ks := newStore()
	defer os.RemoveAll(ks.Path)
	defer closeSampleKeys()

	prefix := types.ACIdentifier("example.com/foo")
	prefix2 := types.ACIdentifier("example.org/bar")

	if _, err := ks.StoreTrustedKey(prefix, openSampleKey(0), sampleKeyFingerprints[0]); err != nil {
		t.Errorf("Error storing key: %v\n", err)
	}

	if _, err := ks.StoreTrustedKey(prefix, openSampleKey(1), sampleKeyFingerprints[1]); err != nil {
		t.Errorf("Error storing key: %v\n", err)
	}

	if _, err := ks.StoreTrustedKey(prefix2, openSampleKey(1), sampleKeyFingerprints[1]); err != nil {
		t.Errorf("Error storing key: %v\n", err)
	}

	if _, err := ks.StoreTrustedKey(prefix2, openSampleKey(2), sampleKeyFingerprints[2]); err != nil {
		t.Errorf("Error storing key: %v\n", err)
	}

	if _, err := ks.StoreTrustedKey(Root, openSampleKey(2), sampleKeyFingerprints[2]); err != nil {
		t.Errorf("Error storing key: %v\n", err)
	}

	kr, err := ks.GetAllKeys()
	if err != nil {
		panic(err)
	}
	kc := countKeys(kr)

	if kc[Root] != 1 || kc[prefix] != 2 || kc[prefix2] != 2 {
		t.Errorf("Wrong keyrings even before remove: %v\n", kc)
	}

	if prefixes, err := ks.UntrustKey(sampleKeyFingerprints[2]); err != nil {
		t.Errorf("Error untrusting key: %v %v\n", err, prefixes)
	} else {
		sort.Sort(acNames(prefixes))
		expectedPrefixes := []types.ACIdentifier{Root, prefix2}
		if !reflect.DeepEqual(prefixes, expectedPrefixes) {
			t.Errorf("Expected removed prefixes to be %v, got %v instead.\n", expectedPrefixes, prefixes)
		}
	}

	kr, err = ks.GetAllKeys()
	if err != nil {
		panic(err)
	}
	kc = countKeys(kr)

	if kc[Root] != 0 || kc[prefix] != 2 || kc[prefix2] != 1 {
		t.Errorf("Wrong counts after remove: %v\n", kc)
	}
}
