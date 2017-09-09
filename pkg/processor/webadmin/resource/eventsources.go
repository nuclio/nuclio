package resource

import (
	"net/http"

	"github.com/go-chi/chi"
)

type eventSourcesResource struct {
	*abstractResource
}

func (esr *eventSourcesResource) getAll(request *http.Request) map[string]attributes {
	eventSources := map[string]attributes{}

	// iterate over event sources
	// TODO: when this is dynamic (create/delete support), add some locking
	for _, eventSource := range esr.processor.GetEventSources() {
		configuration := eventSource.GetConfig()

		// extract the ID from the configuration (get and remove)
		id := esr.extractIDFromConfiguration(configuration)

		// set the event source with its ID as key
		eventSources[id] = configuration
	}

	return eventSources
}

func (esr *eventSourcesResource) getByID(request *http.Request, id string) attributes {
	for _, eventSource := range esr.processor.GetEventSources() {
		configuration := eventSource.GetConfig()

		// extract the ID from the configuration (get and remove)
		eventSourceID := esr.extractIDFromConfiguration(configuration)

		if id == eventSourceID {
			return configuration
		}
	}

	return nil
}

func (esr *eventSourcesResource) getStatistics(request *http.Request) (string, map[string]attributes, bool, error) {
	resourceID := chi.URLParam(request, "id")

	return "statistics", map[string]attributes{
		resourceID: {"stats": "example"},
	}, true, nil
}

// returns a list of custom routes for the resource
func (esr *eventSourcesResource) getCustomRoutes() map[string]customRoute {

	// just for demonstration. when stats are supported, this will be wired
	return map[string]customRoute{
		"/{id}/stats": {http.MethodGet, esr.getStatistics},
	}
}

func (esr *eventSourcesResource) extractIDFromConfiguration(configuration map[string]interface{}) string {
	id := configuration["ID"].(string)

	delete(configuration, "ID")

	return id
}

// register the resource
var eventSources = &eventSourcesResource{
	abstractResource: newAbstractInterface("event_sources", []resourceMethod{
		resourceMethodGetList,
		resourceMethodGetDetail,
	}),
}

func init() {
	eventSources.resource = eventSources
	eventSources.register()
}
