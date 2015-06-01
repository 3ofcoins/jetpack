package keystore

import (
	"os"
	"path/filepath"

	"github.com/appc/spec/schema/types"
	"github.com/juju/errors"
	"golang.org/x/crypto/openpgp"
)

type Keyring struct {
	openpgp.EntityList
	paths    []string
	prefixes []types.ACName
}

func (kr *Keyring) loadFile(path string) error {
	trustedKey, err := os.Open(path)
	if err != nil {
		return err
	}
	defer trustedKey.Close()
	entityList, err := openpgp.ReadArmoredKeyRing(trustedKey)
	if err != nil {
		return err
	}
	if len(entityList) < 1 {
		return errors.New("missing opengpg entity")
	}
	fingerprint := fingerprintToFilename(entityList[0].PrimaryKey.Fingerprint)
	keyFile := filepath.Base(trustedKey.Name())
	if fingerprint != keyFile {
		return errors.Errorf("fingerprint mismatch: %q:%q", keyFile, fingerprint)
	}

	prefix, err := pathToACName(path)
	if err != nil {
		return err
	}

	kr.EntityList = append(kr.EntityList, entityList[0])
	kr.paths = append(kr.paths, path)
	kr.prefixes = append(kr.prefixes, *prefix)

	return nil
}

func (kr *Keyring) Entities() EntityList {
	rv := make(EntityList, len(kr.EntityList))
	for i, e := range kr.EntityList {
		rv[i] = Entity{e, kr.paths[i], kr.prefixes[i]}
	}
	return rv
}

// sort.Interface - sort by prefix, then by path
func (kr *Keyring) Len() int { return len(kr.EntityList) }

func (kr *Keyring) Less(i, j int) bool {
	if kr.prefixes[i] == kr.prefixes[j] {
		return kr.paths[i] < kr.paths[j]
	}
	return kr.prefixes[i].String() < kr.prefixes[j].String()
}

func (kr *Keyring) Swap(i, j int) {
	kr.EntityList[i], kr.EntityList[j] = kr.EntityList[j], kr.EntityList[i]
	kr.paths[i], kr.paths[j] = kr.paths[j], kr.paths[i]
	kr.prefixes[i], kr.prefixes[j] = kr.prefixes[j], kr.prefixes[i]
}
