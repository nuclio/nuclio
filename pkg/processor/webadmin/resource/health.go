package resource

import "net/http"

type healthResource struct {
	*abstractResource
}

func (esr *healthResource) getSingle(request *http.Request) (string, attributes) {
	return "processor", attributes{
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
