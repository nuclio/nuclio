/*
Copyright 2023 The Nuclio Authors.

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
	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/restful"
	"net/http"
)

type registryKind struct {
	*resource
}

func (rk *registryKind) ExtendMiddlewares() error {
	rk.resource.addAuthMiddleware(nil)
	return nil
}

// GetAll returns registry kind
func (rk *registryKind) GetAll(request *http.Request) (map[string]restful.Attributes, error) {
	registryKindValue := rk.getPlatform().GetRegistryKind()

	response := map[string]restful.Attributes{
		"registryKind": {
			"kind": registryKindValue,
		},
	}

	return response, nil
}

// register the resource
var registryKindResourceInstance = &registryKind{
	resource: newResource("api/registry_kind", []restful.ResourceMethod{
		restful.ResourceMethodGetList,
	}),
}

func init() {
	registryKindResourceInstance.Resource = registryKindResourceInstance
	registryKindResourceInstance.Register(dashboard.DashboardResourceRegistrySingleton)
}
