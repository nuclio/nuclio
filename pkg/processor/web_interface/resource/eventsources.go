package resource

type eventSourcesResource struct {
	*abstractResource
}

func (esr *eventSourcesResource) getAll() map[string]map[string]interface{} {
	return map[string]map[string]interface{}{
		"http":      {"name": "foo"},
		"generator": {"name": "moo"},
	}
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
