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

type frontendSpecResource struct {
	*resource
}

func (fesr *frontendSpecResource) getFrontendSpec(request *http.Request) (*restful.CustomRouteFuncResponse, error) {
	externalIPAddresses, err := fesr.getPlatform().GetExternalIPAddresses()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get external IP addresses")
	}

	frontendSpec := map[string]restful.Attributes{
		"frontendSpec": { // frontendSpec is the ID of this singleton resource
			"externalIPAddresses":            externalIPAddresses,
			"namespace":                      fesr.getNamespaceOrDefault(""),
			"defaultHTTPIngressHostTemplate": fesr.getPlatform().GetDefaultHTTPIngressHostTemplate(),
		},
	}

	return &restful.CustomRouteFuncResponse{
		Single:     true,
		StatusCode: http.StatusOK,
		Resources:  frontendSpec,
	}, nil
}

// returns a list of custom routes for the resource
func (fesr *frontendSpecResource) GetCustomRoutes() ([]restful.CustomRoute, error) {

	// since frontendSpec is a singleton we create a custom route that will return this single object
	return []restful.CustomRoute{
		{
			Pattern:   "/",
			Method:    http.MethodGet,
			RouteFunc: fesr.getFrontendSpec,
		},
	}, nil
}

// register the resource
var frontendSpecResourceInstance = &frontendSpecResource{
	resource: newResource("api/frontend_spec", []restful.ResourceMethod{}),
}

func init() {
	frontendSpecResourceInstance.Resource = frontendSpecResourceInstance
	frontendSpecResourceInstance.Register(dashboard.DashboardResourceRegistrySingleton)
}
