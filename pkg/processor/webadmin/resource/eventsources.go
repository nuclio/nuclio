package resource

import "net/http"

type eventSourcesResource struct {
	*abstractResource
}

func (esr *eventSourcesResource) getAll(request *http.Request) map[string]map[string]interface{} {
	eventSources := map[string]map[string]interface{}{}

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

func (esr *eventSourcesResource) getByID(request *http.Request, id string) map[string]interface{} {
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
