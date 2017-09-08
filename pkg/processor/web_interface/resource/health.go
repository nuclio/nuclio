package resource

type healthResource struct {
	*abstractResource
}

func (esr *healthResource) getAll() map[string]map[string]interface{} {
	return map[string]map[string]interface{}{
		"processor": {"oper_status": "up"},
	}
}

// register the resource
var health = &healthResource{
	abstractResource: newAbstractInterface("health", []resourceMethod{
		resourceMethodGetList,
	}),
}

func init() {
	health.resource = health
	health.register()
}
