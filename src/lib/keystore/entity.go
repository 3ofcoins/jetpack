package keystore

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/appc/spec/schema/types"
	"golang.org/x/crypto/openpgp"
)

type Entity struct {
	*openpgp.Entity
	Path   string
	Prefix types.ACName
}

func (e *Entity) Fingerprint() string {
	return filepath.Base(e.Path)
}

func (e *Entity) String() string {
	identities := make([]string, 0, len(e.Entity.Identities))
	for name := range e.Entity.Identities {
		identities = append(identities, name)
	}
	sort.Strings(identities)
	return fmt.Sprintf("%v\t%v\t%v", e.Prefix, e.Fingerprint(), strings.Join(identities, "; "))
}

type EntityList []Entity

// sort.Interface
func (ee EntityList) Len() int { return len(ee) }
func (ee EntityList) Less(i, j int) bool {
	if ee[i].Prefix == ee[j].Prefix {
		return ee[i].Path < ee[j].Path
	}
	return ee[i].Prefix.String() < ee[j].Prefix.String()
}
func (ee EntityList) Swap(i, j int) { ee[i], ee[j] = ee[j], ee[i] }
