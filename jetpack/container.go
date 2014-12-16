package jetpack

import "github.com/appc/spec/schema"

type Container struct {
	DS       Dataset                         `json:"-"`
	Manifest schema.ContainerRuntimeManifest `json:"-"`
}

func CreateContainer(ds *Dataset, img *Image) (*Container, error) {
	c := &Container{*ds, *NewContainerRuntimeManifest()}
	return c, nil
}
