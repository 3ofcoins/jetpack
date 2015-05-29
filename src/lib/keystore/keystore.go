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

func (ks *Keystore) GetKeyring(prefix types.ACName) (openpgp.KeyRing, error) {
	prefixPath := ks.prefixPath(prefix)
	var kr openpgp.EntityList
	if err := filepath.Walk(ks.Path, func(path string, fi os.FileInfo, err error) error {
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if fi == nil {
			return nil
		}

		if fi.IsDir() {
			if strings.HasPrefix(prefixPath, path) {
				return nil
			} else {
				return filepath.SkipDir
			}
		}

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

func fingerprintToFilename(fp [20]byte) string {
	return fmt.Sprintf("%x", fp)
}
