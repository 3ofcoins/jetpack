package jetpack

import "github.com/3ofcoins/go-zfs"
import "github.com/appc/spec/schema"

type Container struct {
	schema.ContainerRuntimeManifest
	DS *zfs.Dataset
}

func CreateContainer(h *Host, img *Image) (*Container, error) {
	c := &Container{*NewContainerRuntimeManifest(), nil}
	return c, nil
}
