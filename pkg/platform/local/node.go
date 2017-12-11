package local

import "github.com/nuclio/nuclio/pkg/platform"

type node struct{}

func (n *node) GetAddresses() []platform.Address {
	return []platform.Address{
		{
			Type:    platform.AddressTypeExternalIP,
			Address: "127.0.0.1",
		},
	}
}
