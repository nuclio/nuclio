package mock

import (
	"bufio"
	"context"
	"io"
	"time"

	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/opa"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"

	"github.com/stretchr/testify/mock"
)

//
// Platform mock
//

// Platform defines the interface that any underlying function platform must provide for nuclio
// to run over it
type Platform struct {
	mock.Mock
}

//
// Function
//

func (mp *Platform) GetConfig() *platformconfig.Config {
	args := mp.Called()
	return args.Get(0).(*platformconfig.Config)
}

func (mp *Platform) GetContainerBuilderKind() string {
	return "docker"
}

// Build will locally build a processor image and return its name (or the error)
func (mp *Platform) CreateFunctionBuild(createFunctionBuildOptions *platform.CreateFunctionBuildOptions) (*platform.CreateFunctionBuildResult, error) {
	args := mp.Called(createFunctionBuildOptions)
	return args.Get(0).(*platform.CreateFunctionBuildResult), args.Error(1)
}

// Deploy will deploy a processor image to the platform (optionally building it, if source is provided)
func (mp *Platform) CreateFunction(ctx context.Context, createFunctionOptions *platform.CreateFunctionOptions) (*platform.CreateFunctionResult, error) {

	// release requester
	if createFunctionOptions.CreationStateUpdated != nil {
		createFunctionOptions.CreationStateUpdated <- true
	}

	args := mp.Called(ctx, createFunctionOptions)
	return args.Get(0).(*platform.CreateFunctionResult), args.Error(1)
}

func (mp *Platform) EnrichFunctionConfig(ctx context.Context, functionConfig *functionconfig.Config) error {
	args := mp.Called(ctx, functionConfig)
	return args.Error(0)
}

func (mp *Platform) ValidateFunctionConfig(ctx context.Context, functionConfig *functionconfig.Config) error {
	args := mp.Called(ctx, functionConfig)
	return args.Error(0)
}

// UpdateFunction will update a previously deployed function
func (mp *Platform) UpdateFunction(ctx context.Context, updateFunctionOptions *platform.UpdateFunctionOptions) error {
	args := mp.Called(ctx, updateFunctionOptions)
	return args.Error(0)
}

// DeleteFunction will delete a previously deployed function
func (mp *Platform) DeleteFunction(ctx context.Context, deleteFunctionOptions *platform.DeleteFunctionOptions) error {
	args := mp.Called(ctx, deleteFunctionOptions)
	return args.Error(0)
}

// CreateFunctionInvocation will invoke a previously deployed function
func (mp *Platform) CreateFunctionInvocation(ctx context.Context, createFunctionInvocationOptions *platform.CreateFunctionInvocationOptions) (*platform.CreateFunctionInvocationResult, error) {
	args := mp.Called(ctx, createFunctionInvocationOptions)
	return args.Get(0).(*platform.CreateFunctionInvocationResult), args.Error(1)
}

// GetFunctions will list existing functions
func (mp *Platform) GetFunctions(ctx context.Context, getFunctionsOptions *platform.GetFunctionsOptions) ([]platform.Function, error) {
	args := mp.Called(ctx, getFunctionsOptions)
	return args.Get(0).([]platform.Function), args.Error(1)
}

func (mp *Platform) FilterFunctionsByPermissions(ctx context.Context,
	permissionOptions *opa.PermissionOptions,
	functions []platform.Function) ([]platform.Function, error) {
	args := mp.Called(ctx, permissionOptions, functions)
	return args.Get(0).([]platform.Function), args.Error(1)
}

// GetFunctionReplicaLogsStream return the function instance (Kubernetes - Pod / Docker - Container) logs stream
func (mp *Platform) GetFunctionReplicaLogsStream(ctx context.Context, options *platform.GetFunctionReplicaLogsStreamOptions) (io.ReadCloser, error) {
	args := mp.Called(ctx, options)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

// GetFunctionReplicaNames returns function replica names (Pod / Container names)
func (mp *Platform) GetFunctionReplicaNames(ctx context.Context, functionConfig *functionconfig.Config) ([]string, error) {
	args := mp.Called(ctx, functionConfig)
	return args.Get(0).([]string), args.Error(1)
}

//
// Project
//

// CreateProject will probably create a new project
func (mp *Platform) CreateProject(ctx context.Context, createProjectOptions *platform.CreateProjectOptions) error {
	args := mp.Called(ctx, createProjectOptions)
	return args.Error(0)
}

// UpdateProject will update a previously existing project
func (mp *Platform) UpdateProject(ctx context.Context, updateProjectOptions *platform.UpdateProjectOptions) error {
	args := mp.Called(ctx, updateProjectOptions)
	return args.Error(0)
}

// DeleteProject will delete a previously existing project
func (mp *Platform) DeleteProject(ctx context.Context, deleteProjectOptions *platform.DeleteProjectOptions) error {
	args := mp.Called(ctx, deleteProjectOptions)
	return args.Error(0)
}

// GetProjects will list existing projects
func (mp *Platform) GetProjects(ctx context.Context, getProjectsOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	args := mp.Called(ctx, getProjectsOptions)
	return args.Get(0).([]platform.Project), args.Error(1)
}

func (mp *Platform) GetRuntimeBuildArgs(runtime runtime.Runtime) map[string]string {
	args := mp.Called()
	return args.Get(0).(map[string]string)
}

//
// API Gateway
//

// CreateAPIGateway creates and deploys a new api gateway
func (mp *Platform) CreateAPIGateway(ctx context.Context, createAPIGatewayOptions *platform.CreateAPIGatewayOptions) error {
	args := mp.Called(ctx, createAPIGatewayOptions)
	return args.Error(0)
}

// UpdateAPIGateway will update a previously deployed api gateway
func (mp *Platform) UpdateAPIGateway(ctx context.Context, updateAPIGatewayOptions *platform.UpdateAPIGatewayOptions) error {
	args := mp.Called(ctx, updateAPIGatewayOptions)
	return args.Error(0)
}

// DeleteAPIGateway will delete a previously deployed api gateway
func (mp *Platform) DeleteAPIGateway(ctx context.Context, deleteAPIGatewayOptions *platform.DeleteAPIGatewayOptions) error {
	args := mp.Called(ctx, deleteAPIGatewayOptions)
	return args.Error(0)
}

// GetAPIGateways will list existing api gateways
func (mp *Platform) GetAPIGateways(ctx context.Context, getAPIGatewaysOptions *platform.GetAPIGatewaysOptions) ([]platform.APIGateway, error) {
	args := mp.Called(ctx, getAPIGatewaysOptions)
	return args.Get(0).([]platform.APIGateway), args.Error(1)
}

//
// Function event
//

// CreateFunctionEvent will create a new function event that can later be used as a template from
// which to invoke functions
func (mp *Platform) CreateFunctionEvent(ctx context.Context, createFunctionEventOptions *platform.CreateFunctionEventOptions) error {
	args := mp.Called(ctx, createFunctionEventOptions)
	return args.Error(0)
}

// UpdateFunctionEvent will update a previously existing function event
func (mp *Platform) UpdateFunctionEvent(ctx context.Context, updateFunctionEventOptions *platform.UpdateFunctionEventOptions) error {
	args := mp.Called(ctx, updateFunctionEventOptions)
	return args.Error(0)
}

// DeleteFunctionEvent will delete a previously existing function event
func (mp *Platform) DeleteFunctionEvent(ctx context.Context, deleteFunctionEventOptions *platform.DeleteFunctionEventOptions) error {
	args := mp.Called(ctx, deleteFunctionEventOptions)
	return args.Error(0)
}

// GetFunctionEvents will list existing function events
func (mp *Platform) GetFunctionEvents(ctx context.Context, getFunctionEventsOptions *platform.GetFunctionEventsOptions) ([]platform.FunctionEvent, error) {
	args := mp.Called(ctx, getFunctionEventsOptions)
	return args.Get(0).([]platform.FunctionEvent), args.Error(1)
}

func (mp *Platform) FilterFunctionEventsByPermissions(ctx context.Context,
	permissionOptions *opa.PermissionOptions,
	functionEvents []platform.FunctionEvent) ([]platform.FunctionEvent, error) {
	args := mp.Called(ctx, permissionOptions, functionEvents)
	return args.Get(0).([]platform.FunctionEvent), args.Error(1)
}

//
// Misc
//

func (mp *Platform) SetImageNamePrefixTemplate(imageNamePrefixTemplate string) {
	mp.Called(imageNamePrefixTemplate)
}

func (mp *Platform) GetImageNamePrefixTemplate() string {
	args := mp.Called()
	return args.Get(0).(string)
}

func (mp *Platform) RenderImageNamePrefixTemplate(projectName string, functionName string) (string, error) {
	args := mp.Called()
	return args.Get(0).(string), args.Error(1)
}

// SetExternalIPAddresses configures the IP addresses invocations will use, if "via" is set to "external-ip".
// If this is not invoked, each platform will try to discover these addresses automatically
func (mp *Platform) SetExternalIPAddresses(externalIPAddresses []string) error {
	args := mp.Called(externalIPAddresses)
	return args.Error(0)
}

// GetExternalIPAddresses returns the external IP addresses invocations will use, if "via" is set to "external-ip".
// These addresses are either set through SetExternalIPAddresses or automatically discovered
func (mp *Platform) GetExternalIPAddresses() ([]string, error) {
	args := mp.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (mp *Platform) GetScaleToZeroConfiguration() *platformconfig.ScaleToZero {
	args := mp.Called()
	return args.Get(0).(*platformconfig.ScaleToZero)
}

func (mp *Platform) GetAllowedAuthenticationModes() []string {
	args := mp.Called()
	return args.Get(0).([]string)
}

// GetHealthCheckMode returns the healthcheck mode the platform requires
func (mp *Platform) GetHealthCheckMode() platform.HealthCheckMode {
	args := mp.Called()
	return args.Get(0).(platform.HealthCheckMode)
}

// GetName returns the platform name
func (mp *Platform) GetName() string {
	args := mp.Called()
	return args.String(0)
}

// GetNodes returns a slice of nodes currently in the cluster
func (mp *Platform) GetNodes() ([]platform.Node, error) {
	args := mp.Called()
	return args.Get(0).([]platform.Node), args.Error(1)
}

// ResolveDefaultNamespace returns the proper default resource namespace, given the current default namespace
func (mp *Platform) ResolveDefaultNamespace(defaultNamespace string) string {
	args := mp.Called()
	return args.Get(0).(string)
}

// GetNamespaces returns the namespaces
func (mp *Platform) GetNamespaces(ctx context.Context) ([]string, error) {
	args := mp.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func (mp *Platform) GetDefaultInvokeIPAddresses() ([]string, error) {
	args := mp.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (mp *Platform) BuildAndPushContainerImage(buildOptions *containerimagebuilderpusher.BuildOptions) error {
	return nil
}

func (mp *Platform) GetOnbuildStages(onbuildArtifacts []runtime.Artifact) ([]string, error) {
	return []string{}, nil
}

func (mp *Platform) TransformOnbuildArtifactPaths(onbuildArtifacts []runtime.Artifact) (map[string]string, error) {
	return map[string]string{}, nil
}

func (mp *Platform) GetBaseImageRegistry(registry string, runtime runtime.Runtime) (string, error) {
	return "quay.io", nil
}

func (mp *Platform) GetOnbuildImageRegistry(registry string, runtime runtime.Runtime) (string, error) {
	return "", nil
}

func (mp *Platform) GetDefaultRegistryCredentialsSecretName() string {
	return "nuclio-registry-credentials"
}

func (mp *Platform) SaveFunctionDeployLogs(ctx context.Context, functionName, namespace string) error {
	return nil
}

func (mp *Platform) Initialize(ctx context.Context) error {
	return nil
}

func (mp *Platform) EnsureDefaultProjectExistence(ctx context.Context) error {
	return nil
}

func (mp *Platform) GetProcessorLogsAndBriefError(scanner *bufio.Scanner) (string, string) {
	return "", ""
}

func (mp *Platform) WaitForProjectResourcesDeletion(ctx context.Context, projectMeta *platform.ProjectMeta, duration time.Duration) error {
	return nil
}

func (mp *Platform) QueryOPAFunctionPermissions(projectName,
	functionName string,
	action opa.Action,
	permissionOptions *opa.PermissionOptions) (bool, error) {
	args := mp.Called(projectName, functionName, action, permissionOptions)
	return args.Get(0).(bool), args.Error(1)
}

func (mp *Platform) QueryOPAFunctionEventPermissions(projectName,
	functionName,
	functionEventName string,
	action opa.Action,
	permissionOptions *opa.PermissionOptions) (bool, error) {
	args := mp.Called(projectName, functionName, functionEventName, action, permissionOptions)
	return args.Get(0).(bool), args.Error(1)
}
