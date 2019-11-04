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

package platform

import (
	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
)

type HealthCheckMode string

const (
	// health check should be performed by an internal client
	HealthCheckModeInternalClient HealthCheckMode = "internalClient"

	// health check should be performed by an outside entity
	HealthCheckModeExternal = "external"
)

// Platform defines the interface that any underlying function platform must provide for nuclio
// to run over it
type Platform interface {
	//
	// Function
	//

	// Build will locally build a processor image and return its name (or the error)
	CreateFunctionBuild(createFunctionBuildOptions *CreateFunctionBuildOptions) (*CreateFunctionBuildResult, error)

	// Deploy will deploy a processor image to the platform (optionally building it, if source is provided)
	CreateFunction(createFunctionOptions *CreateFunctionOptions) (*CreateFunctionResult, error)

	// UpdateFunction will update a previously deployed function
	UpdateFunction(updateFunctionOptions *UpdateFunctionOptions) error

	// DeleteFunction will delete a previously deployed function
	DeleteFunction(deleteFunctionOptions *DeleteFunctionOptions) error

	// CreateFunctionInvocation will invoke a previously deployed function
	CreateFunctionInvocation(createFunctionInvocationOptions *CreateFunctionInvocationOptions) (*CreateFunctionInvocationResult, error)

	// GetFunctions will list existing functions
	GetFunctions(getFunctionsOptions *GetFunctionsOptions) ([]Function, error)

	// GetDefaultInvokeIPAddresses will return a list of ip addresses to be used by the platform to inovke a function
	GetDefaultInvokeIPAddresses() ([]string, error)

	//
	// Project
	//

	// CreateProject will probably create a new project
	CreateProject(createProjectOptions *CreateProjectOptions) error

	// UpdateProject will update a previously existing project
	UpdateProject(updateProjectOptions *UpdateProjectOptions) error

	// DeleteProject will delete a previously existing project
	DeleteProject(deleteProjectOptions *DeleteProjectOptions) error

	// GetProjects will list existing projects
	GetProjects(getProjectsOptions *GetProjectsOptions) ([]Project, error)

	//
	// Function event
	//

	// CreateFunctionEvent will create a new function event that can later be used as a template from
	// which to invoke functions
	CreateFunctionEvent(createFunctionEventOptions *CreateFunctionEventOptions) error

	// UpdateFunctionEvent will update a previously existing function event
	UpdateFunctionEvent(updateFunctionEventOptions *UpdateFunctionEventOptions) error

	// DeleteFunctionEvent will delete a previously existing function event
	DeleteFunctionEvent(deleteFunctionEventOptions *DeleteFunctionEventOptions) error

	// GetFunctionEvents will list existing function events
	GetFunctionEvents(getFunctionEventsOptions *GetFunctionEventsOptions) ([]FunctionEvent, error)

	//
	// Misc
	//

	// SetExternalIPAddresses configures the IP addresses invocations will use, if "via" is set to "external-ip".
	// If this is not invoked, each platform will try to discover these addresses automatically
	SetExternalIPAddresses(externalIPAddresses []string) error

	// GetExternalIPAddresses returns the external IP addresses invocations will use, if "via" is set to "external-ip".
	// These addresses are either set through SetExternalIPAddresses or automatically discovered
	GetExternalIPAddresses() ([]string, error)

	SetDefaultHTTPIngressHostTemplate(string)

	GetDefaultHTTPIngressHostTemplate() string

	// GetNamespaces returns all the namespaces in the platform
	GetNamespaces() ([]string, error)

	// GetHealthCheckMode returns the healthcheck mode the platform requires
	GetHealthCheckMode() HealthCheckMode

	// GetName returns the platform name
	GetName() string

	// GetNodes returns a slice of nodes currently in the cluster
	GetNodes() ([]Node, error)

	// ResolveDefaultNamespace returns the proper default resource namespace, given the current default namespace
	ResolveDefaultNamespace(string) string

	// BuildAndPushContainerImage builds container image and pushes it into container registry
	BuildAndPushContainerImage(buildOptions *containerimagebuilderpusher.BuildOptions) error

	// Get Onbuild stage for multistage builds
	GetOnbuildStages(onbuildArtifacts []runtime.Artifact) ([]string, error)

	// Change Onbuild artifact paths depending on the type of the builder used
	TransformOnbuildArtifactPaths(onbuildArtifacts []runtime.Artifact) (map[string]string, error)
}
