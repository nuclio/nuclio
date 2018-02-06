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

package local

import (
	"os"

	"github.com/nuclio/nuclio/pkg/platform"
)

type node struct{}

func (n *node) GetAddresses() []platform.Address {

	// get proper url for test
	baseURL := "127.0.0.1"
	if os.Getenv("NUCLIO_TEST_HOST") != "" {
		baseURL = os.Getenv("NUCLIO_TEST_HOST")
	}

	return []platform.Address{
		{
			Type:    platform.AddressTypeExternalIP,
			Address: baseURL,
		},
	}
}
