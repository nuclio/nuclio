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

package abstract

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher"
	"github.com/nuclio/nuclio/pkg/dashboard/auth"
	"github.com/nuclio/nuclio/pkg/dashboard/auth/iguazio"
	"github.com/nuclio/nuclio/pkg/errgroup"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/logprocessing"
	"github.com/nuclio/nuclio/pkg/opa"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor/build"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/docker/distribution/reference"
	"github.com/google/go-cmp/cmp"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation"
)

//
// Base for all platforms
//

const (
	FunctionContainerHTTPPort            = 8080
	FunctionContainerWebAdminHTTPPort    = 8081
	FunctionContainerHealthCheckHTTPPort = 8082
	DefaultTargetCPU                     = 75
)

type Platform struct {
	Logger                  logger.Logger
	platform                platform.Platform
	invoker                 *invoker
	Config                  *platformconfig.Config
	ExternalIPAddresses     []string
	DeployLogStreams        *sync.Map
	ContainerBuilder        containerimagebuilderpusher.BuilderPusher
	ImageNamePrefixTemplate string
	DefaultNamespace        string
	OpaClient               opa.Client
}

func NewPlatform(parentLogger logger.Logger,
	platform platform.Platform,
	platformConfiguration *platformconfig.Config,
	defaultNamespace string) (*Platform, error) {
	var err error

	newPlatform := &Platform{
		Logger:           parentLogger.GetChild("platform"),
		platform:         platform,
		Config:           platformConfiguration,
		DeployLogStreams: &sync.Map{},
	}

	// create invoker
	newPlatform.invoker, err = newInvoker(newPlatform.Logger, platform)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create invoker")
	}

	newPlatform.DefaultNamespace = defaultNamespace

	newPlatform.OpaClient = opa.CreateOpaClient(newPlatform.Logger, &platformConfiguration.Opa)

	return newPlatform, nil
}

func (ap *Platform) GetConfig() *platformconfig.Config {
	return ap.Config
}

func (ap *Platform) CreateFunctionBuild(createFunctionBuildOptions *platform.CreateFunctionBuildOptions) (
	*platform.CreateFunctionBuildResult, error) {

	// execute a build
	builder, err := build.NewBuilder(createFunctionBuildOptions.Logger, ap.platform, &common.AbstractS3Client{})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create builder")
	}

	// convert types
	return builder.Build(createFunctionBuildOptions)
}

// HandleDeployFunction calls a deployer that does the platform specific deploy, but adds a lot
// of common code
func (ap *Platform) HandleDeployFunction(ctx context.Context,
	existingFunctionConfig *functionconfig.ConfigWithStatus,
	createFunctionOptions *platform.CreateFunctionOptions,
	onAfterConfigUpdated func() error,
	onAfterBuild func(*platform.CreateFunctionBuildResult, error) (*platform.CreateFunctionResult, error)) (
	*platform.CreateFunctionResult, error) {

	createFunctionOptions.Logger.InfoWithCtx(ctx, "Deploying function",
		"name", createFunctionOptions.FunctionConfig.Meta.Name)

	var buildResult *platform.CreateFunctionBuildResult
	var buildErr error

	// when the config is updated, save to deploy options and call underlying hook
	onAfterConfigUpdatedWrapper := func(updatedFunctionConfig *functionconfig.Config) error {
		createFunctionOptions.FunctionConfig = *updatedFunctionConfig

		return onAfterConfigUpdated()
	}

	functionBuildRequired, err := ap.functionBuildRequired(&createFunctionOptions.FunctionConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed determining whether function should build")
	}

	// clear build mode
	createFunctionOptions.FunctionConfig.Spec.Build.Mode = ""

	// check if we need to build the image
	if functionBuildRequired && !functionconfig.ShouldSkipBuild(createFunctionOptions.FunctionConfig.Meta.Annotations) {
		buildResult, buildErr = ap.platform.CreateFunctionBuild(&platform.CreateFunctionBuildOptions{
			Logger:                     createFunctionOptions.Logger,
			FunctionConfig:             createFunctionOptions.FunctionConfig,
			PlatformName:               ap.platform.GetName(),
			OnAfterConfigUpdate:        onAfterConfigUpdatedWrapper,
			DependantImagesRegistryURL: createFunctionOptions.DependantImagesRegistryURL,
		})

		if buildErr == nil {

			// use the function configuration augmented by the builder
			createFunctionOptions.FunctionConfig.Spec.Image = buildResult.Image

			// if run registry isn't set, set it to that of the build
			if createFunctionOptions.FunctionConfig.Spec.RunRegistry == "" {
				createFunctionOptions.FunctionConfig.Spec.RunRegistry =
					createFunctionOptions.FunctionConfig.Spec.Build.Registry
			}

			// on successful build set the timestamp of build
			createFunctionOptions.FunctionConfig.Spec.Build.Timestamp = time.Now().Unix()
		}
	} else {
		createFunctionOptions.Logger.InfoWithCtx(ctx, "Skipping build",
			"name", createFunctionOptions.FunctionConfig.Meta.Name)

		// verify user passed runtime
		if createFunctionOptions.FunctionConfig.Spec.Runtime == "" {
			return nil, errors.New("If image is passed, runtime must be specified")
		}

		// populate image if possible
		if existingFunctionConfig != nil && createFunctionOptions.FunctionConfig.Spec.Image == "" {
			createFunctionOptions.FunctionConfig.Spec.Image = existingFunctionConfig.Spec.Image
		}

		// trigger the on after config update ourselves
		if err = onAfterConfigUpdatedWrapper(&createFunctionOptions.FunctionConfig); err != nil {
			return nil, errors.Wrap(err, "Failed to trigger on after config update")
		}
	}

	// wrap the deployer's deploy with the base HandleDeployFunction
	deployResult, err := onAfterBuild(buildResult, buildErr)
	if buildErr != nil || err != nil {
		return nil, errors.Wrap(err, "Failed to deploy function")
	}

	// sanity
	if deployResult == nil {
		return nil, errors.New("Deployer returned no error, but nil deploy result")
	}

	// if we got a deploy result and build result, set them
	if buildResult != nil {
		deployResult.CreateFunctionBuildResult = *buildResult
	}

	// indicate that we're done
	createFunctionOptions.Logger.InfoWithCtx(ctx, "Function deploy complete",
		"functionName", deployResult.UpdatedFunctionConfig.Meta.Name,
		"httpPort", deployResult.Port,
		"internalInvocationURLs", deployResult.FunctionStatus.InternalInvocationURLs,
		"externalInvocationURLs", deployResult.FunctionStatus.ExternalInvocationURLs)
	return deployResult, nil
}

// EnrichFunctionConfig enriches function config
func (ap *Platform) EnrichFunctionConfig(ctx context.Context, functionConfig *functionconfig.Config) error {

	// if labels is nil assign an empty map to it
	if functionConfig.Meta.Labels == nil {
		functionConfig.Meta.Labels = map[string]string{}
	}
	ap.EnrichLabels(ctx, functionConfig.Meta.Labels)

	if err := ap.enrichImageName(functionConfig); err != nil {
		return errors.Wrap(err, "Failed enriching image name")
	}

	ap.enrichMinMaxReplicas(functionConfig)

	// enrich with registry credential secret name
	if functionConfig.Spec.ImagePullSecrets == "" {
		functionConfig.Spec.ImagePullSecrets =
			ap.GetDefaultRegistryCredentialsSecretName()
	}

	// `python` is just an alias
	if functionConfig.Spec.Runtime == "python" {
		functionConfig.Spec.Runtime = "python:3.6"
	}

	// enrich triggers
	if err := ap.enrichTriggers(functionConfig); err != nil {
		return errors.Wrap(err, "Failed enriching triggers")
	}

	// enrich with security context
	if functionConfig.Spec.SecurityContext == nil {
		functionConfig.Spec.SecurityContext = &v1.PodSecurityContext{}
	}

	if err := ap.enrichVolumes(functionConfig); err != nil {
		return errors.Wrap(err, "Failed enriching volumes")
	}

	return nil
}

// EnrichLabels enriches labels with default project name
func (ap *Platform) EnrichLabels(ctx context.Context, labels map[string]string) {
	if labels[common.NuclioResourceLabelKeyProjectName] == "" {
		labels[common.NuclioResourceLabelKeyProjectName] = platform.DefaultProjectName
		ap.Logger.DebugCtx(ctx, "No project name specified. Setting to default")
	}

	// enrich labels with iguazio.com/username of the creating user
	if authSession, ok := ctx.Value(auth.AuthSessionContextKey).(auth.IguazioSession); ok {
		if labels[iguazio.IguzioUsernameLabel] == "" {
			labels[iguazio.IguzioUsernameLabel] = authSession.GetUsername()
		}
	}
}

func (ap *Platform) enrichDefaultHTTPTrigger(functionConfig *functionconfig.Config) {
	if len(functionconfig.GetTriggersByKind(functionConfig.Spec.Triggers, "http")) > 0 {
		return
	}

	if functionConfig.Spec.Triggers == nil {
		functionConfig.Spec.Triggers = map[string]functionconfig.Trigger{}
	}

	defaultHTTPTrigger := functionconfig.GetDefaultHTTPTrigger()
	functionConfig.Spec.Triggers[defaultHTTPTrigger.Name] = defaultHTTPTrigger
}

// ValidateCreateFunctionOptionsAgainstExistingFunctionConfig validates a function against its existing instance
func (ap *Platform) ValidateCreateFunctionOptionsAgainstExistingFunctionConfig(ctx context.Context,
	existingFunctionConfig *functionconfig.ConfigWithStatus,
	createFunctionOptions *platform.CreateFunctionOptions) error {

	// special case when we are asked to build the function and it wasn't been created yet
	if existingFunctionConfig == nil &&
		createFunctionOptions.FunctionConfig.Spec.Build.Mode == functionconfig.NeverBuild {
		return errors.New("Non existing function cannot be created with neverBuild mode")
	}

	// validate resource version
	if err := ap.ValidateResourceVersion(ctx, existingFunctionConfig, &createFunctionOptions.FunctionConfig); err != nil {
		return nuclio.WrapErrConflict(err)
	}

	// do not allow disabling a function in imported state
	// in the imported state, after the function has the skip-build and skip-deploy annotations removed,
	// if the user tries to disable the function, it will in turn build and deploy the function and then disable it.
	if existingFunctionConfig != nil &&
		existingFunctionConfig.Status.State == functionconfig.FunctionStateImported &&
		createFunctionOptions.FunctionConfig.Spec.Disable {
		return errors.New("Failed to disable function: non-deployed functions cannot be disabled")
	}

	// do not allow updating functions that are being provisioned
	if existingFunctionConfig != nil &&
		functionconfig.FunctionStateProvisioning(existingFunctionConfig.Status.State) {
		return nuclio.WrapErrPreconditionFailed(errors.New("Function cannot be updated when existing function is being provisioned"))
	}

	// do not allow disabling a function being used by an api gateway
	if existingFunctionConfig != nil &&
		len(existingFunctionConfig.Status.APIGateways) > 0 &&
		createFunctionOptions.FunctionConfig.Spec.Disable {
		ap.Logger.WarnWithCtx(ctx, "Disabling function with assigned api gateway validation failed",
			"functionName", existingFunctionConfig.Meta.Name,
			"apiGateways", existingFunctionConfig.Status.APIGateways)
		return nuclio.NewErrBadRequest("Cannot disable function while used by an API gateway")
	}
	return nil
}

// ValidateResourceVersion validates existing and new create function options resource version
func (ap *Platform) ValidateResourceVersion(ctx context.Context,
	functionConfigWithStatus *functionconfig.ConfigWithStatus,
	requestFunctionConfig *functionconfig.Config) error {

	// if function has no existing instance, resource version validation is irrelevant.
	if functionConfigWithStatus == nil {
		return nil
	}

	// existing function should always be the latest
	// reason: the way we `GET` nuclio function ensures we retrieve the latest copy.
	existingResourceVersion := functionConfigWithStatus.Meta.ResourceVersion
	requestResourceVersion := requestFunctionConfig.Meta.ResourceVersion

	// when requestResourceVersion is empty, the existing one will be overridden
	if requestResourceVersion != "" &&
		requestResourceVersion != existingResourceVersion {
		ap.Logger.WarnWithCtx(ctx, "Function resource version is stale",
			"functionName", functionConfigWithStatus.Meta.Name,
			"requestResourceVersion", requestResourceVersion,
			"existingResourceVersion", existingResourceVersion)
		return errors.New("Function resource version is stale")
	}
	return nil
}

// EnrichFunctionsWithDeployLogStream enriches functions status with logs
func (ap *Platform) EnrichFunctionsWithDeployLogStream(functions []platform.Function) {

	// iterate over functions and enrich with deploy logs
	for _, function := range functions {
		if deployLogStream, exists := ap.DeployLogStreams.Load(function.GetConfig().Meta.GetUniqueID()); exists {
			deployLogStream.(*LogStream).ReadLogs(nil, &function.GetStatus().Logs)
		}
	}
}

// ValidateFunctionConfig validaets and enforces of required function creation logic
func (ap *Platform) ValidateFunctionConfig(ctx context.Context, functionConfig *functionconfig.Config) error {

	if common.StringInSlice(functionConfig.Meta.Name, ap.ResolveReservedResourceNames()) {
		return nuclio.NewErrPreconditionFailed(fmt.Sprintf("Function name %s is reserved and cannot be used.",
			functionConfig.Meta.Name))
	}

	// check function config for possible malicious content
	if err := ap.validateDockerImageFields(ctx, functionConfig); err != nil {
		return errors.Wrap(err, "Docker image fields validation failed")
	}

	if err := ap.validateTriggers(functionConfig); err != nil {
		return errors.Wrap(err, "Triggers validation failed")
	}

	if err := ap.validateMinMaxReplicas(functionConfig); err != nil {
		return errors.Wrap(err, "Min max replicas validation failed")
	}

	if err := ap.validateNodeSelector(functionConfig); err != nil {
		return errors.Wrap(err, "Node selector validation failed")
	}

	if err := ap.validateProjectExists(ctx, functionConfig); err != nil {
		return errors.Wrap(err, "Project existence validation failed")
	}

	if err := ap.validateVolumes(ctx, functionConfig); err != nil {
		return errors.Wrap(err, "Volumes validation failed")
	}

	if err := ap.validatePriorityClassName(functionConfig); err != nil {
		return errors.Wrap(err, "Priority class name validation failed")
	}

	return nil
}

// ValidateDeleteProjectOptions validates and enforces of required project deletion logic
func (ap *Platform) ValidateDeleteProjectOptions(ctx context.Context,
	deleteProjectOptions *platform.DeleteProjectOptions) error {
	projectName := deleteProjectOptions.Meta.Name

	switch projectName {
	case platform.DefaultProjectName:

		// projects is controlled by a leader. when not set, do not allow deleting the only project
		if ap.Config.ProjectsLeader == nil {
			return nuclio.NewErrPreconditionFailed("Cannot delete the default project")
		}
	case "":
		return nuclio.NewErrBadRequest("Project name cannot be empty")
	}

	switch deleteProjectOptions.Strategy {
	case platform.DeleteProjectStrategyCheck, platform.DeleteProjectStrategyRestricted:
		// listing project resources might be too excessive
		// to avoid listing resources for non-existing project, first we ensure it exists
		projects, err := ap.platform.GetProjects(ctx, &platform.GetProjectsOptions{
			Meta:              deleteProjectOptions.Meta,
			RequestOrigin:     deleteProjectOptions.RequestOrigin,
			PermissionOptions: deleteProjectOptions.PermissionOptions,
			AuthSession:       deleteProjectOptions.AuthSession,
		})
		if err != nil {
			return errors.Wrap(err, "Failed to get project")

		}

		// project does not exists, stop here
		if len(projects) == 0 {
			return nil
		}

		functions, apiGateways, err := ap.GetProjectResources(ctx, &deleteProjectOptions.Meta)
		if err != nil {
			return errors.Wrap(err, "Failed to get project resources")
		}

		if len(functions) > 0 {
			return platform.ErrProjectContainsFunctions
		} else if len(apiGateways) > 0 {
			return platform.ErrProjectContainsAPIGateways
		}
	}
	return nil
}

// ValidateDeleteFunctionOptions validates and enforces of required function deletion logic
func (ap *Platform) ValidateDeleteFunctionOptions(ctx context.Context, deleteFunctionOptions *platform.DeleteFunctionOptions) (
	platform.Function, error) {
	functionName := deleteFunctionOptions.FunctionConfig.Meta.Name
	functionNamespace := deleteFunctionOptions.FunctionConfig.Meta.Namespace
	functions, err := ap.platform.GetFunctions(ctx, &platform.GetFunctionsOptions{
		Name:              functionName,
		Namespace:         functionNamespace,
		AuthSession:       deleteFunctionOptions.AuthSession,
		PermissionOptions: deleteFunctionOptions.PermissionOptions,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get functions")
	}

	// function does not exist and hence nothing to validate (that might happen, delete method can be idempotent)
	if len(functions) == 0 {
		ap.Logger.DebugWithCtx(ctx, "Function was not found (deleted already?)", "functionName", functionName)
		return nil, nil
	}

	functionToDelete := functions[0]

	// validate resource version
	if err := ap.ValidateResourceVersion(ctx,
		functionToDelete.GetConfigWithStatus(),
		&deleteFunctionOptions.FunctionConfig); err != nil {
		return functionToDelete, nuclio.WrapErrConflict(err)
	}

	if !deleteFunctionOptions.IgnoreFunctionStateValidation {

		// do not allow deleting functions that are being provisioned
		if functionconfig.FunctionStateProvisioning(functionToDelete.GetStatus().State) {
			ap.Logger.WarnWith("Function cannot be deleted as it is being provisioned",
				"functionName", functionToDelete.GetConfig().Meta.Name)

			// update UI when changing text / code
			return functionToDelete, nuclio.NewErrPreconditionFailed("Function is being provisioned and cannot be deleted")
		}
	}

	// Check OPA permissions
	permissionOptions := deleteFunctionOptions.PermissionOptions
	permissionOptions.RaiseForbidden = true
	if _, err := ap.QueryOPAFunctionPermissions(functionToDelete.GetConfig().Meta.Labels[common.NuclioResourceLabelKeyProjectName],
		functionToDelete.GetConfig().Meta.Name,
		opa.ActionDelete,
		&permissionOptions); err != nil {
		return functionToDelete, errors.Wrap(err, "Failed authorizing OPA permissions for resource")
	}

	return functionToDelete, nil
}

// ResolveReservedResourceNames returns a list of reserved resource names
func (ap *Platform) ResolveReservedResourceNames() []string {

	// these names are reserved for Nuclio internal purposes and to avoid collisions with nuclio internal resources
	return []string{
		"dashboard",
		"controller",
		"dlx",
		"scaler",
	}
}

// FilterProjectsByPermissions will filter out some projects
func (ap *Platform) FilterProjectsByPermissions(permissionOptions *opa.PermissionOptions,
	projects []platform.Project) ([]platform.Project, error) {

	// no cleansing is mandated
	if len(permissionOptions.MemberIds) == 0 || len(projects) == 0 {
		return projects, nil
	}

	// prepare resource list
	resources := make([]string, len(projects))
	for idx, project := range projects {
		projectName := project.GetConfig().Meta.Name
		resources[idx] = opa.GenerateProjectResourceString(projectName)
	}

	allowedList, err := ap.QueryOPAMultipleResources(resources, opa.ActionRead, permissionOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed querying OPA for projects permissions")
	}

	// fill permitted / filtered project list
	var permittedProjects []platform.Project
	var filteredProjectNames []string
	for idx, allowed := range allowedList {
		if allowed {
			permittedProjects = append(permittedProjects, projects[idx])
		} else {
			filteredProjectNames = append(filteredProjectNames, projects[idx].GetConfig().Meta.Name)
		}
	}

	if len(filteredProjectNames) > 0 {
		ap.Logger.DebugWith("Some projects were filtered out", "projectNames", filteredProjectNames)
	}
	return permittedProjects, nil
}

// FilterFunctionsByPermissions will filter out some functions
func (ap *Platform) FilterFunctionsByPermissions(ctx context.Context,
	permissionOptions *opa.PermissionOptions,
	functions []platform.Function) ([]platform.Function, error) {

	// no cleansing is mandated
	if len(permissionOptions.MemberIds) == 0 || len(functions) == 0 {
		return functions, nil
	}

	// prepare resource list
	resources := make([]string, len(functions))
	for idx, function := range functions {
		functionName := function.GetConfig().Meta.Name
		projectName := function.GetConfig().Meta.Labels[common.NuclioResourceLabelKeyProjectName]
		resources[idx] = opa.GenerateFunctionResourceString(projectName, functionName)
	}

	allowedList, err := ap.QueryOPAMultipleResources(resources, opa.ActionRead, permissionOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed querying OPA for function permissions")
	}

	// fill permitted / filtered function list
	var permittedFunctions []platform.Function
	var filteredFunctionNames []string
	for idx, allowed := range allowedList {
		if allowed {
			permittedFunctions = append(permittedFunctions, functions[idx])
		} else {
			filteredFunctionNames = append(filteredFunctionNames, functions[idx].GetConfig().Meta.Name)
		}
	}

	if len(filteredFunctionNames) > 0 {
		ap.Logger.DebugWithCtx(ctx, "Some functions were filtered out", "functionNames", filteredFunctionNames)
	}
	return permittedFunctions, nil
}

// FilterFunctionEventsByPermissions will filter out some function events
func (ap *Platform) FilterFunctionEventsByPermissions(ctx context.Context,
	permissionOptions *opa.PermissionOptions,
	functionEvents []platform.FunctionEvent) ([]platform.FunctionEvent, error) {

	// no cleansing is mandated
	if len(permissionOptions.MemberIds) == 0 || len(functionEvents) == 0 {
		return functionEvents, nil
	}

	var resources []string
	for _, functionEventInstance := range functionEvents {
		projectName := functionEventInstance.GetConfig().Meta.Labels[common.NuclioResourceLabelKeyProjectName]
		functionName := functionEventInstance.GetConfig().Meta.Labels[common.NuclioResourceLabelKeyFunctionName]
		functionEventName := functionEventInstance.GetConfig().Meta.Name
		resources = append(resources, opa.GenerateFunctionEventResourceString(projectName,
			functionName,
			functionEventName))
	}
	allowedList, err := ap.QueryOPAMultipleResources(resources, opa.ActionRead, permissionOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed querying OPA for function events permissions")
	}

	// fill permitted / filtered function event list
	var permittedFunctionEvents []platform.FunctionEvent
	var filteredFunctionEventNames []string
	for idx, allowed := range allowedList {
		if allowed {
			permittedFunctionEvents = append(permittedFunctionEvents, functionEvents[idx])
		} else {
			filteredFunctionEventNames = append(filteredFunctionEventNames, functionEvents[idx].GetConfig().Meta.Name)
		}
	}

	if len(filteredFunctionEventNames) > 0 {
		ap.Logger.DebugWithCtx(ctx,
			"Some function events were filtered out",
			"functionEventNames", filteredFunctionEventNames)
	}
	return permittedFunctionEvents, nil
}

// CreateFunctionInvocation will invoke a previously deployed function
func (ap *Platform) CreateFunctionInvocation(ctx context.Context,
	createFunctionInvocationOptions *platform.CreateFunctionInvocationOptions) (
	*platform.CreateFunctionInvocationResult, error) {
	if createFunctionInvocationOptions.Headers == nil {
		createFunctionInvocationOptions.Headers = http.Header{}
	}

	// get the function
	functions, err := ap.platform.GetFunctions(ctx, &platform.GetFunctionsOptions{
		Name:              createFunctionInvocationOptions.Name,
		Namespace:         createFunctionInvocationOptions.Namespace,
		AuthSession:       createFunctionInvocationOptions.AuthSession,
		PermissionOptions: createFunctionInvocationOptions.PermissionOptions,
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get functions")
	}

	if len(functions) == 0 {
		return nil, nuclio.NewErrNotFound(fmt.Sprintf("Function not found: %s @ %s",
			createFunctionInvocationOptions.Name,
			createFunctionInvocationOptions.Namespace))
	}

	// use the first function found (should always be one, but if there's more just use first)
	function := functions[0]

	// make sure to initialize the function (some underlying functions are lazy load)
	if err := function.Initialize(ctx, nil); err != nil {
		return nil, errors.Wrap(err, "Failed to initialize function")
	}

	return ap.invoker.invoke(ctx, function, createFunctionInvocationOptions)
}

// GetHealthCheckMode returns the healthcheck mode the platform requires
func (ap *Platform) GetHealthCheckMode() platform.HealthCheckMode {

	// by default return that some external entity does health checks for us
	return platform.HealthCheckModeExternal
}

// CreateProject will probably create a new project
func (ap *Platform) CreateProject(ctx context.Context, createProjectOptions *platform.CreateProjectOptions) error {
	return platform.ErrUnsupportedMethod
}

// EnrichCreateProjectConfig enrich project configuration with defaults
func (ap *Platform) EnrichCreateProjectConfig(createProjectOptions *platform.CreateProjectOptions) error {

	// enrich project owner from auth session
	if createProjectOptions.AuthSession != nil && createProjectOptions.ProjectConfig.Spec.Owner == "" {
		createProjectOptions.ProjectConfig.Spec.Owner = createProjectOptions.AuthSession.GetUsername()
	}

	return nil
}

// ValidateProjectConfig perform validation on a given project config
func (ap *Platform) ValidateProjectConfig(projectConfig *platform.ProjectConfig) error {
	if projectConfig.Meta.Name == "" {
		return nuclio.NewErrBadRequest("Project name cannot be empty")
	}

	// project name should adhere Kubernetes label restrictions
	errorMessages := validation.IsDNS1123Label(projectConfig.Meta.Name)
	if len(errorMessages) != 0 {
		joinedErrorMessage := strings.Join(errorMessages, ", ")
		return nuclio.NewErrBadRequest(
			fmt.Sprintf(`Project name must adhere to Kubernetes naming conventions. Errors: %s`,
				joinedErrorMessage))
	}
	return nil
}

// UpdateProject will update a previously existing project
func (ap *Platform) UpdateProject(ctx context.Context, updateProjectOptions *platform.UpdateProjectOptions) error {
	return platform.ErrUnsupportedMethod
}

// DeleteProject will delete a previously existing project
func (ap *Platform) DeleteProject(ctx context.Context, deleteProjectOptions *platform.DeleteProjectOptions) error {
	return platform.ErrUnsupportedMethod
}

// GetProjects will list existing projects
func (ap *Platform) GetProjects(ctx context.Context, getProjectsOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	return nil, platform.ErrUnsupportedMethod
}

// CreateAPIGateway creates and deploys a new api gateway
func (ap *Platform) CreateAPIGateway(ctx context.Context, createAPIGatewayOptions *platform.CreateAPIGatewayOptions) error {
	return platform.ErrUnsupportedMethod
}

// UpdateAPIGateway will update a previously existing api gateway
func (ap *Platform) UpdateAPIGateway(ctx context.Context, updateAPIGatewayOptions *platform.UpdateAPIGatewayOptions) error {
	return platform.ErrUnsupportedMethod
}

// DeleteAPIGateway will delete a previously existing api gateway
func (ap *Platform) DeleteAPIGateway(ctx context.Context, deleteAPIGatewayOptions *platform.DeleteAPIGatewayOptions) error {
	return platform.ErrUnsupportedMethod
}

// GetAPIGateways will list existing api gateways
func (ap *Platform) GetAPIGateways(ctx context.Context, getAPIGatewaysOptions *platform.GetAPIGatewaysOptions) ([]platform.APIGateway, error) {
	return nil, platform.ErrUnsupportedMethod
}

// CreateFunctionEvent will create a new function event that can later be used as a template from
// which to invoke functions
func (ap *Platform) CreateFunctionEvent(ctx context.Context, createFunctionEventOptions *platform.CreateFunctionEventOptions) error {
	return platform.ErrUnsupportedMethod
}

func (ap *Platform) EnrichFunctionEvent(ctx context.Context, functionEventConfig *platform.FunctionEventConfig) error {

	// to avoid blow-ups
	if functionEventConfig.Meta.Labels == nil {
		functionEventConfig.Meta.Labels = map[string]string{}
	}

	// take display name from display name if missing
	if functionEventConfig.Spec.DisplayName == "" {
		functionEventConfig.Spec.DisplayName = functionEventConfig.Meta.Name
	}

	// default to http trigger
	if functionEventConfig.Spec.TriggerKind == "" {
		functionEventConfig.Spec.TriggerKind = platform.DefaultFunctionEventTriggerKind
	}

	// enrich http kind
	if functionEventConfig.Spec.TriggerKind == platform.FunctionEventTriggerKindHTTP {
		if functionEventConfig.Spec.Attributes == nil {
			functionEventConfig.Spec.Attributes = map[string]interface{}{}
		}

		// enrich attributes with key: value
		for key, value := range map[string]interface{}{
			"headers": map[string]string{"Content-Type": "text/plain"},
			"method":  http.MethodPost,
			"path":    "",
		} {
			if _, exists := functionEventConfig.Spec.Attributes[key]; !exists {
				functionEventConfig.Spec.Attributes[key] = value
			}
		}
	}

	functionName, functionNameFound := functionEventConfig.Meta.Labels[common.NuclioResourceLabelKeyFunctionName]
	if !functionNameFound {
		return errors.Errorf("Function event has a missing label - `%s`",
			common.NuclioResourceLabelKeyFunctionName)
	}

	projectName, projectNameFound := functionEventConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName]
	if !projectNameFound {
		ap.Logger.DebugWithCtx(ctx, "Enriching function event project name",
			"functionEventName", functionEventConfig.Meta.Name,
			"functionEventNamespace", functionEventConfig.Meta.Namespace,
			"functionName", functionName)

		// infer project name from its function
		functions, err := ap.platform.GetFunctions(ctx, &platform.GetFunctionsOptions{
			Name:      functionName,
			Namespace: functionEventConfig.Meta.Namespace,
		})
		if err != nil {
			return errors.Wrap(err, "Failed to get function")
		}
		if len(functions) == 0 {
			return errors.Errorf("The Function event parent function does not exist")
		}

		function := functions[0]
		projectName = function.GetConfig().Meta.Labels[common.NuclioResourceLabelKeyProjectName]
	}

	functionEventConfig.Meta.Labels[common.NuclioResourceLabelKeyFunctionName] = functionName
	functionEventConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName] = projectName
	return nil
}

// UpdateFunctionEvent will update a previously existing function event
func (ap *Platform) UpdateFunctionEvent(ctx context.Context, updateFunctionEventOptions *platform.UpdateFunctionEventOptions) error {
	return platform.ErrUnsupportedMethod
}

// DeleteFunctionEvent will delete a previously existing function event
func (ap *Platform) DeleteFunctionEvent(ctx context.Context, deleteFunctionEventOptions *platform.DeleteFunctionEventOptions) error {
	return platform.ErrUnsupportedMethod
}

// GetFunctionEvents will list existing function events
func (ap *Platform) GetFunctionEvents(ctx context.Context, getFunctionEventsOptions *platform.GetFunctionEventsOptions) ([]platform.FunctionEvent, error) {
	return nil, platform.ErrUnsupportedMethod
}

// SetExternalIPAddresses configures the IP addresses invocations will use, if "via" is set to "external-ip".
// If this is not invoked, each platform will try to discover these addresses automatically
func (ap *Platform) SetExternalIPAddresses(externalIPAddresses []string) error {
	ap.ExternalIPAddresses = externalIPAddresses

	return nil
}

func (ap *Platform) SetImageNamePrefixTemplate(imageNamePrefixTemplate string) {
	ap.ImageNamePrefixTemplate = imageNamePrefixTemplate
}

func (ap *Platform) GetImageNamePrefixTemplate() string {
	return ap.ImageNamePrefixTemplate
}

func (ap *Platform) RenderImageNamePrefixTemplate(projectName string, functionName string) (string, error) {
	return common.RenderTemplate(ap.ImageNamePrefixTemplate, map[string]interface{}{
		"ProjectName":  projectName,
		"FunctionName": functionName,
	})
}

// GetExternalIPAddresses returns the external IP addresses invocations will use, if "via" is set to "external-ip".
// These addresses are either set through SetExternalIPAddresses or automatically discovered
func (ap *Platform) GetExternalIPAddresses() ([]string, error) {
	return ap.ExternalIPAddresses, nil
}

// GetScaleToZeroConfiguration returns scale to zero configuration
func (ap *Platform) GetScaleToZeroConfiguration() *platformconfig.ScaleToZero {
	return nil
}

// GetAllowedAuthenticationModes returns allowed authentication modes
func (ap *Platform) GetAllowedAuthenticationModes() []string {
	return nil
}

// ResolveDefaultNamespace returns the proper default resource namespace, given the current default namespace
func (ap *Platform) ResolveDefaultNamespace(defaultNamespace string) string {
	return ""
}

// BuildAndPushContainerImage builds container image and pushes it into docker registry
func (ap *Platform) BuildAndPushContainerImage(buildOptions *containerimagebuilderpusher.BuildOptions) error {
	return ap.ContainerBuilder.BuildAndPushContainerImage(buildOptions,
		ap.platform.ResolveDefaultNamespace("@nuclio.selfNamespace"))
}

// GetOnbuildStages get onbuild multistage builds
func (ap *Platform) GetOnbuildStages(onbuildArtifacts []runtime.Artifact) ([]string, error) {
	return ap.ContainerBuilder.GetOnbuildStages(onbuildArtifacts)
}

// TransformOnbuildArtifactPaths changes Onbuild artifact paths depending on the type of the builder used
func (ap *Platform) TransformOnbuildArtifactPaths(onbuildArtifacts []runtime.Artifact) (map[string]string, error) {
	return ap.ContainerBuilder.TransformOnbuildArtifactPaths(onbuildArtifacts)
}

// GetBaseImageRegistry returns base image registry
func (ap *Platform) GetBaseImageRegistry(registry string, runtime runtime.Runtime) (string, error) {
	baseImagesOverrides := ap.getBaseImagesOverrides()

	if baseImagesOverrides == nil {
		baseImagesOverrides = map[string]string{}
	}

	imageRegistryRuntimeOverride := runtime.GetOverrideImageRegistryFromMap(baseImagesOverrides)
	if imageRegistryRuntimeOverride != "" {
		return imageRegistryRuntimeOverride, nil
	}
	return ap.ContainerBuilder.GetBaseImageRegistry(registry), nil
}

// GetOnbuildImageRegistry returns onbuild image registry
func (ap *Platform) GetOnbuildImageRegistry(registry string, runtime runtime.Runtime) (string, error) {
	onbuildImagesOverrides := ap.getOnbuildImagesOverrides()
	if onbuildImagesOverrides == nil {
		onbuildImagesOverrides = map[string]string{}
	}

	imageRegistryRuntimeOverride := runtime.GetOverrideImageRegistryFromMap(onbuildImagesOverrides)
	if imageRegistryRuntimeOverride != "" {
		return imageRegistryRuntimeOverride, nil
	}
	return ap.ContainerBuilder.GetOnbuildImageRegistry(registry), nil
}

// GetDefaultRegistryCredentialsSecretName returns secret with credentials to push/pull from docker registry
func (ap *Platform) GetDefaultRegistryCredentialsSecretName() string {
	return ap.ContainerBuilder.GetDefaultRegistryCredentialsSecretName()
}

// GetContainerBuilderKind returns the container-builder kind
func (ap *Platform) GetContainerBuilderKind() string {
	return ap.ContainerBuilder.GetKind()
}

// GetRuntimeBuildArgs returns the runtime specific build arguments
func (ap *Platform) GetRuntimeBuildArgs(runtime runtime.Runtime) map[string]string {
	return runtime.GetRuntimeBuildArgs(ap.Config.Runtime)
}

func (ap *Platform) GetProcessorLogsAndBriefError(scanner *bufio.Scanner) (string, string) {
	var formattedProcessorLogs, briefErrorsMessage string
	var stopWritingRawLinesToBriefErrorsMessage bool

	briefErrorsArray := &[]string{}

	for scanner.Scan() {
		currentLogLine, briefLogLine, err := logprocessing.PrettifyFunctionLogLine(ap.Logger, scanner.Bytes())
		if err != nil {
			rawLogLine := scanner.Text()

			// when it is unstructured just add the log as a text
			formattedProcessorLogs += rawLogLine + "\n"

			// if there's a panic or call stack is printed,
			// stop appending raw log lines to the briefErrorsMessage (unnecessary information from now on)
			// this information can still be found in the full log
			if strings.HasPrefix(rawLogLine, "panic: Wrapper") ||
				strings.HasPrefix(rawLogLine, "Call stack:") {
				stopWritingRawLinesToBriefErrorsMessage = true
			}

			if !stopWritingRawLinesToBriefErrorsMessage {
				*briefErrorsArray = append(*briefErrorsArray, rawLogLine)
			}

			continue
		}

		if briefLogLine != "" {
			*briefErrorsArray = append(*briefErrorsArray, briefLogLine)
		}

		// when it is a log line generated by the processor
		formattedProcessorLogs += currentLogLine + "\n"
	}

	*briefErrorsArray = ap.aggregateConsecutiveDuplicateMessages(*briefErrorsArray)

	// create brief errors log as string, and remove double newlines
	briefErrorsMessage = strings.Join(*briefErrorsArray, "\n")

	return common.FixEscapeChars(formattedProcessorLogs), common.FixEscapeChars(briefErrorsMessage)
}

func (ap *Platform) WaitForProjectResourcesDeletion(ctx context.Context, projectMeta *platform.ProjectMeta, duration time.Duration) error {
	if err := common.RetryUntilSuccessful(duration,
		5*time.Second,
		func() bool {
			functions, APIGateways, err := ap.GetProjectResources(ctx, projectMeta)
			if err != nil {
				ap.Logger.WarnWithCtx(ctx, "Failed to get project resources",
					"err", err)
				return false
			}
			if len(functions) > 0 || len(APIGateways) > 0 {
				ap.Logger.DebugWithCtx(ctx, "Waiting for project resources to be deleted",
					"functionsLen", len(functions),
					"apiGatewayLen", len(APIGateways))
				return false
			}
			return true
		}); err != nil {
		return errors.Wrap(err, "Failed waiting for resource deletion, attempts exhausted")
	}
	return nil
}

func (ap *Platform) GetProjectResources(ctx context.Context, projectMeta *platform.ProjectMeta) ([]platform.Function,
	[]platform.APIGateway,
	error) {

	var err error
	var functions []platform.Function
	var apiGateways []platform.APIGateway
	errGroup, _ := errgroup.WithContext(ctx, ap.Logger)

	// get api gateways
	errGroup.Go("GetAPIGateways", func() error {
		apiGateways, err = ap.platform.GetAPIGateways(ctx, &platform.GetAPIGatewaysOptions{
			Namespace: projectMeta.Namespace,
			Labels:    fmt.Sprintf("%s=%s", common.NuclioResourceLabelKeyProjectName, projectMeta.Name),
		})
		if err != nil {
			return errors.Wrap(err, "Failed to get project api gateways")
		}
		return nil
	})

	// get functions
	errGroup.Go("GetFunctions", func() error {
		functions, err = ap.platform.GetFunctions(ctx, &platform.GetFunctionsOptions{
			Namespace: projectMeta.Namespace,
			Labels:    fmt.Sprintf("%s=%s", common.NuclioResourceLabelKeyProjectName, projectMeta.Name),
		})
		if err != nil {
			return errors.Wrap(err, "Failed to get project functions")
		}
		return nil
	})

	if err := errGroup.Wait(); err != nil {
		return nil, nil, errors.Wrap(err, "Failed to get project resources")
	}
	return functions, apiGateways, nil
}

func (ap *Platform) EnsureDefaultProjectExistence(ctx context.Context) error {
	resolvedNamespace := ap.platform.ResolveDefaultNamespace(ap.DefaultNamespace)

	projects, err := ap.platform.GetProjects(ctx, &platform.GetProjectsOptions{
		Meta: platform.ProjectMeta{
			Name:      platform.DefaultProjectName,
			Namespace: resolvedNamespace,
		},
	})
	if err != nil {
		return errors.Wrap(err, "Failed to get projects")
	}

	if len(projects) == 0 {

		// if we're here the default project doesn't exist. create it
		projectConfig := platform.ProjectConfig{
			Meta: platform.ProjectMeta{
				Name:      platform.DefaultProjectName,
				Namespace: resolvedNamespace,
			},
			Spec: platform.ProjectSpec{},
		}
		newProject, err := platform.NewAbstractProject(ap.Logger, ap.platform, projectConfig)
		if err != nil {
			return errors.Wrap(err, "Failed to create abstract default project")
		}

		if err := ap.platform.CreateProject(ctx, &platform.CreateProjectOptions{
			ProjectConfig: newProject.GetConfig(),
		}); err != nil {

			// if project already exists, return
			if apierrors.IsAlreadyExists(errors.RootCause(err)) {
				return nil
			}

			return errors.Wrap(err, "Failed to create default project")
		}

		ap.Logger.DebugWithCtx(ctx, "Default project was successfully created",
			"name", platform.DefaultProjectName,
			"namespace", resolvedNamespace)
	}

	return nil
}

// ResolveProjectNameFromLabelsStr resolves first project name from label string
func (ap *Platform) ResolveProjectNameFromLabelsStr(encodedLabels string) (string, error) {
	labelSelector, err := labels.Parse(encodedLabels)
	if err != nil {
		return "", errors.Wrap(err, "Failed to parse encoded labels")
	}
	requirements, _ := labelSelector.Requirements()
	for _, requirement := range requirements {
		if requirement.Key() == common.NuclioResourceLabelKeyProjectName {
			return requirement.Values().List()[0], nil
		}
	}
	return "", nil
}

func (ap *Platform) EnsureProjectRead(projectName string,
	permissionOptions *opa.PermissionOptions) error {

	if projectName != "" && len(permissionOptions.MemberIds) > 0 {
		if _, err := ap.QueryOPAProjectPermissions(projectName,
			opa.ActionRead,
			&opa.PermissionOptions{
				MemberIds:           permissionOptions.MemberIds,
				OverrideHeaderValue: permissionOptions.OverrideHeaderValue,
				RaiseForbidden:      true,
			}); err != nil {
			return errors.Wrap(err, "Failed authorizing OPA permissions for resource")
		}
	}
	return nil
}

func (ap *Platform) QueryOPAProjectPermissions(projectName string,
	action opa.Action,
	permissionOptions *opa.PermissionOptions) (bool, error) {
	return ap.queryOPAPermissions(opa.GenerateProjectResourceString(projectName),
		action,
		permissionOptions)
}

func (ap *Platform) QueryOPAFunctionPermissions(projectName,
	functionName string,
	action opa.Action,
	permissionOptions *opa.PermissionOptions) (bool, error) {
	if projectName == "" {
		projectName = "*"
	}
	if functionName == "" {
		functionName = "*"
	}
	return ap.queryOPAPermissions(opa.GenerateFunctionResourceString(projectName, functionName),
		action,
		permissionOptions)
}

func (ap *Platform) QueryOPAFunctionEventPermissions(projectName,
	functionName,
	functionEventName string,
	action opa.Action,
	permissionOptions *opa.PermissionOptions) (bool, error) {
	if projectName == "" {
		projectName = "*"
	}
	if functionName == "" {
		functionName = "*"
	}
	if functionEventName == "" {
		functionEventName = "*"
	}
	return ap.queryOPAPermissions(opa.GenerateFunctionEventResourceString(projectName, functionName, functionEventName),
		action,
		permissionOptions)
}

func (ap *Platform) QueryOPAMultipleResources(resources []string,
	action opa.Action,
	permissionOptions *opa.PermissionOptions) ([]bool, error) {
	return ap.queryOPAPermissionsMultiResources(resources, action, permissionOptions)
}

func (ap *Platform) functionBuildRequired(functionConfig *functionconfig.Config) (bool, error) {

	// if neverBuild was passed explicitly don't build
	if functionConfig.Spec.Build.Mode == functionconfig.NeverBuild {
		return false, nil
	}

	// if the function contains source code, an image name or a path somewhere - we need to rebuild. the shell
	// runtime supports a case where user just tells image name and we build around the handler without a need
	// for a path
	if functionConfig.Spec.Build.FunctionSourceCode != "" ||
		functionConfig.Spec.Build.Path != "" ||
		functionConfig.Spec.Build.Image != "" {
		return true, nil
	}

	if functionConfig.Spec.Build.CodeEntryType == build.S3EntryType {
		return true, nil
	}

	// if user didn't give any of the above but _did_ specify an image to run from, just dont build
	if functionConfig.Spec.Image != "" {
		return false, nil
	}

	// should not get here - we should either be able to build an image or have one specified for us
	return false, errors.New("Function must have either spec.build.path," +
		"spec.build.functionSourceCode, spec.build.image or spec.image set in order to create")
}

func (ap *Platform) aggregateConsecutiveDuplicateMessages(errorMessagesArray []string) []string {
	var aggregatedErrorsArray []string

	for i := 0; i < len(errorMessagesArray); i++ {
		currentErrorMessage := errorMessagesArray[i]
		consecutiveErrorMessageCount := 1

		// count how many consecutive times current error message reoccurs
		for i+1 < len(errorMessagesArray) && errorMessagesArray[i+1] == currentErrorMessage {
			consecutiveErrorMessageCount++
			i++
		}

		if consecutiveErrorMessageCount > 1 {
			aggregatedErrorsArray = append(aggregatedErrorsArray,
				fmt.Sprintf("[repeated %d times] %s", consecutiveErrorMessageCount, currentErrorMessage))
			continue
		}

		aggregatedErrorsArray = append(aggregatedErrorsArray, currentErrorMessage)
	}

	return aggregatedErrorsArray
}

// If a user specify the image name to be built - add "projectName-functionName-" prefix to it
func (ap *Platform) enrichImageName(functionConfig *functionconfig.Config) error {
	if ap.ImageNamePrefixTemplate == "" {
		return nil
	}
	functionName := functionConfig.Meta.Name
	projectName := functionConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName]

	functionBuildRequired, err := ap.functionBuildRequired(functionConfig)
	if err != nil {
		return errors.Wrap(err, "Failed determining whether function build is required for image name enrichment")
	}

	// if build is not required or custom image name was not asked enrichment is irrelevant
	// note that leaving Spec.Build.Image will cause further enrichment deeper in build/builder.go.
	// TODO: Revisit the need for this logic being stretched on so many places
	if !functionBuildRequired || functionConfig.Spec.Build.Image == "" {
		return nil
	}

	imagePrefix, err := ap.RenderImageNamePrefixTemplate(projectName, functionName)

	if err != nil {
		return errors.Wrap(err, "Failed to render image name prefix template")
	}

	// avoid re-enrichment
	if !strings.HasPrefix(functionConfig.Spec.Build.Image, imagePrefix) {

		functionConfig.Spec.Build.Image = fmt.Sprintf("%s%s",
			imagePrefix, functionConfig.Spec.Build.Image)
	}

	return nil
}

func (ap *Platform) validateMinMaxReplicas(functionConfig *functionconfig.Config) error {
	minReplicas := functionConfig.Spec.MinReplicas
	maxReplicas := functionConfig.Spec.MaxReplicas

	if minReplicas != nil {
		if maxReplicas == nil && *minReplicas == 0 {
			return nuclio.NewErrBadRequest("Max replicas must be set when min replicas is zero")
		}
		if maxReplicas != nil && *minReplicas > *maxReplicas {
			return nuclio.NewErrBadRequest("Min replicas must be less than or equal to max replicas")
		}
	}
	if maxReplicas != nil && *maxReplicas == 0 {
		return nuclio.NewErrBadRequest("Max replicas must be greater than zero")
	}

	return nil
}

func (ap *Platform) validateNodeSelector(functionConfig *functionconfig.Config) error {
	for labelKey, labelValue := range functionConfig.Spec.NodeSelector {
		if errs := validation.IsValidLabelValue(labelValue); len(errs) > 0 {
			errs = append([]string{fmt.Sprintf("Invalid value: %s", labelValue)}, errs...)
			return nuclio.NewErrBadRequest(strings.Join(errs, ", "))
		}

		// Valid label keys have two segments: an optional prefix and name, separated by a slash (/).
		// The name segment is required and must conform to the rules of a valid label value.
		// The prefix is optional. If specified, the prefix must be a DNS subdomain.
		if errs := validation.IsQualifiedName(labelKey); len(errs) > 0 {
			errs = append([]string{fmt.Sprintf("Invalid key: %s", labelKey)}, errs...)
			return nuclio.NewErrBadRequest(strings.Join(errs, ", "))
		}
	}
	return nil
}

func (ap *Platform) validatePriorityClassName(functionConfig *functionconfig.Config) error {

	// function uses default class name
	if functionConfig.Spec.PriorityClassName == ap.Config.Kube.DefaultFunctionPriorityClassName {
		return nil
	}

	// look for class name in list of valid class names
	if ap.Config.Kube.ValidFunctionPriorityClassNames != nil && !common.StringSliceContainsString(
		ap.Config.Kube.ValidFunctionPriorityClassNames,
		functionConfig.Spec.PriorityClassName) {
		return nuclio.NewErrBadRequest(fmt.Sprintf(
			"Priority class name `%s` is not in valid priority class names list: [%s]",
			functionConfig.Spec.PriorityClassName,
			strings.Join(ap.Config.Kube.ValidFunctionPriorityClassNames, ", ")))
	}
	return nil
}

func (ap *Platform) validateVolumes(ctx context.Context, functionConfig *functionconfig.Config) error {

	// volume mount can be shared by many volumes (e.g.: mount volume X in /here and /there)
	volumeNameToVolumeMounts := map[string][]v1.Volume{}
	for _, configVolume := range functionConfig.Spec.Volumes {
		if configVolume.VolumeMount.Name == "" {
			return nuclio.NewErrBadRequest("Volume mount name is missing")
		}
		if configVolume.Volume.Name == "" {
			return nuclio.NewErrBadRequest("Volume name is missing")
		}

		if configVolume.VolumeMount.Name != configVolume.Volume.Name {
			return nuclio.NewErrBadRequest("Volume and volume mount must have the same name")
		}

		// aggregate volumes by the volume mount they refer to
		volumeNameToVolumeMounts[configVolume.VolumeMount.Name] = append(
			volumeNameToVolumeMounts[configVolume.VolumeMount.Name],
			configVolume.Volume)
	}

	// make sure all volumes sharing the same volume mount are the same to ensure invalid mode
	// where different volumes sharing the same volume mount
	for volumeMountName, volumes := range volumeNameToVolumeMounts {

		// irrelevant check for a single volume
		if len(volumes) <= 1 {
			continue
		}

		// make sure the first volume equals all the rest volumes sharing the same volume mount
		firstVolume := volumes[0]
		for _, volume := range volumes[1:] {
			if volumeDiff := cmp.Diff(firstVolume, volume); volumeDiff != "" {
				ap.Logger.WarnWithCtx(ctx, "Invalid volumes configuration found",
					"volumeMountName", volumeMountName,
					"volumeDiff", volumeDiff)
				return nuclio.NewErrBadRequest(
					fmt.Sprintf("Volumes sharing the same volume mount '%s' must having the same configuration",
						volumeMountName))
			}
		}
	}
	return nil
}

func (ap *Platform) validateProjectExists(ctx context.Context, functionConfig *functionconfig.Config) error {

	// validate the project exists
	getProjectsOptions := &platform.GetProjectsOptions{
		Meta: platform.ProjectMeta{
			Name:      functionConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName],
			Namespace: functionConfig.Meta.Namespace,
		},
	}

	// NOTE: This is a temporary hack
	// we perform a validation for project existence only, we want to make sure
	// that the project exists so we set up the request origin as it came from a leader
	if ap.Config.ProjectsLeader != nil {
		getProjectsOptions.RequestOrigin = ap.Config.ProjectsLeader.Kind
	}

	projects, err := ap.platform.GetProjects(ctx, getProjectsOptions)
	if err != nil {
		return errors.Wrap(err, "Failed to get projects")
	}

	if len(projects) == 0 {
		return nuclio.NewErrPreconditionFailed("Project does not exist")
	}
	return nil
}

func (ap *Platform) validateTriggers(functionConfig *functionconfig.Config) error {
	var httpTriggerExists bool

	// validate ingresses structure correctness
	if err := ap.validateIngresses(functionConfig.Spec.Triggers); err != nil {
		return errors.Wrap(err, "Ingresses validation failed")
	}

	for triggerKey, triggerInstance := range functionConfig.Spec.Triggers {

		// do not allow trigger with empty name
		if triggerKey == "" {
			return nuclio.NewErrBadRequest("Trigger key can not be empty")
		}

		// trigger key and name must match
		if triggerKey != triggerInstance.Name {
			return nuclio.NewErrBadRequest(fmt.Sprintf("Trigger key and name mismatch (%s != %s)",
				triggerKey, triggerInstance.Name))
		}

		// no more workers than limitation allows
		if triggerInstance.MaxWorkers > trigger.MaxWorkersLimit {
			return nuclio.NewErrBadRequest(fmt.Sprintf("MaxWorkers value for %s trigger (%d) exceeds the limit of %d",
				triggerKey,
				triggerInstance.MaxWorkers,
				trigger.MaxWorkersLimit))
		}

		// no more than one http trigger is allowed
		if triggerInstance.Kind == "http" {
			if !httpTriggerExists {
				httpTriggerExists = true
				continue
			}
			return nuclio.NewErrBadRequest("There's more than one http trigger (unsupported)")
		}
	}

	return nil
}

func (ap *Platform) validateIngresses(triggers map[string]functionconfig.Trigger) error {
	for triggerName, triggerInstance := range functionconfig.GetTriggersByKind(triggers, "http") {

		// if there are ingresses
		if encodedIngresses, found := triggerInstance.Attributes["ingresses"]; found {

			// validate ingresses structure
			encodedIngresses, validStructure := encodedIngresses.(map[string]interface{})
			if !validStructure {
				return nuclio.NewErrBadRequest(fmt.Sprintf("Malformed structure for ingresses in trigger '%s' (expects a map)", triggerName))
			}

			for encodedIngressName, encodedIngress := range encodedIngresses {

				// validate each ingress structure
				if _, validStructure := encodedIngress.(map[string]interface{}); !validStructure {
					return nuclio.NewErrBadRequest(fmt.Sprintf("Malformed structure for ingress '%s' in trigger '%s'", encodedIngressName, triggerName))
				}
			}
		}
	}

	return nil
}

func (ap *Platform) enrichMinMaxReplicas(functionConfig *functionconfig.Config) {

	// if min replicas was not set, and max replicas is set, assign max replicas to min replicas
	if functionConfig.Spec.MinReplicas == nil &&
		functionConfig.Spec.MaxReplicas != nil {
		functionConfig.Spec.MinReplicas = functionConfig.Spec.MaxReplicas
	}

	// if max replicas was not set, and min replicas is set, assign min replicas to max replicas
	if functionConfig.Spec.MaxReplicas == nil &&
		functionConfig.Spec.MinReplicas != nil {
		functionConfig.Spec.MaxReplicas = functionConfig.Spec.MinReplicas
	}
}

func (ap *Platform) enrichTriggers(functionConfig *functionconfig.Config) error {

	// add default http trigger if missing http trigger
	ap.enrichDefaultHTTPTrigger(functionConfig)

	for triggerName, triggerInstance := range functionConfig.Spec.Triggers {

		// if name was not given, inherit its key
		if triggerInstance.Name == "" {
			triggerInstance.Name = triggerName
		}

		// ensure having max workers
		if common.StringInSlice(triggerInstance.Kind, []string{"http", "v3ioStream"}) {
			if triggerInstance.MaxWorkers == 0 {
				triggerInstance.MaxWorkers = 1
			}
		}

		functionConfig.Spec.Triggers[triggerName] = triggerInstance
	}
	return nil
}

// returns overrides for base images per runtime
func (ap *Platform) getBaseImagesOverrides() map[string]string {
	return ap.Config.ImageRegistryOverrides.BaseImageRegistries
}

// returns overrides for base images per runtime
func (ap *Platform) getOnbuildImagesOverrides() map[string]string {
	return ap.Config.ImageRegistryOverrides.OnbuildImageRegistries
}

func (ap *Platform) validateDockerImageFields(ctx context.Context, functionConfig *functionconfig.Config) error {

	// here we sanitize registry/image fields for malformed or potentially malicious inputs
	for fieldName, fieldValue := range map[string]*string{
		"Spec.Image":                   &functionConfig.Spec.Image,
		"Spec.RunRegistry":             &functionConfig.Spec.RunRegistry,
		"Spec.Build.Image":             &functionConfig.Spec.Build.Image,
		"Spec.Build.OnbuildImage":      &functionConfig.Spec.Build.OnbuildImage,
		"Spec.Build.Registry":          &functionConfig.Spec.Build.Registry,
		"Spec.Build.BaseImageRegistry": &functionConfig.Spec.Build.BaseImageRegistry,
	} {
		if *fieldValue != "" {

			// HACK: cleanup possible trailing /
			valueToValidate := strings.TrimSuffix(*fieldValue, "/")
			if _, err := reference.Parse(valueToValidate); err != nil {
				ap.Logger.WarnWithCtx(ctx, "Invalid docker image ref passed in spec field - this may be malicious",
					"err", err,
					"fieldName", fieldName,
					"fieldValue", fieldValue)

				// if this is invalid it might also ruin the response serialization - clean out the offending field
				*fieldValue = ""

				// do not return "err" itself as root cause, to avoid confusion when returning the error to the user
				// note: err is being logged above.
				return nuclio.NewErrBadRequest(fmt.Sprintf("Invalid %s passed", fieldName))
			}
		}
	}

	return nil
}

func (ap *Platform) queryOPAPermissionsMultiResources(resources []string,
	action opa.Action,
	permissionOptions *opa.PermissionOptions) ([]bool, error) {

	allowedList, err := ap.OpaClient.QueryPermissionsMultiResources(resources, action, permissionOptions)
	if err != nil {
		return nil, nuclio.WrapErrInternalServerError(err)
	}

	for idx, allowed := range allowedList {
		if !allowed && permissionOptions.RaiseForbidden {
			return nil, nuclio.NewErrForbidden(fmt.Sprintf("Not allowed to %s resource %s",
				action,
				resources[idx]))
		}
	}

	return allowedList, nil
}

func (ap *Platform) queryOPAPermissions(resource string,
	action opa.Action,
	permissionOptions *opa.PermissionOptions) (bool, error) {

	allowed, err := ap.OpaClient.QueryPermissions(resource, action, permissionOptions)
	if err != nil {
		return allowed, nuclio.WrapErrInternalServerError(err)
	}
	if !allowed && permissionOptions.RaiseForbidden {
		return false, nuclio.NewErrForbidden(fmt.Sprintf("Not allowed to %s resource %s", action, resource))
	}
	return allowed, nil
}

func (ap *Platform) enrichVolumes(functionConfig *functionconfig.Config) error {
	for _, configVolume := range functionConfig.Spec.Volumes {

		// fill volume mount name from its volume
		if configVolume.VolumeMount.Name == "" {
			configVolume.VolumeMount.Name = configVolume.Volume.Name
		}

		// fill volume name from its volume mount
		if configVolume.Volume.Name == "" {
			configVolume.Volume.Name = configVolume.VolumeMount.Name
		}
	}
	return nil
}
