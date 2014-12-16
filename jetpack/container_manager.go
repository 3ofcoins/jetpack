package jetpack

import "github.com/3ofcoins/go-zfs"

type ContainerManager struct {
	*zfs.Dataset `json:"-"`

	Interface   string
	AddressPool string
}

var defaultContainerManager = ContainerManager{
	Interface:   "lo1",
	AddressPool: "172.23.0.1/16",
}
