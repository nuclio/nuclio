package resource

import (
	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/restful"
	"net/http"
)

type registryKind struct {
	*resource
}

func (rk *registryKind) ExtendMiddlewares() error {
	rk.resource.addAuthMiddleware(nil)
	return nil
}

// GetAll returns registry kind
func (rk *registryKind) GetAll(request *http.Request) (map[string]restful.Attributes, error) {
	registryKindValue := rk.getPlatform().GetRegistryKind()

	response := map[string]restful.Attributes{
		"registryKind": {
			"kind": registryKindValue,
		},
	}

	return response, nil
}

// register the resource
var registryKindResourceInstance = &registryKind{
	resource: newResource("api/registry_kind", []restful.ResourceMethod{
		restful.ResourceMethodGetList,
	}),
}

func init() {
	registryKindResourceInstance.Resource = registryKindResourceInstance
	registryKindResourceInstance.Register(dashboard.DashboardResourceRegistrySingleton)
}
