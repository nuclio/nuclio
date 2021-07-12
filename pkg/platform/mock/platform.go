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

func (mp *Platform) GetContainerBuilderKind() string {
	return "docker"
}

// Build will locally build a processor image and return its name (or the error)
func (mp *Platform) CreateFunctionBuild(createFunctionBuildOptions *platform.CreateFunctionBuildOptions) (*platform.CreateFunctionBuildResult, error) {
	args := mp.Called(createFunctionBuildOptions)
	return args.Get(0).(*platform.CreateFunctionBuildResult), args.Error(1)
}

// Deploy will deploy a processor image to the platform (optionally building it, if source is provided)
func (mp *Platform) CreateFunction(createFunctionOptions *platform.CreateFunctionOptions) (*platform.CreateFunctionResult, error) {

	// release requester
	if createFunctionOptions.CreationStateUpdated != nil {
		createFunctionOptions.CreationStateUpdated <- true
	}

	args := mp.Called(createFunctionOptions)
	return args.Get(0).(*platform.CreateFunctionResult), args.Error(1)
}

func (mp *Platform) EnrichFunctionConfig(functionConfig *functionconfig.Config) error {
	args := mp.Called(functionConfig)
	return args.Error(0)
}

func (mp *Platform) ValidateFunctionConfig(functionConfig *functionconfig.Config) error {
	args := mp.Called(functionConfig)
	return args.Error(0)
}

// UpdateFunction will update a previously deployed function
func (mp *Platform) UpdateFunction(updateFunctionOptions *platform.UpdateFunctionOptions) error {
	args := mp.Called(updateFunctionOptions)
	return args.Error(0)
}

// DeleteFunction will delete a previously deployed function
func (mp *Platform) DeleteFunction(deleteFunctionOptions *platform.DeleteFunctionOptions) error {
	args := mp.Called(deleteFunctionOptions)
	return args.Error(0)
}

// CreateFunctionInvocation will invoke a previously deployed function
func (mp *Platform) CreateFunctionInvocation(createFunctionInvocationOptions *platform.CreateFunctionInvocationOptions) (*platform.CreateFunctionInvocationResult, error) {
	args := mp.Called(createFunctionInvocationOptions)
	return args.Get(0).(*platform.CreateFunctionInvocationResult), args.Error(1)
}

// GetFunctions will list existing functions
func (mp *Platform) GetFunctions(getFunctionsOptions *platform.GetFunctionsOptions) ([]platform.Function, error) {
	args := mp.Called(getFunctionsOptions)
	return args.Get(0).([]platform.Function), args.Error(1)
}

func (mp *Platform) FilterFunctionsByPermissions(permissionOptions *opa.PermissionOptions,
	functions []platform.Function) ([]platform.Function, error) {
	args := mp.Called(permissionOptions, functions)
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
func (mp *Platform) CreateProject(createProjectOptions *platform.CreateProjectOptions) error {
	args := mp.Called(createProjectOptions)
	return args.Error(0)
}

// UpdateProject will update a previously existing project
func (mp *Platform) UpdateProject(updateProjectOptions *platform.UpdateProjectOptions) error {
	args := mp.Called(updateProjectOptions)
	return args.Error(0)
}

// DeleteProject will delete a previously existing project
func (mp *Platform) DeleteProject(deleteProjectOptions *platform.DeleteProjectOptions) error {
	args := mp.Called(deleteProjectOptions)
	return args.Error(0)
}

// GetProjects will list existing projects
func (mp *Platform) GetProjects(getProjectsOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	args := mp.Called(getProjectsOptions)
	return args.Get(0).([]platform.Project), args.Error(1)
}

func (mp *Platform) GetRuntimeBuildArgs(runtime runtime.Runtime) map[string]string {
	args := mp.Called()
	return args.Get(0).(map[string]string)
}

//
// API Gateway
//

// Create APIGateway creates and deploys a new api gateway
func (mp *Platform) CreateAPIGateway(createAPIGatewayOptions *platform.CreateAPIGatewayOptions) error {
	args := mp.Called(createAPIGatewayOptions)
	return args.Error(0)
}

// UpdateAPIGateway will update a previously deployed api gateway
func (mp *Platform) UpdateAPIGateway(updateAPIGatewayOptions *platform.UpdateAPIGatewayOptions) error {
	args := mp.Called(updateAPIGatewayOptions)
	return args.Error(0)
}

// DeleteAPIGateway will delete a previously deployed api gateway
func (mp *Platform) DeleteAPIGateway(deleteAPIGatewayOptions *platform.DeleteAPIGatewayOptions) error {
	args := mp.Called(deleteAPIGatewayOptions)
	return args.Error(0)
}

// GetAPIGateways will list existing api gateways
func (mp *Platform) GetAPIGateways(getAPIGatewaysOptions *platform.GetAPIGatewaysOptions) ([]platform.APIGateway, error) {
	args := mp.Called(getAPIGatewaysOptions)
	return args.Get(0).([]platform.APIGateway), args.Error(1)
}

//
// Function event
//

// CreateFunctionEvent will create a new function event that can later be used as a template from
// which to invoke functions
func (mp *Platform) CreateFunctionEvent(createFunctionEventOptions *platform.CreateFunctionEventOptions) error {
	args := mp.Called(createFunctionEventOptions)
	return args.Error(0)
}

// UpdateFunctionEvent will update a previously existing function event
func (mp *Platform) UpdateFunctionEvent(updateFunctionEventOptions *platform.UpdateFunctionEventOptions) error {
	args := mp.Called(updateFunctionEventOptions)
	return args.Error(0)
}

// DeleteFunctionEvent will delete a previously existing function event
func (mp *Platform) DeleteFunctionEvent(deleteFunctionEventOptions *platform.DeleteFunctionEventOptions) error {
	args := mp.Called(deleteFunctionEventOptions)
	return args.Error(0)
}

// GetFunctionEvents will list existing function events
func (mp *Platform) GetFunctionEvents(getFunctionEventsOptions *platform.GetFunctionEventsOptions) ([]platform.FunctionEvent, error) {
	args := mp.Called(getFunctionEventsOptions)
	return args.Get(0).([]platform.FunctionEvent), args.Error(1)
}

func (mp *Platform) FilterFunctionEventsByPermissions(permissionOptions *opa.PermissionOptions,
	functionEvents []platform.FunctionEvent) ([]platform.FunctionEvent, error) {
	args := mp.Called(permissionOptions, functionEvents)
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
func (mp *Platform) GetNamespaces() ([]string, error) {
	args := mp.Called()
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

func (mp *Platform) SaveFunctionDeployLogs(functionName, namespace string) error {
	return nil
}

func (mp *Platform) Initialize() error {
	return nil
}

func (mp *Platform) EnsureDefaultProjectExistence() error {
	return nil
}

func (mp *Platform) GetProcessorLogsAndBriefError(scanner *bufio.Scanner) (string, string) {
	return "", ""
}

func (mp *Platform) WaitForProjectResourcesDeletion(projectMeta *platform.ProjectMeta, duration time.Duration) error {
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
