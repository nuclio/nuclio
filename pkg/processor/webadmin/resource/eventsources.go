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

func (esr *triggersResource) GetAll(request *http.Request) map[string]restful.Attributes {
	triggers := map[string]restful.Attributes{}

	// iterate over triggers
	// TODO: when this is dynamic (create/delete support), add some locking
	for _, trigger := range esr.getProcessor().GetTriggers() {
		configuration := trigger.GetConfig()

		// extract the ID from the configuration (get and remove)
		id := esr.extractIDFromConfiguration(configuration)

		// set the trigger with its ID as key
		triggers[id] = configuration
	}

	return triggers
}

func (esr *triggersResource) GetByID(request *http.Request, id string) restful.Attributes {
	for _, trigger := range esr.getProcessor().GetTriggers() {
		configuration := trigger.GetConfig()

		// extract the ID from the configuration (get and remove)
		triggerID := esr.extractIDFromConfiguration(configuration)

		if id == triggerID {
			return configuration
		}
	}

	return nil
}

// returns a list of custom routes for the resource
func (esr *triggersResource) GetCustomRoutes() map[string]restful.CustomRoute {

	// just for demonstration. when stats are supported, this will be wired
	return map[string]restful.CustomRoute{
		"/{id}/stats": {
			Method:    http.MethodGet,
			RouteFunc: esr.getStatistics,
		},
	}
}

func (esr *triggersResource) getStatistics(request *http.Request) (string, map[string]restful.Attributes, bool, int, error) {
	resourceID := chi.URLParam(request, "id")

	return "statistics", map[string]restful.Attributes{
		resourceID: {"stats": "example"},
	}, true, http.StatusOK, nil
}

func (esr *triggersResource) extractIDFromConfiguration(configuration map[string]interface{}) string {
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
