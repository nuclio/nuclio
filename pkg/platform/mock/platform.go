package mock

import (
	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher"
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

//
// Misc
//

func (mp *Platform) SetDefaultHTTPIngressHostTemplate(defaultHTTPIngressHostTemplate string) {
	mp.Called(defaultHTTPIngressHostTemplate)
}

func (mp *Platform) GetDefaultHTTPIngressHostTemplate() string {
	args := mp.Called()
	return args.Get(0).(string)
}

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

func (mp *Platform) GetScaleToZeroConfiguration() (*platformconfig.ScaleToZero, error) {
	args := mp.Called()
	return args.Get(0).(*platformconfig.ScaleToZero), args.Error(1)
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

func (mp *Platform) GetBaseImageRegistry(registry string) string {
	return "quay.io"
}

func (mp *Platform) GetOnbuildImageRegistry(registry string) string {
	return ""
}

func (mp *Platform) GetDefaultRegistryCredentialsSecretName() string {
	return "nuclio-registry-credentials"
}

func (mp *Platform) SaveFunctionDeployLogs(functionName, namespace string) error {
	return nil
}
