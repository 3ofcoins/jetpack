package keystore

// Heavily based on https://github.com/coreos/rkt/blob/master/pkg/keystore/keystore.go

// We don't use rkt's keystore, because we want to escape ACIdentifier to
// avoid path travelrsal issues and not to worry about prefix
// collisions.

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/juju/errors"
	"golang.org/x/crypto/openpgp"

	"github.com/appc/spec/schema/types"
)

// Intentionally invalid ACIdentifier to mark root key
const Root = types.ACIdentifier("@")

type Keystore struct {
	Path string
}

func New(path string) *Keystore {
	return &Keystore{path}
}

func (ks *Keystore) prefixPath(prefix types.ACIdentifier) string {
	if prefix.Empty() {
		panic("Empty prefix!")
	}
	return filepath.Join(ks.Path, strings.Replace(string(prefix), "/", ",", -1))
}

func (ks *Keystore) StoreTrustedKey(prefix types.ACIdentifier, key *os.File, forceAccept bool) (string, error) {
	if prefix.Empty() {
		panic("Empty prefix!")
	}

	if accepted, err := reviewKey(prefix, key, forceAccept); err != nil {
		return "", err
	} else if !accepted {
		return "", nil
	}

	pubkeyBytes, err := ioutil.ReadAll(key)
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

func (ks *Keystore) UntrustKey(fingerprint string) (removed []types.ACIdentifier, err error) {
	err = ks.walk("", func(prefix types.ACIdentifier, path string) error {
		if filepath.Base(path) == fingerprint {
			if err := os.Remove(path); err != nil {
				return err
			}
			removed = append(removed, prefix)
		}
		return nil
	})
	return
}

func (ks *Keystore) walk(name types.ACIdentifier, fn func(prefix types.ACIdentifier, path string) error) error {
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
			if namePath == "" || strings.HasPrefix(namePath, path) || fi.Name() == "@" {
				return nil
			} else {
				return filepath.SkipDir
			}
		}

		if prefix, err := pathToACIdentifier(path); err != nil {
			return errors.Trace(err)
		} else {
			return fn(prefix, path)
		}
	})
}

func walkLoaderFn(kr *Keyring) func(types.ACIdentifier, string) error {
	return func(_ types.ACIdentifier, path string) error {
		return kr.loadFile(path)
	}
}

func (ks *Keystore) GetAllKeys() (*Keyring, error) {
	kr := &Keyring{}
	if err := ks.walk("", walkLoaderFn(kr)); err != nil {
		return nil, err
	}
	return kr, nil
}

func (ks *Keystore) GetKeysFor(name types.ACIdentifier) (*Keyring, error) {
	kr := &Keyring{}
	if err := ks.walk(name, walkLoaderFn(kr)); err != nil {
		return nil, err
	}
	return kr, nil
}

func (ks *Keystore) CheckSignature(name types.ACIdentifier, signed, signature io.Reader) (*openpgp.Entity, error) {
	kr, err := ks.GetKeysFor(name)
	if err != nil {
		return nil, errors.Trace(err)
	}
	entities, err := openpgp.CheckArmoredDetachedSignature(kr, signed, signature)
	if err == io.EOF {
		err = errors.New("No signatures found")
	}
	return entities, err
}
