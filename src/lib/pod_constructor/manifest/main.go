package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/appc/spec/schema/types"

	"lib/pod_constructor"
)

func main() {
	if pm, err := pod_constructor.ConstructPodManifest(nil, os.Args[1:]); err != nil {
		panic(err)
	} else {
		// FIXME: make appc/spec accept empty hash?
		hash, _ := types.NewHash("sha512-FIXME")
		for i, app := range pm.Apps {
			if app.Image.ID.Empty() {
				pm.Apps[i].Image.ID = *hash
			}
		}

		if pmjson, err := json.MarshalIndent(pm, "", "  "); err != nil {
			panic(err)
		} else {
			fmt.Println(string(pmjson))
		}
	}
}
