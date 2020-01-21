package test

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
type MockPlatform struct {
	mock.Mock
}

//
// Function
//

// Build will locally build a processor image and return its name (or the error)
func (mp *MockPlatform) CreateFunctionBuild(createFunctionBuildOptions *platform.CreateFunctionBuildOptions) (*platform.CreateFunctionBuildResult, error) {
	args := mp.Called(createFunctionBuildOptions)
	return args.Get(0).(*platform.CreateFunctionBuildResult), args.Error(1)
}

// Deploy will deploy a processor image to the platform (optionally building it, if source is provided)
func (mp *MockPlatform) CreateFunction(createFunctionOptions *platform.CreateFunctionOptions) (*platform.CreateFunctionResult, error) {

	// release requester
	if createFunctionOptions.CreationStateUpdated != nil {
		createFunctionOptions.CreationStateUpdated <- true
	}

	args := mp.Called(createFunctionOptions)
	return args.Get(0).(*platform.CreateFunctionResult), args.Error(1)
}

// UpdateFunction will update a previously deployed function
func (mp *MockPlatform) UpdateFunction(updateFunctionOptions *platform.UpdateFunctionOptions) error {
	args := mp.Called(updateFunctionOptions)
	return args.Error(0)
}

// DeleteFunction will delete a previously deployed function
func (mp *MockPlatform) DeleteFunction(deleteFunctionOptions *platform.DeleteFunctionOptions) error {
	args := mp.Called(deleteFunctionOptions)
	return args.Error(0)
}

// CreateFunctionInvocation will invoke a previously deployed function
func (mp *MockPlatform) CreateFunctionInvocation(createFunctionInvocationOptions *platform.CreateFunctionInvocationOptions) (*platform.CreateFunctionInvocationResult, error) {
	args := mp.Called(createFunctionInvocationOptions)
	return args.Get(0).(*platform.CreateFunctionInvocationResult), args.Error(1)
}

// GetFunctions will list existing functions
func (mp *MockPlatform) GetFunctions(getFunctionsOptions *platform.GetFunctionsOptions) ([]platform.Function, error) {
	args := mp.Called(getFunctionsOptions)
	return args.Get(0).([]platform.Function), args.Error(1)
}

//
// Project
//

// CreateProject will probably create a new project
func (mp *MockPlatform) CreateProject(createProjectOptions *platform.CreateProjectOptions) error {
	args := mp.Called(createProjectOptions)
	return args.Error(0)
}

// UpdateProject will update a previously existing project
func (mp *MockPlatform) UpdateProject(updateProjectOptions *platform.UpdateProjectOptions) error {
	args := mp.Called(updateProjectOptions)
	return args.Error(0)
}

// DeleteProject will delete a previously existing project
func (mp *MockPlatform) DeleteProject(deleteProjectOptions *platform.DeleteProjectOptions) error {
	args := mp.Called(deleteProjectOptions)
	return args.Error(0)
}

// GetProjects will list existing projects
func (mp *MockPlatform) GetProjects(getProjectsOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	args := mp.Called(getProjectsOptions)
	return args.Get(0).([]platform.Project), args.Error(1)
}

//
// Function event
//

// CreateFunctionEvent will create a new function event that can later be used as a template from
// which to invoke functions
func (mp *MockPlatform) CreateFunctionEvent(createFunctionEventOptions *platform.CreateFunctionEventOptions) error {
	args := mp.Called(createFunctionEventOptions)
	return args.Error(0)
}

// UpdateFunctionEvent will update a previously existing function event
func (mp *MockPlatform) UpdateFunctionEvent(updateFunctionEventOptions *platform.UpdateFunctionEventOptions) error {
	args := mp.Called(updateFunctionEventOptions)
	return args.Error(0)
}

// DeleteFunctionEvent will delete a previously existing function event
func (mp *MockPlatform) DeleteFunctionEvent(deleteFunctionEventOptions *platform.DeleteFunctionEventOptions) error {
	args := mp.Called(deleteFunctionEventOptions)
	return args.Error(0)
}

// GetFunctionEvents will list existing function events
func (mp *MockPlatform) GetFunctionEvents(getFunctionEventsOptions *platform.GetFunctionEventsOptions) ([]platform.FunctionEvent, error) {
	args := mp.Called(getFunctionEventsOptions)
	return args.Get(0).([]platform.FunctionEvent), args.Error(1)
}

//
// Misc
//

func (mp *MockPlatform) SetDefaultHTTPIngressHostTemplate(defaultHTTPIngressHostTemplate string) {
	mp.Called(defaultHTTPIngressHostTemplate)
}

func (mp *MockPlatform) GetDefaultHTTPIngressHostTemplate() string {
	args := mp.Called()
	return args.Get(0).(string)
}

func (mp *MockPlatform) SetImageNamePrefixTemplate(imageNamePrefixTemplate string) {
	mp.Called(imageNamePrefixTemplate)
}

func (mp *MockPlatform) GetImageNamePrefixTemplate() string {
	args := mp.Called()
	return args.Get(0).(string)
}

func (mp *MockPlatform) RenderImageNamePrefixTemplate(projectName string, functionName string) (string, error) {
	args := mp.Called()
	return args.Get(0).(string), args.Error(1)
}

// SetExternalIPAddresses configures the IP addresses invocations will use, if "via" is set to "external-ip".
// If this is not invoked, each platform will try to discover these addresses automatically
func (mp *MockPlatform) SetExternalIPAddresses(externalIPAddresses []string) error {
	args := mp.Called(externalIPAddresses)
	return args.Error(0)
}

// GetExternalIPAddresses returns the external IP addresses invocations will use, if "via" is set to "external-ip".
// These addresses are either set through SetExternalIPAddresses or automatically discovered
func (mp *MockPlatform) GetExternalIPAddresses() ([]string, error) {
	args := mp.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (mp *MockPlatform) GetScaleToZeroConfiguration() (*platformconfig.ScaleToZero, error) {
	args := mp.Called()
	return args.Get(0).(*platformconfig.ScaleToZero), args.Error(1)
}

// GetHealthCheckMode returns the healthcheck mode the platform requires
func (mp *MockPlatform) GetHealthCheckMode() platform.HealthCheckMode {
	args := mp.Called()
	return args.Get(0).(platform.HealthCheckMode)
}

// GetName returns the platform name
func (mp *MockPlatform) GetName() string {
	args := mp.Called()
	return args.String(0)
}

// GetNodes returns a slice of nodes currently in the cluster
func (mp *MockPlatform) GetNodes() ([]platform.Node, error) {
	args := mp.Called()
	return args.Get(0).([]platform.Node), args.Error(1)
}

// ResolveDefaultNamespace returns the proper default resource namespace, given the current default namespace
func (mp *MockPlatform) ResolveDefaultNamespace(defaultNamespace string) string {
	args := mp.Called()
	return args.Get(0).(string)
}

// GetNamespaces returns the namespaces
func (mp *MockPlatform) GetNamespaces() ([]string, error) {
	args := mp.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (mp *MockPlatform) GetDefaultInvokeIPAddresses() ([]string, error) {
	args := mp.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (mp *MockPlatform) BuildAndPushContainerImage(buildOptions *containerimagebuilderpusher.BuildOptions) error {
	return nil
}

func (mp *MockPlatform) GetOnbuildStages(onbuildArtifacts []runtime.Artifact) ([]string, error) {
	return []string{}, nil
}

func (mp *MockPlatform) TransformOnbuildArtifactPaths(onbuildArtifacts []runtime.Artifact) (map[string]string, error) {
	return map[string]string{}, nil
}

func (mp *MockPlatform) GetBaseImageRegistry(registry string) string {
	return "quay.io"
}
