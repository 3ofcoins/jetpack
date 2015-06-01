package keystore

import (
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/appc/spec/schema/types"
	"github.com/juju/errors"
)

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
