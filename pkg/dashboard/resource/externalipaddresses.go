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

package resource

import (
	"net/http"

	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/restful"
)

type externalIPAddressesResource struct {
	*resource
}

// GetAll returns all externalIPAddressess
func (eiar *externalIPAddressesResource) GetAll(request *http.Request) (map[string]restful.Attributes, error) {
	externalIPAddresses, err := eiar.getPlatform().GetExternalIPAddresses()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get external IP addresses")
	}

	response := map[string]restful.Attributes{
		"externalIPAddresses": {
			"addresses": externalIPAddresses,
		},
	}

	return response, nil
}

// register the resource
var externalIPAddressesResourceInstance = &externalIPAddressesResource{
	resource: newResource("api/external_ip_addresses", []restful.ResourceMethod{
		restful.ResourceMethodGetList,
	}),
}

func init() {
	externalIPAddressesResourceInstance.Resource = externalIPAddressesResourceInstance
	externalIPAddressesResourceInstance.Register(dashboard.DashboardResourceRegistrySingleton)
}
