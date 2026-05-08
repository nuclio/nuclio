/*
Copyright 2025 The Nuclio Authors.

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

package nuclio

import (
	"context"

	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Client provides resilient methods to interact with Nuclio CRDs
type Client interface {

	// --- NuclioFunctions ---

	// GetNuclioFunction fetches a specific NuclioFunction by name from a given namespace.
	GetNuclioFunction(ctx context.Context, namespace, name string) (*nuclioio.NuclioFunction, error)

	// ListNuclioFunctions retrieves the list of NuclioFunctions in a given namespace.
	ListNuclioFunctions(ctx context.Context, namespace string, opts metav1.ListOptions) (*nuclioio.NuclioFunctionList, error)

	// CreateNuclioFunction creates a new NuclioFunction in the specified namespace.
	CreateNuclioFunction(ctx context.Context, namespace string, function *nuclioio.NuclioFunction) (*nuclioio.NuclioFunction, error)

	// UpdateNuclioFunction updates an existing NuclioFunction in the specified namespace.
	UpdateNuclioFunction(ctx context.Context, namespace string, function *nuclioio.NuclioFunction) (*nuclioio.NuclioFunction, error)

	// DeleteNuclioFunction removes a NuclioFunction from the specified namespace.
	DeleteNuclioFunction(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error

	// --- NuclioProjects ---

	// GetNuclioProject fetches a specific NuclioProject by name from a given namespace.
	GetNuclioProject(ctx context.Context, namespace, name string) (*nuclioio.NuclioProject, error)

	// ListNuclioProjects retrieves the list of NuclioProjects in a given namespace.
	ListNuclioProjects(ctx context.Context, namespace string, opts metav1.ListOptions) (*nuclioio.NuclioProjectList, error)

	// CreateNuclioProject creates a new NuclioProject in the specified namespace.
	CreateNuclioProject(ctx context.Context, namespace string, project *nuclioio.NuclioProject) (*nuclioio.NuclioProject, error)

	// UpdateNuclioProject updates an existing NuclioProject in the specified namespace.
	UpdateNuclioProject(ctx context.Context, namespace string, project *nuclioio.NuclioProject) (*nuclioio.NuclioProject, error)

	// DeleteNuclioProject removes a NuclioProject from the specified namespace.
	DeleteNuclioProject(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error

	// --- NuclioAPIGateways ---

	// GetNuclioAPIGateway fetches a specific NuclioAPIGateway by name from a given namespace.
	GetNuclioAPIGateway(ctx context.Context, namespace, name string) (*nuclioio.NuclioAPIGateway, error)

	// ListNuclioAPIGateways retrieves the list of NuclioAPIGateways in a given namespace.
	ListNuclioAPIGateways(ctx context.Context, namespace string, opts metav1.ListOptions) (*nuclioio.NuclioAPIGatewayList, error)

	// CreateNuclioAPIGateway creates a new NuclioAPIGateway in the specified namespace.
	CreateNuclioAPIGateway(ctx context.Context, namespace string, apiGateway *nuclioio.NuclioAPIGateway) (*nuclioio.NuclioAPIGateway, error)

	// UpdateNuclioAPIGateway updates an existing NuclioAPIGateway in the specified namespace.
	UpdateNuclioAPIGateway(ctx context.Context, namespace string, apiGateway *nuclioio.NuclioAPIGateway) (*nuclioio.NuclioAPIGateway, error)

	// DeleteNuclioAPIGateway removes a NuclioAPIGateway from the specified namespace.
	DeleteNuclioAPIGateway(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error

	// --- NuclioFunctionEvents ---

	// GetNuclioFunctionEvent fetches a specific NuclioFunctionEvent by name from a given namespace.
	GetNuclioFunctionEvent(ctx context.Context, namespace, name string) (*nuclioio.NuclioFunctionEvent, error)

	// ListNuclioFunctionEvents retrieves the list of NuclioFunctionEvents in a given namespace.
	ListNuclioFunctionEvents(ctx context.Context, namespace string, opts metav1.ListOptions) (*nuclioio.NuclioFunctionEventList, error)

	// CreateNuclioFunctionEvent creates a new NuclioFunctionEvent in the specified namespace.
	CreateNuclioFunctionEvent(ctx context.Context, namespace string, functionEvent *nuclioio.NuclioFunctionEvent) (*nuclioio.NuclioFunctionEvent, error)

	// UpdateNuclioFunctionEvent updates an existing NuclioFunctionEvent in the specified namespace.
	UpdateNuclioFunctionEvent(ctx context.Context, namespace string, functionEvent *nuclioio.NuclioFunctionEvent) (*nuclioio.NuclioFunctionEvent, error)

	// DeleteNuclioFunctionEvent removes a NuclioFunctionEvent from the specified namespace.
	DeleteNuclioFunctionEvent(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error
}
