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
	"github.com/nuclio/nuclio/pkg/restful"
)

type namespaceResource struct {
	*resource
}

func (nr *namespaceResource) ExtendMiddlewares() error {
	nr.resource.addAuthMiddleware()
	return nil
}

// GetAll returns all namespaces
func (nr *namespaceResource) GetAll(request *http.Request) (map[string]restful.Attributes, error) {
	ctx := request.Context()
	namespaces, err := nr.getPlatform().GetNamespaces(ctx)
	if err != nil {
		return nil, err
	}

	response := map[string]restful.Attributes{
		"namespaces": {
			"names": namespaces,
		},
	}

	return response, nil
}

// register the resource
var namespaceResourceInstance = &namespaceResource{
	resource: newResource("api/namespaces", []restful.ResourceMethod{
		restful.ResourceMethodGetList,
	}),
}

func init() {
	namespaceResourceInstance.Resource = namespaceResourceInstance
	namespaceResourceInstance.Register(dashboard.DashboardResourceRegistrySingleton)
}
