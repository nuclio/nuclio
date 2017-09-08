package resource

type eventSourcesResource struct {
	*abstractResource
}

func (esr *eventSourcesResource) getAll() map[string]map[string]interface{} {
	eventSources := map[string]map[string]interface{}{}

	// iterate over event sources
	// TODO: when this is dynamic (create/delete support), add some locking
	for _, eventSource := range esr.processor.GetEventProcessor() {
		eventSources[eventSource.GetKind()] = eventSource.GetConfig()

		// remove ID, we don't need to display it since its encoded above the attributes
		delete(eventSources[eventSource.GetKind()], "ID")
	}

	return eventSources
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
