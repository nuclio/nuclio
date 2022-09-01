/*
Copyright 2017 The Nuclio Authors.

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
