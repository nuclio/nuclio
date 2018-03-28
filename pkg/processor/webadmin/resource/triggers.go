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

	"github.com/nuclio/nuclio/pkg/processor/webadmin"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/go-chi/chi"
)

type triggersResource struct {
	*resource
}

func (tr *triggersResource) GetAll(request *http.Request) (map[string]restful.Attributes, error) {
	triggers := map[string]restful.Attributes{}

	// iterate over triggers
	// TODO: when this is dynamic (create/delete support), add some locking
	for _, trigger := range tr.getProcessor().GetTriggers() {
		configuration := trigger.GetConfig()

		// extract the ID from the configuration (get and remove)
		id := tr.extractIDFromConfiguration(configuration)

		// set the trigger with its ID as key
		triggers[id] = configuration
	}

	return triggers, nil
}

func (tr *triggersResource) GetByID(request *http.Request, id string) (restful.Attributes, error) {
	for _, trigger := range tr.getProcessor().GetTriggers() {
		configuration := trigger.GetConfig()

		// extract the ID from the configuration (get and remove)
		triggerID := tr.extractIDFromConfiguration(configuration)

		if id == triggerID {
			return configuration, nil
		}
	}

	return nil, nil
}

// returns a list of custom routes for the resource
func (tr *triggersResource) GetCustomRoutes() ([]restful.CustomRoute, error) {

	// just for demonstration. when stats are supported, this will be wired
	return []restful.CustomRoute{
		{
			Pattern:   "/{id}/stats",
			Method:    http.MethodGet,
			RouteFunc: tr.getStatistics,
		},
	}, nil
}

func (tr *triggersResource) getStatistics(request *http.Request) (*restful.CustomRouteFuncResponse, error) {
	resourceID := chi.URLParam(request, "id")

	return &restful.CustomRouteFuncResponse{
		ResourceType: "statistics",
		Resources: map[string]restful.Attributes{
			resourceID: {"stats": "example"},
		},
		Single:     true,
		StatusCode: http.StatusOK,
	}, nil
}

func (tr *triggersResource) extractIDFromConfiguration(configuration map[string]interface{}) string {
	id := configuration["ID"].(string)

	delete(configuration, "ID")

	return id
}

// register the resource
var triggers = &triggersResource{
	resource: newResource("triggers", []restful.ResourceMethod{
		restful.ResourceMethodGetList,
		restful.ResourceMethodGetDetail,
	}),
}

func init() {
	triggers.Resource = triggers
	triggers.Register(webadmin.WebAdminResourceRegistrySingleton)
}
