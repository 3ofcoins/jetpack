package jetpack

type ContainerManager struct {
	Dataset `json:"-"`

	Interface   string
	AddressPool string
}

var defaultContainerManager = ContainerManager{
	Interface:   "lo1",
	AddressPool: "172.23.0.1/16",
}
