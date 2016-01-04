package acutil

import (
	"crypto/sha512"

	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
)

func IsHashPartial(hash *types.Hash) bool {
	// We assume that hash.typ == "sha512". The field is not exported
	// (WHY?), so we can't double-check that.
	return len(hash.Val) < sha512.Size*2
}

func IsPodManifestEmpty(pm *schema.PodManifest) bool {
	return pm == nil ||
		(len(pm.Apps) == 0 &&
			len(pm.Volumes) == 0 &&
			len(pm.Isolators) == 0 &&
			len(pm.Annotations) == 0 &&
			len(pm.Ports) == 0)
}
