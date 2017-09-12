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

	"github.com/go-chi/chi"
	"github.com/nuclio/nuclio/pkg/processor/webadmin"
	"github.com/nuclio/nuclio/pkg/restful"
)

type eventSourcesResource struct {
	*resource
}

func (esr *eventSourcesResource) GetAll(request *http.Request) map[string]restful.Attributes {
	eventSources := map[string]restful.Attributes{}

	// iterate over event sources
	// TODO: when this is dynamic (create/delete support), add some locking
	for _, eventSource := range esr.getProcessor().GetEventSources() {
		configuration := eventSource.GetConfig()

		// extract the ID from the configuration (get and remove)
		id := esr.extractIDFromConfiguration(configuration)

		// set the event source with its ID as key
		eventSources[id] = configuration
	}

	return eventSources
}

func (esr *eventSourcesResource) GetByID(request *http.Request, id string) restful.Attributes {
	for _, eventSource := range esr.getProcessor().GetEventSources() {
		configuration := eventSource.GetConfig()

		// extract the ID from the configuration (get and remove)
		eventSourceID := esr.extractIDFromConfiguration(configuration)

		if id == eventSourceID {
			return configuration
		}
	}

	return nil
}

// returns a list of custom routes for the resource
func (esr *eventSourcesResource) GetCustomRoutes() map[string]restful.CustomRoute {

	// just for demonstration. when stats are supported, this will be wired
	return map[string]restful.CustomRoute{
		"/{id}/stats": {http.MethodGet, esr.getStatistics},
	}
}

func (esr *eventSourcesResource) getStatistics(request *http.Request) (string, map[string]restful.Attributes, bool, int, error) {
	resourceID := chi.URLParam(request, "id")

	return "statistics", map[string]restful.Attributes{
		resourceID: {"stats": "example"},
	}, true, http.StatusOK, nil
}

func (esr *eventSourcesResource) extractIDFromConfiguration(configuration map[string]interface{}) string {
	id := configuration["ID"].(string)

	delete(configuration, "ID")

	return id
}

// register the resource
var eventSources = &eventSourcesResource{
	resource: newResource("event_sources", []restful.ResourceMethod{
		restful.ResourceMethodGetList,
		restful.ResourceMethodGetDetail,
	}),
}

func init() {
	eventSources.Resource = eventSources
	eventSources.Register(webadmin.WebAdminResourceRegistrySingleton)
}
