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

	// UpdateFunctionOptions will update a previously deployed function
	UpdateFunction(updateFunctionOptions *UpdateFunctionOptions) error

	// DeleteFunction will delete a previously deployed function
	DeleteFunction(deleteFunctionOptions *DeleteFunctionOptions) error

	// CreateFunctionInvocation will invoke a previously deployed function
	CreateFunctionInvocation(createFunctionInvocationOptions *CreateFunctionInvocationOptions) (*CreateFunctionInvocationResult, error)

	// CreateFunctionInvocation will invoke a previously deployed function
	GetFunctions(getFunctionsOptions *GetFunctionsOptions) ([]Function, error)

	//
	// Project
	//

	// Deploy will deploy a processor image to the platform (optionally building it, if source is provided)
	CreateProject(createProjectOptions *CreateProjectOptions) error

	// UpdateProjectOptions will update a previously deployed function
	UpdateProject(updateProjectOptions *UpdateProjectOptions) error

	// DeleteProject will delete a previously deployed function
	DeleteProject(deleteProjectOptions *DeleteProjectOptions) error

	// CreateProjectInvocation will invoke a previously deployed function
	GetProjects(getProjectsOptions *GetProjectsOptions) ([]Project, error)

	//
	// Misc
	//

	// SetExternalIPAddresses configures the IP addresses invocations will use, if "via" is set to "external-ip".
	// If this is not invoked, each platform will try to discover these addresses automatically
	SetExternalIPAddresses(externalIPAddresses []string) error

	// GetExternalIPAddresses returns the external IP addresses invocations will use, if "via" is set to "external-ip".
	// These addresses are either set through SetExternalIPAddresses or automatically discovered
	GetExternalIPAddresses() ([]string, error)

	// GetDeployRequiresRegistry returns true if a registry is required for deploy, false otherwise
	GetDeployRequiresRegistry() bool

	// GetName returns the platform name
	GetName() string

	// GetNodes returns a slice of nodes currently in the cluster
	GetNodes() ([]Node, error)
}
