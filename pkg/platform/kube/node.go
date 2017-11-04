package kube

import (
	"github.com/nuclio/nuclio/pkg/platform"

	"k8s.io/api/core/v1"
)

type node struct {
	v1.Node
}

func (n *node) GetAddresses() []platform.Address {
	var addresses []platform.Address

	for _, nodeAddress := range n.Status.Addresses {
		var addressType platform.AddressType

		switch nodeAddress.Type {
		case v1.NodeExternalIP:
			addressType = platform.AddressTypeExternalIP
		case v1.NodeInternalIP:
			addressType = platform.AddressTypeInternalIP
		default:
			continue
		}

		addresses = append(addresses, platform.Address{
			Address: nodeAddress.Address,
			Type:    addressType,
		})
	}

	return addresses
}
