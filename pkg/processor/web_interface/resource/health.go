package resource

type healthResource struct {
	*abstractResource
}

func (esr *healthResource) getSingle() (string, map[string]interface{}) {
	return "processor", map[string]interface{}{
		"oper_status": "up",
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
