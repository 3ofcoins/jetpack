package keystore

// Heavily based on https://github.com/coreos/rkt/blob/master/pkg/keystore/keystore.go

// We don't use rkt's keystore, because we want to escape ACName to
// avoid path travelrsal issues and not to worry about prefix
// collisions.

import "bytes"
import "fmt"
import "io"
import "io/ioutil"
import "net/url"
import "os"
import "path/filepath"
import "strings"

import "github.com/juju/errors"
import "golang.org/x/crypto/openpgp"

import "github.com/appc/spec/schema/types"

type Keystore struct {
	Path string
}

func New(path string) *Keystore {
	return &Keystore{path}
}

func (ks *Keystore) prefixPath(prefix types.ACName) string {
	return filepath.Join(ks.Path, "_"+url.QueryEscape(string(prefix)))
}

func (ks *Keystore) StoreTrustedKey(prefix types.ACName, r io.Reader) (string, error) {
	pubkeyBytes, err := ioutil.ReadAll(r)
	if err != nil {
		return "", err
	}

	dir := ks.prefixPath(prefix)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", err
	}

	entityList, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(pubkeyBytes))
	if err != nil {
		return "", err
	}

	// FIXME: cargo cult from rkt
	// FIXME: can we import more than one key here, and note only one?
	// Maybe we should split and re-armor the entityList?
	trustedKeyPath := filepath.Join(dir, fingerprintToFilename(entityList[0].PrimaryKey.Fingerprint))

	if err := ioutil.WriteFile(trustedKeyPath, pubkeyBytes, 0640); err != nil {
		return "", err
	}

	return trustedKeyPath, nil
}

func entityFromFile(path string) (*openpgp.Entity, error) {
	trustedKey, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer trustedKey.Close()
	entityList, err := openpgp.ReadArmoredKeyRing(trustedKey)
	if err != nil {
		return nil, err
	}
	if len(entityList) < 1 {
		return nil, errors.New("missing opengpg entity")
	}
	fingerprint := fingerprintToFilename(entityList[0].PrimaryKey.Fingerprint)
	keyFile := filepath.Base(trustedKey.Name())
	if fingerprint != keyFile {
		return nil, errors.Errorf("fingerprint mismatch: %q:%q", keyFile, fingerprint)
	}
	return entityList[0], nil
}

func (ks *Keystore) walk(name types.ACName, fn func(prefix types.ACName, path string) error) error {
	var namePath string
	if !name.Empty() {
		namePath = ks.prefixPath(name)
	}
	return filepath.Walk(ks.Path, func(path string, fi os.FileInfo, err error) error {
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if fi == nil {
			return nil
		}

		if fi.IsDir() {
			if namePath == "" || strings.HasPrefix(namePath, path) {
				return nil
			} else {
				return filepath.SkipDir
			}
		}

		if sPrefix, err := url.QueryUnescape(filepath.Base(filepath.Dir(path))[1:]); err != nil {
			return errors.Trace(err)
		} else if sPrefix == "" {
			// to avoid ErrEmptyACName; any better ideas?
			return fn(types.ACName(""), path)
		} else if prefix, err := types.NewACName(sPrefix); err != nil {
			return errors.Trace(err)
		} else {
			return fn(*prefix, path)
		}
	})
}

func (ks *Keystore) GetKeyring(name types.ACName) (openpgp.EntityList, error) {
	var kr openpgp.EntityList
	if err := ks.walk(name, func(_ types.ACName, path string) error {
		if ety, err := entityFromFile(path); err != nil {
			return err
		} else {
			kr = append(kr, ety)
			return nil
		}
	}); err != nil {
		return nil, err
	}
	return kr, nil
}

func (ks *Keystore) GetAllKeyrings() (map[types.ACName]openpgp.EntityList, error) {
	rv := make(map[types.ACName]openpgp.EntityList)
	if err := ks.walk(types.ACName(""), func(prefix types.ACName, path string) error {
		if ety, err := entityFromFile(path); err != nil {
			return err
		} else {
			rv[prefix] = append(rv[prefix], ety)
			return nil
		}
	}); err != nil {
		return nil, err
	}
	return rv, nil
}

func fingerprintToFilename(fp [20]byte) string {
	return fmt.Sprintf("%x", fp)
}
