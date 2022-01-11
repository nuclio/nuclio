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
	"bufio"
	"context"
	"io"
	"time"

	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/opa"
	"github.com/nuclio/nuclio/pkg/platformconfig"
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

	// Initialize Initializes the platform
	Initialize(ctx context.Context) error

	//
	// Function
	//

	// CreateFunctionBuild Build will locally build a processor image and return its name (or the error)
	CreateFunctionBuild(createFunctionBuildOptions *CreateFunctionBuildOptions) (*CreateFunctionBuildResult, error)

	// CreateFunction Deploy will deploy a processor image to the platform (optionally building it, if source is provided)
	CreateFunction(ctx context.Context, createFunctionOptions *CreateFunctionOptions) (*CreateFunctionResult, error)

	// EnrichFunctionConfig Enrich function config upon creating function
	EnrichFunctionConfig(ctx context.Context, functionConfig *functionconfig.Config) error

	// ValidateFunctionConfig Validate function config upon creating function
	ValidateFunctionConfig(ctx context.Context, functionConfig *functionconfig.Config) error

	// UpdateFunction will update a previously deployed function
	UpdateFunction(ctx context.Context, updateFunctionOptions *UpdateFunctionOptions) error

	// DeleteFunction will delete a previously deployed function
	DeleteFunction(ctx context.Context, deleteFunctionOptions *DeleteFunctionOptions) error

	// CreateFunctionInvocation will invoke a previously deployed function
	CreateFunctionInvocation(ctx context.Context, createFunctionInvocationOptions *CreateFunctionInvocationOptions) (*CreateFunctionInvocationResult, error)

	// GetFunctions will list existing functions
	GetFunctions(ctx context.Context, getFunctionsOptions *GetFunctionsOptions) ([]Function, error)

	// FilterFunctionsByPermissions will filter out some functions
	FilterFunctionsByPermissions(context.Context, *opa.PermissionOptions, []Function) ([]Function, error)

	// GetDefaultInvokeIPAddresses will return a list of ip addresses to be used by the platform to invoke a function
	GetDefaultInvokeIPAddresses() ([]string, error)

	// GetFunctionReplicaLogsStream return the function instance (Kubernetes - Pod / Docker - Container) logs stream
	GetFunctionReplicaLogsStream(context.Context, *GetFunctionReplicaLogsStreamOptions) (io.ReadCloser, error)

	// GetFunctionReplicaNames returns function replica names (Pod / Container names)
	GetFunctionReplicaNames(context.Context, *functionconfig.Config) ([]string, error)

	//
	// Project
	//

	// CreateProject will probably create a new project
	CreateProject(ctx context.Context, createProjectOptions *CreateProjectOptions) error

	// UpdateProject will update a previously existing project
	UpdateProject(ctx context.Context, updateProjectOptions *UpdateProjectOptions) error

	// DeleteProject will delete a previously existing project
	DeleteProject(ctx context.Context, deleteProjectOptions *DeleteProjectOptions) error

	// GetProjects will list existing projects
	GetProjects(ctx context.Context, getProjectsOptions *GetProjectsOptions) ([]Project, error)

	// EnsureDefaultProjectExistence ensure default project exists, creates it otherwise
	EnsureDefaultProjectExistence(ctx context.Context) error

	// WaitForProjectResourcesDeletion waits for all of the project's resources to be deleted
	WaitForProjectResourcesDeletion(ctx context.Context, projectMeta *ProjectMeta, duration time.Duration) error

	//
	// Function event
	//

	// CreateFunctionEvent will create a new function event that can later be used as a template from
	// which to invoke functions
	CreateFunctionEvent(ctx context.Context, createFunctionEventOptions *CreateFunctionEventOptions) error

	// UpdateFunctionEvent will update a previously existing function event
	UpdateFunctionEvent(ctx context.Context, updateFunctionEventOptions *UpdateFunctionEventOptions) error

	// DeleteFunctionEvent will delete a previously existing function event
	DeleteFunctionEvent(ctx context.Context, deleteFunctionEventOptions *DeleteFunctionEventOptions) error

	// GetFunctionEvents will list existing function events
	GetFunctionEvents(ctx context.Context, getFunctionEventsOptions *GetFunctionEventsOptions) ([]FunctionEvent, error)

	// FilterFunctionEventsByPermissions will filter out some function events
	FilterFunctionEventsByPermissions(context.Context, *opa.PermissionOptions, []FunctionEvent) ([]FunctionEvent, error)

	//
	// API Gateway
	//

	// CreateAPIGateway creates and deploy APIGateway
	CreateAPIGateway(ctx context.Context, createAPIGatewayOptions *CreateAPIGatewayOptions) error

	// UpdateAPIGateway will update a previously deployed api gateway
	UpdateAPIGateway(ctx context.Context, updateAPIGatewayOptions *UpdateAPIGatewayOptions) error

	// DeleteAPIGateway will delete a previously deployed api gateway
	DeleteAPIGateway(ctx context.Context, deleteAPIGatewayOptions *DeleteAPIGatewayOptions) error

	// GetAPIGateways will list existing api gateways
	GetAPIGateways(ctx context.Context, getAPIGatewaysOptions *GetAPIGatewaysOptions) ([]APIGateway, error)

	//
	// Misc
	//

	// SetExternalIPAddresses configures the IP addresses invocations will use, if "via" is set to "external-ip".
	// If this is not invoked, each platform will try to discover these addresses automatically
	SetExternalIPAddresses(externalIPAddresses []string) error

	// GetExternalIPAddresses returns the external IP addresses invocations will use, if "via" is set to "external-ip".
	// These addresses are either set through SetExternalIPAddresses or automatically discovered
	GetExternalIPAddresses() ([]string, error)

	SetImageNamePrefixTemplate(string)

	GetImageNamePrefixTemplate() string

	RenderImageNamePrefixTemplate(projectName string, functionName string) (string, error)

	// GetScaleToZeroConfiguration returns scale to zero configuration
	GetScaleToZeroConfiguration() *platformconfig.ScaleToZero

	// GetAllowedAuthenticationModes returns allowed authentication modes
	GetAllowedAuthenticationModes() []string

	// GetNamespaces returns all the namespaces in the platform
	GetNamespaces(context.Context) ([]string, error)

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

	// GetOnbuildStages Get Onbuild stage for multistage builds
	GetOnbuildStages(onbuildArtifacts []runtime.Artifact) ([]string, error)

	// TransformOnbuildArtifactPaths Change Onbuild artifact paths depending on the type of the builder used
	TransformOnbuildArtifactPaths(onbuildArtifacts []runtime.Artifact) (map[string]string, error)

	// GetOnbuildImageRegistry returns onbuild base registry
	GetOnbuildImageRegistry(registry string, runtime runtime.Runtime) (string, error)

	// GetBaseImageRegistry returns base image registry
	GetBaseImageRegistry(registry string, runtime runtime.Runtime) (string, error)

	// GetDefaultRegistryCredentialsSecretName returns secret with credentials to push/pull from docker registry
	GetDefaultRegistryCredentialsSecretName() string

	// SaveFunctionDeployLogs Save build logs from platform logger to function store or k8s
	SaveFunctionDeployLogs(ctx context.Context, functionName, namespace string) error

	// GetProcessorLogsAndBriefError Parse and construct a function processor logs and brief error
	GetProcessorLogsAndBriefError(scanner *bufio.Scanner) (string, string)

	// GetContainerBuilderKind returns the container-builder kind
	GetContainerBuilderKind() string

	// GetRuntimeBuildArgs returns the runtime specific build arguments
	GetRuntimeBuildArgs(runtime runtime.Runtime) map[string]string

	// GetConfig returns platform config
	GetConfig() *platformconfig.Config

	//
	// OPA
	//

	// QueryOPAFunctionPermissions queries opa permissions for a certain function
	QueryOPAFunctionPermissions(projectName, functionName string, action opa.Action, permissionOptions *opa.PermissionOptions) (bool, error)
}
