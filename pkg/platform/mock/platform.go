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

// Build will locally build a processor image and return its name (or the error)
func (p *Platform) CreateFunctionBuild(createFunctionBuildOptions *platform.CreateFunctionBuildOptions) (*platform.CreateFunctionBuildResult, error) {
	args := p.Called(createFunctionBuildOptions)
	return args.Get(0).(*platform.CreateFunctionBuildResult), args.Error(1)
}

// Deploy will deploy a processor image to the platform (optionally building it, if source is provided)
func (p *Platform) CreateFunction(createFunctionOptions *platform.CreateFunctionOptions) (*platform.CreateFunctionResult, error) {

	// release requester
	if createFunctionOptions.CreationStateUpdated != nil {
		createFunctionOptions.CreationStateUpdated <- true
	}

	args := p.Called(createFunctionOptions)
	return args.Get(0).(*platform.CreateFunctionResult), args.Error(1)
}

// UpdateFunction will update a previously deployed function
func (p *Platform) UpdateFunction(updateFunctionOptions *platform.UpdateFunctionOptions) error {
	args := p.Called(updateFunctionOptions)
	return args.Error(0)
}

// DeleteFunction will delete a previously deployed function
func (p *Platform) DeleteFunction(deleteFunctionOptions *platform.DeleteFunctionOptions) error {
	args := p.Called(deleteFunctionOptions)
	return args.Error(0)
}

// CreateFunctionInvocation will invoke a previously deployed function
func (p *Platform) CreateFunctionInvocation(createFunctionInvocationOptions *platform.CreateFunctionInvocationOptions) (*platform.CreateFunctionInvocationResult, error) {
	args := p.Called(createFunctionInvocationOptions)
	return args.Get(0).(*platform.CreateFunctionInvocationResult), args.Error(1)
}

// GetFunctions will list existing functions
func (p *Platform) GetFunctions(getFunctionsOptions *platform.GetFunctionsOptions) ([]platform.Function, error) {
	args := p.Called(getFunctionsOptions)
	return args.Get(0).([]platform.Function), args.Error(1)
}

//
// Project
//

// CreateProject will probably create a new project
func (p *Platform) CreateProject(createProjectOptions *platform.CreateProjectOptions) error {
	args := p.Called(createProjectOptions)
	return args.Error(0)
}

// UpdateProject will update a previously existing project
func (p *Platform) UpdateProject(updateProjectOptions *platform.UpdateProjectOptions) error {
	args := p.Called(updateProjectOptions)
	return args.Error(0)
}

// DeleteProject will delete a previously existing project
func (p *Platform) DeleteProject(deleteProjectOptions *platform.DeleteProjectOptions) error {
	args := p.Called(deleteProjectOptions)
	return args.Error(0)
}

// GetProjects will list existing projects
func (p *Platform) GetProjects(getProjectsOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	args := p.Called(getProjectsOptions)
	return args.Get(0).([]platform.Project), args.Error(1)
}

//
// Function event
//

// CreateFunctionEvent will create a new function event that can later be used as a template from
// which to invoke functions
func (p *Platform) CreateFunctionEvent(createFunctionEventOptions *platform.CreateFunctionEventOptions) error {
	args := p.Called(createFunctionEventOptions)
	return args.Error(0)
}

// UpdateFunctionEvent will update a previously existing function event
func (p *Platform) UpdateFunctionEvent(updateFunctionEventOptions *platform.UpdateFunctionEventOptions) error {
	args := p.Called(updateFunctionEventOptions)
	return args.Error(0)
}

// DeleteFunctionEvent will delete a previously existing function event
func (p *Platform) DeleteFunctionEvent(deleteFunctionEventOptions *platform.DeleteFunctionEventOptions) error {
	args := p.Called(deleteFunctionEventOptions)
	return args.Error(0)
}

// GetFunctionEvents will list existing function events
func (p *Platform) GetFunctionEvents(getFunctionEventsOptions *platform.GetFunctionEventsOptions) ([]platform.FunctionEvent, error) {
	args := p.Called(getFunctionEventsOptions)
	return args.Get(0).([]platform.FunctionEvent), args.Error(1)
}

//
// Misc
//

func (p *Platform) SetDefaultHTTPIngressHostTemplate(defaultHTTPIngressHostTemplate string) {
	p.Called(defaultHTTPIngressHostTemplate)
}

func (p *Platform) GetDefaultHTTPIngressHostTemplate() string {
	args := p.Called()
	return args.Get(0).(string)
}

func (p *Platform) SetImageNamePrefixTemplate(imageNamePrefixTemplate string) {
	p.Called(imageNamePrefixTemplate)
}

func (p *Platform) GetImageNamePrefixTemplate() string {
	args := p.Called()
	return args.Get(0).(string)
}

func (p *Platform) RenderImageNamePrefixTemplate(projectName string, functionName string) (string, error) {
	args := p.Called()
	return args.Get(0).(string), args.Error(1)
}

// SetExternalIPAddresses configures the IP addresses invocations will use, if "via" is set to "external-ip".
// If this is not invoked, each platform will try to discover these addresses automatically
func (p *Platform) SetExternalIPAddresses(externalIPAddresses []string) error {
	args := p.Called(externalIPAddresses)
	return args.Error(0)
}

// GetExternalIPAddresses returns the external IP addresses invocations will use, if "via" is set to "external-ip".
// These addresses are either set through SetExternalIPAddresses or automatically discovered
func (p *Platform) GetExternalIPAddresses() ([]string, error) {
	args := p.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (p *Platform) GetScaleToZeroConfiguration() (*platformconfig.ScaleToZero, error) {
	args := p.Called()
	return args.Get(0).(*platformconfig.ScaleToZero), args.Error(1)
}

// GetHealthCheckMode returns the healthcheck mode the platform requires
func (p *Platform) GetHealthCheckMode() platform.HealthCheckMode {
	args := p.Called()
	return args.Get(0).(platform.HealthCheckMode)
}

// GetName returns the platform name
func (p *Platform) GetName() string {
	args := p.Called()
	return args.String(0)
}

// GetNodes returns a slice of nodes currently in the cluster
func (p *Platform) GetNodes() ([]platform.Node, error) {
	args := p.Called()
	return args.Get(0).([]platform.Node), args.Error(1)
}

// ResolveDefaultNamespace returns the proper default resource namespace, given the current default namespace
func (p *Platform) ResolveDefaultNamespace(defaultNamespace string) string {
	args := p.Called()
	return args.Get(0).(string)
}

// GetNamespaces returns the namespaces
func (p *Platform) GetNamespaces() ([]string, error) {
	args := p.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (p *Platform) GetDefaultInvokeIPAddresses() ([]string, error) {
	args := p.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (p *Platform) BuildAndPushContainerImage(buildOptions *containerimagebuilderpusher.BuildOptions) error {
	return nil
}

func (p *Platform) GetOnbuildStages(onbuildArtifacts []runtime.Artifact) ([]string, error) {
	return []string{}, nil
}

func (p *Platform) TransformOnbuildArtifactPaths(onbuildArtifacts []runtime.Artifact) (map[string]string, error) {
	return map[string]string{}, nil
}

func (p *Platform) GetBaseImageRegistry(registry string) string {
	return "quay.io"
}
