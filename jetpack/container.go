package jetpack

import "github.com/appc/spec/schema"

type Container struct {
	schema.ContainerRuntimeManifest
	DS Dataset `json:"-"`
}

func CreateContainer(ds *Dataset, img *Image) (*Container, error) {
	c := &Container{*NewContainerRuntimeManifest(), *ds}
	return c, nil
}
