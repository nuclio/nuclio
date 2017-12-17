/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
