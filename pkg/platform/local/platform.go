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

package local

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/opa"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project"
	externalproject "github.com/nuclio/nuclio/pkg/platform/abstract/project/external"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project/internalc/local"
	"github.com/nuclio/nuclio/pkg/platform/local/client"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/config"

	"github.com/ghodss/yaml"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/nuclio/zap"
)

type Platform struct {
	*abstract.Platform
	cmdRunner                cmdrunner.CmdRunner
	dockerClient             dockerclient.Client
	localStore               *client.Store
	defaultFunctionMountMode FunctionMountMode
	projectsClient           project.Client
}

const Mib = 1048576
const FunctionProcessorContainerDirPath = "/etc/nuclio/config/processor"

func NewProjectsClient(platform *Platform, platformConfiguration *platformconfig.Config) (project.Client, error) {

	// create local projects client
	localProjectsClient, err := local.NewClient(platform.Logger, platform, platform.localStore)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create internal projects client (local)")
	}

	if platformConfiguration.ProjectsLeader != nil {

		// wrap external client around local projects client as internal client
		return externalproject.NewClient(platform.Logger, localProjectsClient, platformConfiguration)
	}

	return localProjectsClient, nil
}

// NewPlatform instantiates a new local platform
func NewPlatform(ctx context.Context,
	parentLogger logger.Logger,
	platformConfiguration *platformconfig.Config,
	defaultNamespace string) (*Platform, error) {
	newPlatform := &Platform{}

	// create base
	newAbstractPlatform, err := abstract.NewPlatform(parentLogger, newPlatform, platformConfiguration, defaultNamespace)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create an abstract platform")
	}

	// init platform
	newPlatform.Platform = newAbstractPlatform

	// create a command runner
	if newPlatform.cmdRunner, err = cmdrunner.NewShellRunner(newPlatform.Logger); err != nil {
		return nil, errors.Wrap(err, "Failed to create a command runner")
	}

	if newPlatform.ContainerBuilder, err = containerimagebuilderpusher.NewDocker(newPlatform.Logger,
		platformConfiguration.ContainerBuilderConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to create containerimagebuilderpusher")
	}

	// create a docker client
	if newPlatform.dockerClient, err = dockerclient.NewShellClient(newPlatform.Logger, nil); err != nil {
		return nil, errors.Wrap(err, "Failed to create a Docker client")
	}

	// create a local store for configs and stuff
	if newPlatform.localStore, err = client.NewStore(parentLogger, newPlatform, newPlatform.dockerClient); err != nil {
		return nil, errors.Wrap(err, "Failed to create a local store")
	}

	// create projects client
	newPlatform.projectsClient, err = NewProjectsClient(newPlatform, platformConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create projects client")
	}

	// ignite goroutine to check function container healthiness
	if newPlatform.Config.Local.FunctionContainersHealthinessEnabled {
		newPlatform.Logger.DebugWithCtx(ctx, "Igniting container healthiness validator")
		go func(newPlatform *Platform) {
			uptimeTicker := time.NewTicker(newPlatform.Config.Local.FunctionContainersHealthinessInterval)
			for range uptimeTicker.C {
				newPlatform.ValidateFunctionContainersHealthiness(ctx)
			}
		}(newPlatform)
	}

	// Default to mount function configurations from docker volume
	newPlatform.defaultFunctionMountMode = FunctionMountMode(
		common.GetEnvOrDefaultString("NUCLIO_DASHBOARD_DEFAULT_FUNCTION_MOUNT_MODE", string(FunctionMountModeVolume)),
	)
	return newPlatform, nil
}

func (p *Platform) Initialize(ctx context.Context) error {
	if err := p.projectsClient.Initialize(); err != nil {
		return errors.Wrap(err, "Failed to initialize projects client")
	}

	// ensure default project existence only when projects aren't managed by external leader
	if p.Config.ProjectsLeader == nil {
		if err := p.EnsureDefaultProjectExistence(ctx); err != nil {
			return errors.Wrap(err, "Failed to ensure default project existence")
		}
	}

	return nil
}

// CreateFunction will simply run a docker image
func (p *Platform) CreateFunction(ctx context.Context, createFunctionOptions *platform.CreateFunctionOptions) (
	*platform.CreateFunctionResult, error) {
	var previousHTTPPort int
	var err error
	var existingFunctionConfig *functionconfig.ConfigWithStatus

	if err := p.enrichAndValidateFunctionConfig(ctx, &createFunctionOptions.FunctionConfig); err != nil {
		return nil, errors.Wrap(err, "Failed to enrich and validate a function configuration")
	}

	// Check OPA permissions
	permissionOptions := createFunctionOptions.PermissionOptions
	permissionOptions.RaiseForbidden = true
	if _, err := p.QueryOPAFunctionPermissions(createFunctionOptions.FunctionConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName],
		createFunctionOptions.FunctionConfig.Meta.Name,
		opa.ActionCreate,
		&permissionOptions); err != nil {
		return nil, errors.Wrap(err, "Failed authorizing OPA permissions for resource")
	}

	// local currently doesn't support registries of any kind. remove push / run registry
	createFunctionOptions.FunctionConfig.Spec.RunRegistry = ""
	createFunctionOptions.FunctionConfig.Spec.Build.Registry = ""

	// it's possible to pass a function without specifying any meta in the request, in that case skip getting existing function
	if createFunctionOptions.FunctionConfig.Meta.Namespace != "" && createFunctionOptions.FunctionConfig.Meta.Name != "" {
		existingFunctions, err := p.localStore.GetFunctions(&createFunctionOptions.FunctionConfig.Meta)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get existing functions")
		}

		if len(existingFunctions) == 0 {
			existingFunctionConfig = nil
		} else {

			// assume only one
			existingFunction := existingFunctions[0]

			// build function options
			existingFunctionConfig = &functionconfig.ConfigWithStatus{
				Config: *existingFunction.GetConfig(),
				Status: *existingFunction.GetStatus(),
			}
		}
	}

	// if function exists, perform some validation with new function create options
	if err := p.ValidateCreateFunctionOptionsAgainstExistingFunctionConfig(ctx,
		existingFunctionConfig,
		createFunctionOptions); err != nil {
		return nil, errors.Wrap(err, "Failed to validate a function configuration against an existing configuration")
	}

	// wrap logger
	logStream, err := abstract.NewLogStream("deployer", nucliozap.InfoLevel, createFunctionOptions.Logger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create a log stream")
	}

	// save the log stream for the name
	p.DeployLogStreams.Store(createFunctionOptions.FunctionConfig.Meta.GetUniqueID(), logStream)

	// replace logger
	createFunctionOptions.Logger = logStream.GetLogger()

	reportCreationError := func(creationError error) error {
		createFunctionOptions.Logger.WarnWithCtx(ctx, "Failed to create a function; setting the function status",
			"err", creationError)

		errorStack := bytes.Buffer{}
		errors.PrintErrorStack(&errorStack, creationError, 20)

		// cut messages that are too big
		if errorStack.Len() >= 4*Mib {
			errorStack.Truncate(4 * Mib)
		}

		// post logs and error
		return p.localStore.CreateOrUpdateFunction(&functionconfig.ConfigWithStatus{
			Config: createFunctionOptions.FunctionConfig,
			Status: functionconfig.Status{
				State:   functionconfig.FunctionStateError,
				Message: errorStack.String(),
			},
		})
	}

	onAfterConfigUpdated := func() error {
		createFunctionOptions.Logger.DebugWithCtx(ctx, "Creating shadow function",
			"name", createFunctionOptions.FunctionConfig.Meta.Name)

		// enrich and validate again because it may not be valid after config was updated by external code entry type
		if err := p.enrichAndValidateFunctionConfig(ctx, &createFunctionOptions.FunctionConfig); err != nil {
			return errors.Wrap(err, "Failed to enrich and validate the updated function configuration")
		}

		// create the function in the store
		if err := p.localStore.CreateOrUpdateFunction(&functionconfig.ConfigWithStatus{
			Config: createFunctionOptions.FunctionConfig,
			Status: functionconfig.Status{
				State: functionconfig.FunctionStateBuilding,
			},
		}); err != nil {
			return errors.Wrap(err, "Failed to create a function")
		}

		previousHTTPPort, err = p.deletePreviousContainers(createFunctionOptions)
		if err != nil {
			return errors.Wrap(err, "Failed to delete previous containers")
		}

		// indicate that the creation state has been updated. local platform has no "building" state yet
		if createFunctionOptions.CreationStateUpdated != nil {
			createFunctionOptions.CreationStateUpdated <- true
		}

		return nil
	}

	onAfterBuild := func(buildResult *platform.CreateFunctionBuildResult, buildErr error) (*platform.CreateFunctionResult, error) {
		if buildErr != nil {
			reportCreationError(buildErr) // nolint: errcheck
			return nil, buildErr
		}

		skipFunctionDeploy := functionconfig.ShouldSkipDeploy(createFunctionOptions.FunctionConfig.Meta.Annotations)

		// after a function build (or skip-build) if the annotations FunctionAnnotationSkipBuild or FunctionAnnotationSkipDeploy
		// exist, they should be removed so next time, the build will happen.
		createFunctionOptions.FunctionConfig.Meta.RemoveSkipDeployAnnotation()
		createFunctionOptions.FunctionConfig.Meta.RemoveSkipBuildAnnotation()

		var createFunctionResult *platform.CreateFunctionResult
		var deployErr error
		var functionStatus functionconfig.Status

		if !skipFunctionDeploy {
			createFunctionResult, deployErr = p.deployFunction(createFunctionOptions, previousHTTPPort)
			if deployErr != nil {
				reportCreationError(deployErr) // nolint: errcheck
				return nil, deployErr
			}

			functionStatus.HTTPPort = createFunctionResult.Port
			functionStatus.State = functionconfig.FunctionStateReady

			if err := p.populateFunctionInvocationStatus(&functionStatus, createFunctionResult); err != nil {
				return nil, errors.Wrap(err, "Failed to populate function invocation status")
			}
		} else {
			p.Logger.InfoCtx(ctx, "Skipping function deployment")
			functionStatus.State = functionconfig.FunctionStateImported
			createFunctionResult = &platform.CreateFunctionResult{
				CreateFunctionBuildResult: platform.CreateFunctionBuildResult{
					Image:                 createFunctionOptions.FunctionConfig.Spec.Image,
					UpdatedFunctionConfig: createFunctionOptions.FunctionConfig,
				},
			}
		}

		// update the function
		if err := p.localStore.CreateOrUpdateFunction(&functionconfig.ConfigWithStatus{
			Config: createFunctionOptions.FunctionConfig,
			Status: functionStatus,
		}); err != nil {
			return nil, errors.Wrap(err, "Failed to update a function with state")
		}

		createFunctionResult.FunctionStatus = functionStatus
		return createFunctionResult, nil
	}

	// If needed, load any docker image from archive into docker
	if createFunctionOptions.InputImageFile != "" {
		p.Logger.InfoWithCtx(ctx, "Loading docker image from archive",
			"input", createFunctionOptions.InputImageFile)
		if err := p.dockerClient.Load(createFunctionOptions.InputImageFile); err != nil {
			return nil, errors.Wrap(err, "Failed to load a Docker image from an archive")
		}
	}

	// wrap the deployer's deploy with the base HandleDeployFunction to provide lots of
	// common functionality
	return p.HandleDeployFunction(ctx, existingFunctionConfig, createFunctionOptions, onAfterConfigUpdated, onAfterBuild)
}

// GetFunctions will return deployed functions
func (p *Platform) GetFunctions(ctx context.Context,
	getFunctionsOptions *platform.GetFunctionsOptions) ([]platform.Function, error) {

	projectName, err := p.Platform.ResolveProjectNameFromLabelsStr(getFunctionsOptions.Labels)
	if err != nil {
		return nil, errors.Wrap(err, "")
	}

	if err := p.Platform.EnsureProjectRead(projectName, &getFunctionsOptions.PermissionOptions); err != nil {
		return nil, errors.Wrap(err, "Failed to ensure project read permission")
	}

	functions, err := p.localStore.GetProjectFunctions(getFunctionsOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read functions from a local store")
	}

	functions, err = p.Platform.FilterFunctionsByPermissions(ctx, &getFunctionsOptions.PermissionOptions, functions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to filter functions by permissions")
	}

	// enrich with build logs
	p.EnrichFunctionsWithDeployLogStream(functions)

	return functions, nil
}

// UpdateFunction will update a previously deployed function
func (p *Platform) UpdateFunction(ctx context.Context, updateFunctionOptions *platform.UpdateFunctionOptions) error {
	return nil
}

// DeleteFunction will delete a previously deployed function
func (p *Platform) DeleteFunction(ctx context.Context, deleteFunctionOptions *platform.DeleteFunctionOptions) error {

	// pre delete validation
	functionToDelete, err := p.ValidateDeleteFunctionOptions(ctx, deleteFunctionOptions)
	if err != nil {
		return errors.Wrap(err, "Failed to validate function-deletion options")
	}

	// nothing to delete
	if functionToDelete == nil {
		return nil
	}

	// actual function and its resources deletion
	return p.delete(ctx, deleteFunctionOptions)
}

func (p *Platform) GetFunctionReplicaLogsStream(ctx context.Context,
	options *platform.GetFunctionReplicaLogsStreamOptions) (io.ReadCloser, error) {

	sinceDuration := ""
	if options.SinceSeconds != nil {
		sinceDuration = (time.Second * time.Duration(*options.SinceSeconds)).String()
	}

	tail := ""
	if options.TailLines != nil {
		tail = strconv.FormatInt(*options.TailLines, 10)
	}

	return p.dockerClient.GetContainerLogStream(ctx,
		options.Name,
		&dockerclient.ContainerLogsOptions{
			Follow: options.Follow,
			Since:  sinceDuration,
			Tail:   tail,
		})
}

func (p *Platform) GetFunctionReplicaNames(ctx context.Context,
	functionConfig *functionconfig.Config) ([]string, error) {
	return []string{
		p.GetFunctionContainerName(functionConfig),
	}, nil
}

// GetHealthCheckMode returns the healthcheck mode the platform requires
func (p *Platform) GetHealthCheckMode() platform.HealthCheckMode {

	// The internal client needs to perform the health check
	return platform.HealthCheckModeInternalClient
}

// GetName returns the platform name
func (p *Platform) GetName() string {
	return "local"
}

func (p *Platform) GetNodes() ([]platform.Node, error) {

	// just create a single node
	return []platform.Node{&node{}}, nil
}

// CreateProject will create a new project
func (p *Platform) CreateProject(ctx context.Context, createProjectOptions *platform.CreateProjectOptions) error {

	// enrich
	if err := p.EnrichCreateProjectConfig(createProjectOptions); err != nil {
		return errors.Wrap(err, "Failed to enrich a project configuration")
	}

	// validate
	if err := p.ValidateProjectConfig(createProjectOptions.ProjectConfig); err != nil {
		return errors.Wrap(err, "Failed to validate a project configuration")
	}

	// create
	if _, err := p.projectsClient.Create(ctx, createProjectOptions); err != nil {
		return errors.Wrap(err, "Failed to create project")
	}

	return nil
}

// UpdateProject will update an existing project
func (p *Platform) UpdateProject(ctx context.Context, updateProjectOptions *platform.UpdateProjectOptions) error {
	if err := p.ValidateProjectConfig(&updateProjectOptions.ProjectConfig); err != nil {
		return nuclio.WrapErrBadRequest(err)
	}

	if _, err := p.projectsClient.Update(ctx, updateProjectOptions); err != nil {
		return errors.Wrap(err, "Failed to update project")
	}

	return nil
}

// DeleteProject will delete an existing project
func (p *Platform) DeleteProject(ctx context.Context, deleteProjectOptions *platform.DeleteProjectOptions) error {
	if err := p.Platform.ValidateDeleteProjectOptions(ctx, deleteProjectOptions); err != nil {
		return errors.Wrap(err, "Failed to validate delete project options")
	}

	// check only, do not delete
	if deleteProjectOptions.Strategy == platform.DeleteProjectStrategyCheck {
		p.Logger.DebugWithCtx(ctx, "Project is ready for deletion", "projectMeta", deleteProjectOptions.Meta)
		return nil
	}

	if err := p.projectsClient.Delete(ctx, deleteProjectOptions); err != nil {
		return errors.Wrapf(err, "Failed to delete project")
	}

	return nil
}

// GetProjects will list existing projects
func (p *Platform) GetProjects(ctx context.Context, getProjectsOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	projects, err := p.projectsClient.Get(ctx, getProjectsOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed getting projects")
	}

	return p.Platform.FilterProjectsByPermissions(&getProjectsOptions.PermissionOptions, projects)
}

// CreateFunctionEvent will create a new function event that can later be used as a template from
// which to invoke functions
func (p *Platform) CreateFunctionEvent(ctx context.Context, createFunctionEventOptions *platform.CreateFunctionEventOptions) error {
	if err := p.Platform.EnrichFunctionEvent(ctx, &createFunctionEventOptions.FunctionEventConfig); err != nil {
		return errors.Wrap(err, "Failed to enrich function event")
	}

	functionName := createFunctionEventOptions.FunctionEventConfig.Meta.Labels[common.NuclioResourceLabelKeyFunctionName]
	projectName := createFunctionEventOptions.FunctionEventConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName]

	// Check OPA permissions
	permissionOptions := createFunctionEventOptions.PermissionOptions
	permissionOptions.RaiseForbidden = true
	if _, err := p.QueryOPAFunctionEventPermissions(projectName,
		functionName,
		createFunctionEventOptions.FunctionEventConfig.Meta.Name,
		opa.ActionCreate,
		&permissionOptions); err != nil {
		return errors.Wrap(err, "Failed authorizing OPA permissions for resource")
	}

	return p.localStore.CreateOrUpdateFunctionEvent(&createFunctionEventOptions.FunctionEventConfig)
}

// UpdateFunctionEvent will update a previously existing function event
func (p *Platform) UpdateFunctionEvent(ctx context.Context, updateFunctionEventOptions *platform.UpdateFunctionEventOptions) error {
	if err := p.Platform.EnrichFunctionEvent(ctx, &updateFunctionEventOptions.FunctionEventConfig); err != nil {
		return errors.Wrap(err, "Failed to enrich function event")
	}

	functionEvents, err := p.localStore.GetFunctionEvents(&platform.GetFunctionEventsOptions{
		Meta: updateFunctionEventOptions.FunctionEventConfig.Meta,
	})
	if err != nil {
		return errors.Wrap(err, "Failed to read function events from a local store")
	}
	functionEventToUpdate := functionEvents[0]

	functionName := updateFunctionEventOptions.FunctionEventConfig.Meta.Labels[common.NuclioResourceLabelKeyFunctionName]
	projectName := updateFunctionEventOptions.FunctionEventConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName]

	// Check OPA permissions
	permissionOptions := updateFunctionEventOptions.PermissionOptions
	permissionOptions.RaiseForbidden = true
	if _, err := p.QueryOPAFunctionEventPermissions(projectName,
		functionName,
		functionEventToUpdate.GetConfig().Meta.Name,
		opa.ActionUpdate,
		&permissionOptions); err != nil {
		return errors.Wrap(err, "Failed authorizing OPA permissions for resource")
	}

	return p.localStore.CreateOrUpdateFunctionEvent(&updateFunctionEventOptions.FunctionEventConfig)
}

// DeleteFunctionEvent will delete a previously existing function event
func (p *Platform) DeleteFunctionEvent(ctx context.Context, deleteFunctionEventOptions *platform.DeleteFunctionEventOptions) error {
	functionEvents, err := p.localStore.GetFunctionEvents(&platform.GetFunctionEventsOptions{
		Meta: deleteFunctionEventOptions.Meta,
	})
	if err != nil {
		return errors.Wrap(err, "Failed to read function events from a local store")
	}

	if len(functionEvents) > 0 {
		functionEventToDelete := functionEvents[0]
		functionName := functionEventToDelete.GetConfig().Meta.Labels[common.NuclioResourceLabelKeyFunctionName]
		projectName := functionEventToDelete.GetConfig().Meta.Labels[common.NuclioResourceLabelKeyProjectName]

		// Check OPA permissions
		permissionOptions := deleteFunctionEventOptions.PermissionOptions
		permissionOptions.RaiseForbidden = true
		if _, err := p.QueryOPAFunctionEventPermissions(projectName,
			functionName,
			functionEventToDelete.GetConfig().Meta.Name,
			opa.ActionDelete,
			&permissionOptions); err != nil {
			return errors.Wrap(err, "Failed authorizing OPA permissions for resource")
		}
	}

	return p.localStore.DeleteFunctionEvent(&deleteFunctionEventOptions.Meta)
}

// GetFunctionEvents will list existing function events
func (p *Platform) GetFunctionEvents(ctx context.Context, getFunctionEventsOptions *platform.GetFunctionEventsOptions) ([]platform.FunctionEvent, error) {
	functionEvents, err := p.localStore.GetFunctionEvents(getFunctionEventsOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read function events from a local store")
	}

	return p.Platform.FilterFunctionEventsByPermissions(ctx,
		&getFunctionEventsOptions.PermissionOptions,
		functionEvents)
}

// GetAPIGateways not supported on this platform
func (p *Platform) GetAPIGateways(ctx context.Context, getAPIGatewaysOptions *platform.GetAPIGatewaysOptions) ([]platform.APIGateway, error) {
	return nil, nil
}

// GetExternalIPAddresses returns the external IP addresses invocations will use, if "via" is set to "external-ip".
// These addresses are either set through SetExternalIPAddresses or automatically discovered
func (p *Platform) GetExternalIPAddresses() ([]string, error) {

	// check if parent has addresses
	externalIPAddress, err := p.Platform.GetExternalIPAddresses()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get external IP addresses from parent")
	}

	// if the parent has something, use that
	if len(externalIPAddress) != 0 {
		return externalIPAddress, nil
	}

	// If the testing environment variable is set - use that
	if os.Getenv("NUCLIO_TEST_HOST") != "" {
		return []string{os.Getenv("NUCLIO_TEST_HOST")}, nil
	}

	if common.RunningInContainer() {
		return []string{"172.17.0.1"}, nil
	}

	// return an empty string to maintain backwards compatibility
	return []string{""}, nil
}

// ResolveDefaultNamespace returns the proper default resource namespace, given the current default namespace
func (p *Platform) ResolveDefaultNamespace(defaultNamespace string) string {

	// if no default namespace is chosen, use "nuclio"
	if defaultNamespace == "@nuclio.selfNamespace" || defaultNamespace == "" {
		return "nuclio"
	}

	return defaultNamespace
}

// GetNamespaces returns all the namespaces in the platform
func (p *Platform) GetNamespaces(ctx context.Context) ([]string, error) {
	return []string{"nuclio"}, nil
}

func (p *Platform) GetDefaultInvokeIPAddresses() ([]string, error) {
	return []string{"172.17.0.1"}, nil
}

func (p *Platform) SaveFunctionDeployLogs(ctx context.Context, functionName, namespace string) error {
	functions, err := p.GetFunctions(ctx,
		&platform.GetFunctionsOptions{
			Name:      functionName,
			Namespace: namespace,
		})
	if err != nil || len(functions) == 0 {
		return errors.Wrap(err, "Failed to get existing functions")
	}

	// enrich with build logs
	p.EnrichFunctionsWithDeployLogStream(functions)

	function := functions[0]

	return p.localStore.CreateOrUpdateFunction(&functionconfig.ConfigWithStatus{
		Config: *function.GetConfig(),
		Status: *function.GetStatus(),
	})
}

func (p *Platform) ValidateFunctionContainersHealthiness(ctx context.Context) {
	namespaces, err := p.GetNamespaces(ctx)
	if err != nil {
		p.Logger.WarnWith("Cannot not get namespaces", "err", err)
		return
	}

	for _, namespace := range namespaces {

		// get functions for that namespace
		functions, err := p.GetFunctions(ctx, &platform.GetFunctionsOptions{
			Namespace: namespace,
		})
		if err != nil {
			p.Logger.WarnWithCtx(ctx, "Failed to get namespaced functions",
				"namespace", namespace,
				"err", err)
			continue
		}

		// check each function container healthiness and update function's status correspondingly
		for _, function := range functions {
			functionConfig := function.GetConfig()
			functionStatus := function.GetStatus()
			functionName := functionConfig.Meta.Name

			functionIsReady := functionStatus.State == functionconfig.FunctionStateReady
			functionWasSetAsUnhealthy := functionconfig.FunctionStateInSlice(functionStatus.State,
				[]functionconfig.FunctionState{
					functionconfig.FunctionStateError,
					functionconfig.FunctionStateUnhealthy,
				}) && functionStatus.Message == string(common.FunctionStateMessageUnhealthy)

			if !(functionIsReady || functionWasSetAsUnhealthy) {

				// cannot be monitored
				continue
			}

			// get function container name
			containerName := p.GetFunctionContainerName(&functionconfig.Config{
				Meta: functionconfig.Meta{
					Name:      functionName,
					Namespace: namespace,
				},
			})

			// get function container by name
			containers, err := p.dockerClient.GetContainers(&dockerclient.GetContainerOptions{
				Name: containerName,
			})
			if err != nil {
				p.Logger.WarnWithCtx(ctx, "Failed to get containers by name",
					"err", err,
					"containerName", containerName)
				continue
			}

			// if function container does not exists, mark as unhealthy
			if len(containers) == 0 {
				p.Logger.WarnWithCtx(ctx, "No containers were found", "functionName", functionName)

				// no running containers were found for function, set function unhealthy
				if err := p.setFunctionUnhealthy(function); err != nil {
					p.Logger.ErrorWithCtx(ctx, "Failed to mark a function as unhealthy",
						"err", err,
						"functionName", functionName,
						"namespace", namespace)
				}
				continue
			}

			container := containers[0]

			// check ready function to ensure its container is healthy
			if functionIsReady {
				if err := p.checkAndSetFunctionUnhealthy(container.ID, function); err != nil {
					p.Logger.ErrorWithCtx(ctx, "Failed to check a function's health and mark it as unhealthy if necessary",
						"err", err,
						"functionName", functionName,
						"namespace", namespace)
				}
			}

			// check unhealthy function to see if its container id is healthy again
			if functionWasSetAsUnhealthy {
				if err := p.checkAndSetFunctionHealthy(container.ID, function); err != nil {
					p.Logger.ErrorWithCtx(ctx, "Failed to check a function's health and mark it as unhealthy if necessary",
						"err", err,
						"functionName", functionName,
						"namespace", namespace)
				}
			}
		}
	}
}

func (p *Platform) GetFunctionContainerName(functionConfig *functionconfig.Config) string {
	return fmt.Sprintf("nuclio-%s-%s",
		functionConfig.Meta.Namespace,
		functionConfig.Meta.Name)
}

func (p *Platform) GetFunctionVolumeMountName(functionConfig *functionconfig.Config) string {
	return fmt.Sprintf("nuclio-%s-%s",
		functionConfig.Meta.Namespace,
		functionConfig.Meta.Name)
}

func (p *Platform) deployFunction(createFunctionOptions *platform.CreateFunctionOptions,
	previousHTTPPort int) (*platform.CreateFunctionResult, error) {

	// get function platform-specific configuration
	functionPlatformConfiguration, err := newFunctionPlatformConfiguration(&createFunctionOptions.FunctionConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create a function's platform configuration")
	}

	functionMountMode, err := p.resolveFunctionMountMode(functionPlatformConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to resolve processor mount mode")
	}

	mountPoints, volumesMap, err := p.resolveAndCreateFunctionMounts(createFunctionOptions, functionMountMode)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to resolve and create function mounts")
	}

	labels := p.compileDeployFunctionLabels(createFunctionOptions)
	envMap := p.compileDeployFunctionEnvMap(createFunctionOptions)

	// get function port - either from configuration, from the previous deployment or from a free port
	functionExternalHTTPPort, err := p.getFunctionHTTPPort(createFunctionOptions, previousHTTPPort)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get a function's HTTP port")
	}

	gpus := ""
	if createFunctionOptions.FunctionConfig.Spec.PositiveGPUResourceLimit() {

		// TODO: allow user specify the exact gpu index/uuid & capabilities
		// https://docs.docker.com/config/containers/resource_constraints/#access-an-nvidia-gpu
		gpus = "all"
	}

	functionSecurityContext := createFunctionOptions.FunctionConfig.Spec.SecurityContext

	// run the docker image
	runContainerOptions := &dockerclient.RunOptions{
		ContainerName: p.GetFunctionContainerName(&createFunctionOptions.FunctionConfig),
		Ports: map[int]int{
			functionExternalHTTPPort: abstract.FunctionContainerHTTPPort,
		},
		Env:           envMap,
		Labels:        labels,
		Volumes:       volumesMap,
		Network:       functionPlatformConfiguration.Network,
		RestartPolicy: functionPlatformConfiguration.RestartPolicy,
		GPUs:          gpus,
		MountPoints:   mountPoints,
		RunAsUser:     functionSecurityContext.RunAsUser,
		RunAsGroup:    functionSecurityContext.RunAsGroup,
		FSGroup:       functionSecurityContext.FSGroup,
	}

	containerID, err := p.dockerClient.RunContainer(createFunctionOptions.FunctionConfig.Spec.Image,
		runContainerOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to run a Docker container")
	}

	timeout := createFunctionOptions.FunctionConfig.Spec.ReadinessTimeoutSeconds
	if err := p.waitForContainer(containerID, timeout); err != nil {
		return nil, err
	}

	functionExternalHTTPPort, err = p.resolveDeployedFunctionHTTPPort(containerID)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to resolve a deployed function's HTTP port")
	}

	return &platform.CreateFunctionResult{
		CreateFunctionBuildResult: platform.CreateFunctionBuildResult{
			Image:                 createFunctionOptions.FunctionConfig.Spec.Image,
			UpdatedFunctionConfig: createFunctionOptions.FunctionConfig,
		},
		Port:        functionExternalHTTPPort,
		ContainerID: containerID,
	}, nil
}

func (p *Platform) delete(ctx context.Context, deleteFunctionOptions *platform.DeleteFunctionOptions) error {

	// delete the function from the local store
	if err := p.localStore.DeleteFunction(ctx, &deleteFunctionOptions.FunctionConfig.Meta); err != nil &&
		err != nuclio.ErrNotFound {
		p.Logger.WarnWithCtx(ctx, "Failed to delete a function from the local store", "err", err.Error())
	}

	getContainerOptions := &dockerclient.GetContainerOptions{
		Stopped: true,
		Labels: map[string]string{
			"nuclio.io/platform":                      "local",
			"nuclio.io/namespace":                     deleteFunctionOptions.FunctionConfig.Meta.Namespace,
			common.NuclioResourceLabelKeyFunctionName: deleteFunctionOptions.FunctionConfig.Meta.Name,
		},
	}

	containersInfo, err := p.dockerClient.GetContainers(getContainerOptions)
	if err != nil {
		return errors.Wrap(err, "Failed to get containers")
	}

	p.Logger.DebugWithCtx(ctx, "Got function containers", "containersInfoLength", len(containersInfo))

	// iterate over contains and delete them. It's possible that under some weird circumstances
	// there are a few instances of this function in the namespace
	for _, containerInfo := range containersInfo {
		p.Logger.DebugWithCtx(ctx, "Removing function container", "containerName", containerInfo.Name)
		if err := p.dockerClient.RemoveContainer(containerInfo.ID); err != nil {
			return errors.Wrapf(err, "Failed to remove container %s", containerInfo.ID)
		}
	}

	// delete function volume mount after containers are deleted
	functionVolumeMountName := p.GetFunctionVolumeMountName(&deleteFunctionOptions.FunctionConfig)
	p.Logger.DebugWithCtx(ctx, "Removing function volume", "functionVolumeMountName", functionVolumeMountName)
	if err := p.dockerClient.DeleteVolume(functionVolumeMountName); err != nil {
		return errors.Wrapf(err, "Failed to delete a function volume %s", functionVolumeMountName)
	}

	p.Logger.InfoWithCtx(ctx, "Successfully deleted function",
		"name", deleteFunctionOptions.FunctionConfig.Meta.Name)
	return nil
}

func (p *Platform) resolveFunctionMountMode(functionPlatformConfiguration *functionPlatformConfiguration) (
	FunctionMountMode, error) {

	// if set, return value from function platform configuration
	if functionPlatformConfiguration.MountMode != "" {
		return functionPlatformConfiguration.MountMode, nil
	}

	// use platform defaults
	return p.defaultFunctionMountMode, nil
}

func (p *Platform) resolveAndCreateFunctionMounts(createFunctionOptions *platform.CreateFunctionOptions,
	functionMountMode FunctionMountMode) ([]dockerclient.MountPoint, map[string]string, error) {

	var mountPoints []dockerclient.MountPoint
	volumesMap := p.compileDeployFunctionVolumesMap(createFunctionOptions)

	switch functionMountMode {
	case FunctionMountModeVolume:
		if err := p.prepareFunctionVolumeMount(createFunctionOptions); err != nil {
			return nil, nil, errors.Wrap(err, "Failed to prepare a function's volume mount")
		}
		mountPoints = append(mountPoints, dockerclient.MountPoint{
			Source:      p.GetFunctionVolumeMountName(&createFunctionOptions.FunctionConfig),
			Destination: FunctionProcessorContainerDirPath,

			// read only mode
			RW: false,
		})
	default:

		// create processor configuration at a temporary location unless user specified a configuration
		localProcessorConfigPath, err := p.createProcessorConfig(createFunctionOptions)
		if err != nil {
			return nil, nil, errors.Wrap(err, "Failed to create a processor configuration")
		}

		// volumize it
		volumesMap[localProcessorConfigPath] = path.Join(FunctionProcessorContainerDirPath, "processor.yaml")
	}

	return mountPoints, volumesMap, nil
}

func (p *Platform) createProcessorConfig(createFunctionOptions *platform.CreateFunctionOptions) (string, error) {

	configWriter, err := processorconfig.NewWriter()
	if err != nil {
		return "", errors.Wrap(err, "Failed to create a processor configuration writer")
	}

	// must specify "/tmp" here so that it's available on docker for mac
	processorConfigFile, err := ioutil.TempFile("/tmp", "processor-config-")
	if err != nil {
		return "", errors.Wrap(err, "Failed to create a temporary processor configuration")
	}

	defer processorConfigFile.Close() // nolint: errcheck

	if err = configWriter.Write(processorConfigFile, &processor.Configuration{
		Config: createFunctionOptions.FunctionConfig,
	}); err != nil {
		return "", errors.Wrap(err, "Failed to write a processor configuration")
	}

	// make it readable by other users, in case user use different USER directive on function image
	if err := os.Chmod(processorConfigFile.Name(), 0644); err != nil {
		return "", errors.Wrap(err, "Failed to change a processor's configuration-file permission")
	}

	p.Logger.DebugWith("Wrote processor configuration", "path", processorConfigFile.Name())

	// read the file once for logging
	processorConfigContents, err := ioutil.ReadFile(processorConfigFile.Name())
	if err != nil {
		return "", errors.Wrap(err, "Failed to read a processor-configuration file")
	}

	// log
	p.Logger.DebugWith("Wrote processor configuration file", "contents", string(processorConfigContents))

	return processorConfigFile.Name(), nil
}

func (p *Platform) encodeFunctionSpec(spec *functionconfig.Spec) string {
	encodedFunctionSpec, _ := json.Marshal(spec)

	return string(encodedFunctionSpec)
}

func (p *Platform) getFunctionHTTPPort(createFunctionOptions *platform.CreateFunctionOptions,
	previousHTTPPort int) (int, error) {

	// if the configuration specified an HTTP port - use that
	if createFunctionOptions.FunctionConfig.Spec.GetHTTPPort() != 0 {
		p.Logger.DebugWith("Configuration specified HTTP port",
			"port",
			createFunctionOptions.FunctionConfig.Spec.GetHTTPPort())

		return createFunctionOptions.FunctionConfig.Spec.GetHTTPPort(), nil
	}

	// if there was a previous deployment and no configuration - use that
	if previousHTTPPort != 0 {
		createFunctionOptions.Logger.DebugWith("Using previous deployment HTTP port ",
			"previousHTTPPort", previousHTTPPort)
		return previousHTTPPort, nil
	}

	return dockerclient.RunOptionsNoPort, nil
}

func (p *Platform) resolveDeployedFunctionHTTPPort(containerID string) (int, error) {
	containers, err := p.dockerClient.GetContainers(&dockerclient.GetContainerOptions{
		ID: containerID,
	})
	if err != nil || len(containers) == 0 {
		return 0, errors.Wrap(err, "Failed to get a container")
	}
	return p.getContainerHTTPTriggerPort(&containers[0])
}

func (p *Platform) getContainerHTTPTriggerPort(container *dockerclient.Container) (int, error) {
	return p.dockerClient.GetContainerPort(container, abstract.FunctionContainerHTTPPort)
}

func (p *Platform) marshallAnnotations(annotations map[string]string) []byte {
	if annotations == nil {
		return nil
	}

	marshalledAnnotations, err := json.Marshal(annotations)
	if err != nil {
		return nil
	}

	// convert to string and return address
	return marshalledAnnotations
}

func (p *Platform) deletePreviousContainers(createFunctionOptions *platform.CreateFunctionOptions) (int, error) {
	var previousHTTPPort int

	createFunctionOptions.Logger.InfoWith("Cleaning up before deployment",
		"functionName", createFunctionOptions.FunctionConfig.Meta.Name)

	// get function containers
	containers, err := p.dockerClient.GetContainers(&dockerclient.GetContainerOptions{
		Name:    p.GetFunctionContainerName(&createFunctionOptions.FunctionConfig),
		Stopped: true,
	})
	if err != nil {
		return 0, errors.Wrap(err, "Failed to get function containers")
	}

	// if the function exists, delete it
	if len(containers) > 0 {
		createFunctionOptions.Logger.InfoWith("Function already exists, deleting function containers",
			"functionName", createFunctionOptions.FunctionConfig.Meta.Name)

		// iterate over containers and delete
		for _, container := range containers {
			createFunctionOptions.Logger.DebugWith("Deleting function container",
				"functionName", createFunctionOptions.FunctionConfig.Meta.Name,
				"containerName", container.Name)
			previousHTTPPort, err = p.getContainerHTTPTriggerPort(&container)
			if err != nil {
				return 0, errors.Wrap(err, "Failed to get a container's HTTP-trigger port")
			}

			if err := p.dockerClient.RemoveContainer(container.ID); err != nil {
				return 0, errors.Wrap(err, "Failed to delete a function container")
			}
		}
	}

	return previousHTTPPort, nil
}

func (p *Platform) checkAndSetFunctionUnhealthy(containerID string, function platform.Function) error {
	if err := p.dockerClient.AwaitContainerHealth(containerID,
		&p.Config.Local.FunctionContainersHealthinessTimeout); err != nil {
		return p.setFunctionUnhealthy(function)
	}
	return nil
}

func (p *Platform) setFunctionUnhealthy(function platform.Function) error {
	functionStatus := function.GetStatus()

	// set function state to error
	functionStatus.State = functionconfig.FunctionStateUnhealthy

	// set unhealthy error message
	functionStatus.Message = string(common.FunctionStateMessageUnhealthy)

	p.Logger.WarnWith("Setting function state as unhealthy",
		"functionName", function.GetConfig().Meta.Name,
		"functionStatus", functionStatus)

	// function container is not healthy or missing, set function state as error
	return p.localStore.CreateOrUpdateFunction(&functionconfig.ConfigWithStatus{
		Config: *function.GetConfig(),
		Status: *functionStatus,
	})
}

func (p *Platform) checkAndSetFunctionHealthy(containerID string, function platform.Function) error {
	if err := p.dockerClient.AwaitContainerHealth(containerID,
		&p.Config.Local.FunctionContainersHealthinessTimeout); err != nil {
		return errors.Wrapf(err, "Failed to ensure the health of container ID %s", containerID)
	}
	functionStatus := function.GetStatus()

	// set function as ready
	functionStatus.State = functionconfig.FunctionStateReady

	// unset error message
	functionStatus.Message = ""

	p.Logger.InfoWith("Setting function state as ready",
		"functionName", function.GetConfig().Meta.Name,
		"functionStatus", functionStatus)

	// function container is not healthy or missing, set function state as error
	return p.localStore.CreateOrUpdateFunction(&functionconfig.ConfigWithStatus{
		Config: *function.GetConfig(),
		Status: *functionStatus,
	})
}

func (p *Platform) waitForContainer(containerID string, timeout int) error {
	p.Logger.InfoWith("Waiting for function to be ready",
		"timeout", timeout)

	readinessTimeout := time.Duration(timeout) * time.Second
	if timeout == 0 {
		readinessTimeout = p.Config.GetDefaultFunctionReadinessTimeout()
	}

	if err := p.dockerClient.AwaitContainerHealth(containerID, &readinessTimeout); err != nil {
		var errMessage string

		// try to get error logs
		containerLogs, getContainerLogsErr := p.dockerClient.GetContainerLogs(containerID)
		if getContainerLogsErr == nil {
			errMessage = fmt.Sprintf("Function wasn't ready in time. Logs:\n%s", containerLogs)
		} else {
			errMessage = fmt.Sprintf("Function wasn't ready in time (couldn't fetch logs: %s)", getContainerLogsErr.Error())
		}

		return errors.Wrap(err, errMessage)
	}
	return nil
}

func (p *Platform) compileDeployFunctionVolumesMap(createFunctionOptions *platform.CreateFunctionOptions) map[string]string {
	volumesMap := map[string]string{}
	for _, volume := range createFunctionOptions.FunctionConfig.Spec.Volumes {

		// only add hostpath volumes
		if volume.Volume.HostPath != nil {
			volumesMap[volume.Volume.HostPath.Path] = volume.VolumeMount.MountPath
		}
	}
	return volumesMap
}

func (p *Platform) prepareFunctionVolumeMount(createFunctionOptions *platform.CreateFunctionOptions) error {

	// create docker volume
	if err := p.dockerClient.CreateVolume(&dockerclient.CreateVolumeOptions{
		Name: p.GetFunctionVolumeMountName(&createFunctionOptions.FunctionConfig),
	}); err != nil {
		return errors.Wrapf(err, "Failed to create a volume for function %s",
			createFunctionOptions.FunctionConfig.Meta.Name)
	}

	// marshaling processor config
	processorConfigBody, err := yaml.Marshal(&processor.Configuration{
		Config: createFunctionOptions.FunctionConfig,
	})
	if err != nil {
		return errors.Wrap(err, "Failed to marshal a processor configuration")
	}

	// dumping contents to volume's processor path
	if _, err := p.dockerClient.RunContainer("gcr.io/iguazio/alpine:3.15", &dockerclient.RunOptions{
		Remove: true,
		MountPoints: []dockerclient.MountPoint{
			{
				Source:      p.GetFunctionVolumeMountName(&createFunctionOptions.FunctionConfig),
				Destination: FunctionProcessorContainerDirPath,
				RW:          true,
			},
		},
		Command: fmt.Sprintf(`sh -c 'echo "%s" | base64 -d | install -m 777 /dev/stdin %s'`,
			base64.StdEncoding.EncodeToString(processorConfigBody),
			path.Join(FunctionProcessorContainerDirPath, "processor.yaml")),
	}); err != nil {
		return errors.Wrap(err, "Failed to write a processor configuration to a volume")
	}
	return nil
}

func (p *Platform) compileDeployFunctionEnvMap(createFunctionOptions *platform.CreateFunctionOptions) map[string]string {
	envMap := map[string]string{}
	for _, env := range createFunctionOptions.FunctionConfig.Spec.Env {
		envMap[env.Name] = env.Value
	}
	return envMap
}

func (p *Platform) compileDeployFunctionLabels(createFunctionOptions *platform.CreateFunctionOptions) map[string]string {
	labels := map[string]string{
		"nuclio.io/platform":                      "local",
		"nuclio.io/namespace":                     createFunctionOptions.FunctionConfig.Meta.Namespace,
		common.NuclioResourceLabelKeyFunctionName: createFunctionOptions.FunctionConfig.Meta.Name,
		"nuclio.io/function-spec":                 p.encodeFunctionSpec(&createFunctionOptions.FunctionConfig.Spec),
	}

	for labelName, labelValue := range createFunctionOptions.FunctionConfig.Meta.Labels {
		labels[labelName] = labelValue
	}

	marshalledAnnotations := p.marshallAnnotations(createFunctionOptions.FunctionConfig.Meta.Annotations)
	if marshalledAnnotations != nil {
		labels["nuclio.io/annotations"] = string(marshalledAnnotations)
	}
	return labels
}

func (p *Platform) enrichAndValidateFunctionConfig(ctx context.Context, functionConfig *functionconfig.Config) error {
	if err := p.EnrichFunctionConfig(ctx, functionConfig); err != nil {
		return errors.Wrap(err, "Failed to enrich a function configuration")
	}

	if err := p.ValidateFunctionConfig(ctx, functionConfig); err != nil {
		return errors.Wrap(err, "Failed to validate a function configuration")
	}

	return nil
}

func (p *Platform) populateFunctionInvocationStatus(functionInvocation *functionconfig.Status,
	createFunctionResults *platform.CreateFunctionResult) error {

	externalIPAddresses, err := p.GetExternalIPAddresses()
	if err != nil {
		return err
	}

	addresses, err := p.dockerClient.GetContainerIPAddresses(createFunctionResults.ContainerID)
	if err != nil {
		return errors.Wrap(err, "Failed to get container network addresses")
	}

	// enrich address with function's container port
	var addressesWithFunctionPort []string
	for _, address := range addresses {
		addressesWithFunctionPort = append(addressesWithFunctionPort,
			fmt.Sprintf("%s:%d", address, abstract.FunctionContainerHTTPPort))
	}

	functionInvocation.InternalInvocationURLs = addressesWithFunctionPort
	functionInvocation.ExternalInvocationURLs = []string{}
	for _, externalIPAddress := range externalIPAddresses {
		if externalIPAddress != "" {
			functionInvocation.ExternalInvocationURLs = append(functionInvocation.ExternalInvocationURLs,
				fmt.Sprintf("%s:%d", externalIPAddress, createFunctionResults.Port))
		}
	}
	return nil
}
