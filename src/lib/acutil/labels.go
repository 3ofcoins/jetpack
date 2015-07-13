package acutil

import "github.com/appc/spec/schema/types"

// True if all labels in `labels` are present in `candidate` and have
// the same value
func MatchLabels(labels, candidate types.Labels) bool {
	if len(labels) == 0 {
		return true
	}
	cmap := candidate.ToMap()
	for _, label := range labels {
		if v, ok := cmap[label.Name]; !ok || v != label.Value {
			return false
		}
	}
	return true
}
