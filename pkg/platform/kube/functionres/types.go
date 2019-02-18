package functionres

import (
	"context"

	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	apps_v1beta1 "k8s.io/api/apps/v1beta1"
	autos_v2 "k8s.io/api/autoscaling/v2beta1"
	"k8s.io/api/core/v1"
	ext_v1beta1 "k8s.io/api/extensions/v1beta1"
)

type PlatformConfigurationProvider interface {

	// GetPlatformConfiguration returns a platform configuration
	GetPlatformConfiguration() *platformconfig.Config
}

type Client interface {

	// List returns the current existing function resources, populating the "deployment" sub-resource
	List(context.Context, string) ([]Resources, error)

	// Get returns the resources named by name, populating the "deployment" sub-resource
	Get(context.Context, string, string) (Resources, error)

	// CreateOrUpdate creates or updates exisisting resources
	CreateOrUpdate(context.Context, *nuclioio.NuclioFunction, string) (Resources, error)

	// WaitAvailable waits until the resources are ready
	WaitAvailable(context.Context, string, string) error

	// Delete deletes resources
	Delete(context.Context, string, string) error

	// SetPlatformConfigurationProvider sets the provider of the platform configuration for any future access
	SetPlatformConfigurationProvider(PlatformConfigurationProvider)
}

// Resources holds the resources a functionres holds
type Resources interface {

	// Deployment returns the deployment
	Deployment() (*apps_v1beta1.Deployment, error)

	// ConfigMap returns the configmap
	ConfigMap() (*v1.ConfigMap, error)

	// Service returns the service
	Service() (*v1.Service, error)

	// HorizontalPodAutoscaler returns the hpa
	HorizontalPodAutoscaler() (*autos_v2.HorizontalPodAutoscaler, error)

	// Ingress returns the ingress
	Ingress() (*ext_v1beta1.Ingress, error)
}
