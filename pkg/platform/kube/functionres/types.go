/*
Copyright 2023 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package functionres

import (
	"context"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	appsv1 "k8s.io/api/apps/v1"
	autosv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

type PlatformConfigurationProvider interface {

	// GetPlatformConfiguration returns a platform configuration
	GetPlatformConfiguration() *platformconfig.Config

	// GetPlatformConfigurationName returns platform configuration resource name
	GetPlatformConfigurationName() string
}

type Client interface {

	// List returns the current existing function resources, populating the "deployment" sub-resource
	List(context.Context, string) ([]Resources, error)

	// Get returns the resources named by name, populating the "deployment" sub-resource
	Get(context.Context, string, string) (Resources, error)

	// CreateOrUpdate creates or updates existing resources
	CreateOrUpdate(context.Context, *nuclioio.NuclioFunction, string) (Resources, error)

	// WaitAvailable waits until the resources are ready
	WaitAvailable(context.Context, *nuclioio.NuclioFunction, time.Time) (error, functionconfig.FunctionState)

	// Delete deletes resources
	Delete(context.Context, string, string) error

	// SetPlatformConfigurationProvider sets the provider of the platform configuration for any future access
	SetPlatformConfigurationProvider(PlatformConfigurationProvider)

	// UpdateFunctionSelectorWhenScaleFromZero updates node selector when scaling from zero when function is ready
	UpdateFunctionSelectorWhenScaleFromZero(ctx context.Context, function *nuclioio.NuclioFunction) error
}

// Resources holds the resources a functionres holds
type Resources interface {

	// Deployment returns the deployment
	Deployment() (*appsv1.Deployment, error)

	// ConfigMap returns the configmap
	ConfigMap() (*v1.ConfigMap, error)

	// Service returns the service
	Service() (*v1.Service, error)

	// HorizontalPodAutoscaler returns the hpa
	HorizontalPodAutoscaler() (*autosv2.HorizontalPodAutoscaler, error)

	// Ingress returns the ingress
	Ingress() (*networkingv1.Ingress, error)

	// CronJobs returns the cron job
	CronJobs() ([]*batchv1.CronJob, error)
}
