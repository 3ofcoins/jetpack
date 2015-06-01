package keystore

// Heavily based on https://github.com/coreos/rkt/blob/master/pkg/keystore/keystore.go

// We don't use rkt's keystore, because we want to escape ACName to
// avoid path travelrsal issues and not to worry about prefix
// collisions.

import "bytes"
import "io"
import "io/ioutil"
import "net/url"
import "os"
import "path/filepath"
import "strings"

import "github.com/juju/errors"
import "golang.org/x/crypto/openpgp"

import "github.com/appc/spec/schema/types"

const Root = types.ACName("")

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

func (ks *Keystore) UntrustKey(fingerprint string) (removed []types.ACName, err error) {
	err = ks.walk(Root, func(prefix types.ACName, path string) error {
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
			return fn(Root, path)
		} else if prefix, err := types.NewACName(sPrefix); err != nil {
			return errors.Trace(err)
		} else {
			return fn(*prefix, path)
		}
	})
}

func (ks *Keystore) GetKeyring(name types.ACName) (*Keyring, error) {
	kr := &Keyring{}
	if err := ks.walk(name, func(_ types.ACName, path string) error {
		return kr.loadFile(path)
	}); err != nil {
		return nil, err
	}
	return kr, nil
}
