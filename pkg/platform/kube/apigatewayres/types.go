package apigatewayres

import (
	"context"

	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/ingress"
)

type Client interface {

	// List returns the current existing api gateway resources
	List(context.Context, string) ([]Resources, error)

	// Get returns the resources named by name
	Get(context.Context, string, string) (Resources, error)

	// CreateOrUpdate creates or updates existing resources
	CreateOrUpdate(context.Context, *nuclioio.NuclioAPIGateway) (Resources, error)

	// WaitAvailable waits until the resources are ready
	WaitAvailable(context.Context, string, string)

	// Delete deletes resources
	Delete(context.Context, string, string)
}

// Resources holds the resources an apigatewayres holds
type Resources interface {

	// IngressResourcesMap returns a mapping of [ string(ingress's name) -> *ingress.Resources ]
	IngressResourcesMap() map[string]*ingress.Resources
}
