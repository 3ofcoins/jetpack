package keystore

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/openpgp"

	"github.com/appc/spec/schema/types"
	"github.com/juju/errors"
)

func fingerToString(fpr [20]byte) string {
	str := ""
	for i, b := range fpr {
		if i > 0 && i%2 == 0 {
			str += " "
			if i == 10 {
				str += " "
			}
		}
		str += strings.ToUpper(fmt.Sprintf("%.2x", b))
	}
	return str
}

func KeyDescription(ety *openpgp.Entity) string {
	rv := make([]string, 2+len(ety.Subkeys)+len(ety.Identities))
	rv[0] = fmt.Sprintf("GPG key fingerprint: %s", fingerToString(ety.PrimaryKey.Fingerprint))
	for i, sk := range ety.Subkeys {
		rv[i+1] = fmt.Sprintf(" Subkey fingerprint: %s", fingerToString(sk.PublicKey.Fingerprint))
	}
	i := len(ety.Subkeys) + 1
	rv[i] = "Identities:"
	for id := range ety.Identities {
		rv[i] = fmt.Sprintf(" - %v", id)
		i += 1
	}
	return strings.Join(rv, "\n")
}

func pathToACName(path string) (*types.ACName, error) {
	if dirname := filepath.Base(filepath.Dir(path)); dirname[0] != '_' {
		return nil, errors.Errorf("Directory is not a quoted ACName: %v", dirname)
	} else if prefix, err := url.QueryUnescape(dirname[1:]); err != nil {
		return nil, err
	} else if prefix, err := types.NewACName(prefix); err == types.ErrEmptyACName {
		root := Root
		return &root, nil
	} else if err != nil {
		return nil, err
	} else {
		return prefix, nil
	}
}

func fingerprintToFilename(fp [20]byte) string {
	return fmt.Sprintf("%x", fp)
}
