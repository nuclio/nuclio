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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/config"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/nuclio/zap"
	"golang.org/x/sync/errgroup"
)

type Platform struct {
	*abstract.Platform
	cmdRunner                             cmdrunner.CmdRunner
	dockerClient                          dockerclient.Client
	localStore                            *store
	checkFunctionContainersHealthiness    bool
	functionContainersHealthinessTimeout  time.Duration
	functionContainersHealthinessInterval time.Duration
}

const Mib = 1048576
const UnhealthyContainerErrorMessage = "Container is not healthy (detected by nuclio platform)"

// NewPlatform instantiates a new local platform
func NewPlatform(parentLogger logger.Logger,
	containerBuilderConfiguration *containerimagebuilderpusher.ContainerBuilderConfiguration,
	platformConfiguration interface{}) (*Platform, error) {
	newPlatform := &Platform{}

	// create base
	newAbstractPlatform, err := abstract.NewPlatform(parentLogger, newPlatform, platformConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract platform")
	}

	// init platform
	newPlatform.Platform = newAbstractPlatform

	// function containers healthiness check is disabled by default
	newPlatform.checkFunctionContainersHealthiness = common.GetEnvOrDefaultBool("NUCLIO_CHECK_FUNCTION_CONTAINERS_HEALTHINESS", false)
	newPlatform.functionContainersHealthinessTimeout = time.Second * 5
	newPlatform.functionContainersHealthinessInterval = time.Second * 30

	// create a command runner
	if newPlatform.cmdRunner, err = cmdrunner.NewShellRunner(newPlatform.Logger); err != nil {
		return nil, errors.Wrap(err, "Failed to create command runner")
	}

	if newPlatform.ContainerBuilder, err = containerimagebuilderpusher.NewDocker(newPlatform.Logger,
		containerBuilderConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to create containerimagebuilderpusher")
	}

	// create a docker client
	if newPlatform.dockerClient, err = dockerclient.NewShellClient(newPlatform.Logger, nil); err != nil {
		return nil, errors.Wrap(err, "Failed to create docker client")
	}

	// create a local store for configs and stuff
	if newPlatform.localStore, err = newStore(parentLogger, newPlatform, newPlatform.dockerClient); err != nil {
		return nil, errors.Wrap(err, "Failed to create local store")
	}

	// ignite goroutine to check function container healthiness
	if newPlatform.checkFunctionContainersHealthiness {
		newPlatform.Logger.DebugWith("Igniting container healthiness validator")
		go func(newPlatform *Platform) {
			uptimeTicker := time.NewTicker(newPlatform.functionContainersHealthinessInterval)
			for range uptimeTicker.C {
				newPlatform.ValidateFunctionContainersHealthiness()
			}
		}(newPlatform)
	}
	return newPlatform, nil
}

// CreateFunction will simply run a docker image
func (p *Platform) CreateFunction(createFunctionOptions *platform.CreateFunctionOptions) (*platform.CreateFunctionResult, error) {
	var previousHTTPPort int
	var err error
	var existingFunctionConfig *functionconfig.ConfigWithStatus

	// wrap logger
	logStream, err := abstract.NewLogStream("deployer", nucliozap.InfoLevel, createFunctionOptions.Logger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create log stream")
	}

	// save the log stream for the name
	p.DeployLogStreams.Store(createFunctionOptions.FunctionConfig.Meta.GetUniqueID(), logStream)

	// replace logger
	createFunctionOptions.Logger = logStream.GetLogger()

	if err := p.EnrichCreateFunctionOptions(createFunctionOptions); err != nil {
		return nil, errors.Wrap(err, "Create function options enrichment failed")
	}

	if err := p.ValidateCreateFunctionOptions(createFunctionOptions); err != nil {
		return nil, errors.Wrap(err, "Create function options validation failed")
	}

	// local currently doesn't support registries of any kind. remove push / run registry
	createFunctionOptions.FunctionConfig.Spec.RunRegistry = ""
	createFunctionOptions.FunctionConfig.Spec.Build.Registry = ""

	// it's possible to pass a function without specifying any meta in the request, in that case skip getting existing function
	if createFunctionOptions.FunctionConfig.Meta.Namespace != "" && createFunctionOptions.FunctionConfig.Meta.Name != "" {
		existingFunctions, err := p.localStore.getFunctions(&createFunctionOptions.FunctionConfig.Meta)
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
	if err := p.ValidateCreateFunctionOptionsAgainstExistingFunctionConfig(existingFunctionConfig,
		createFunctionOptions); err != nil {
		return nil, errors.Wrap(err, "Validation against existing function config failed")
	}

	reportCreationError := func(creationError error) error {
		createFunctionOptions.Logger.WarnWith("Create function failed, setting function status",
			"err", creationError)

		errorStack := bytes.Buffer{}
		errors.PrintErrorStack(&errorStack, creationError, 20)

		// cut messages that are too big
		if errorStack.Len() >= 4*Mib {
			errorStack.Truncate(4 * Mib)
		}

		// post logs and error
		return p.localStore.createOrUpdateFunction(&functionconfig.ConfigWithStatus{
			Config: createFunctionOptions.FunctionConfig,
			Status: functionconfig.Status{
				State:   functionconfig.FunctionStateError,
				Message: errorStack.String(),
			},
		})
	}

	onAfterConfigUpdated := func(updatedFunctionConfig *functionconfig.Config) error {
		createFunctionOptions.Logger.DebugWith("Creating shadow function",
			"name", createFunctionOptions.FunctionConfig.Meta.Name)

		// create the function in the store
		if err = p.localStore.createOrUpdateFunction(&functionconfig.ConfigWithStatus{
			Config: createFunctionOptions.FunctionConfig,
			Status: functionconfig.Status{
				State: functionconfig.FunctionStateBuilding,
			},
		}); err != nil {
			return errors.Wrap(err, "Failed to create function")
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
		functionStatus := functionconfig.Status{
			State: functionconfig.FunctionStateImported,
		}

		if !skipFunctionDeploy {
			createFunctionResult, deployErr = p.deployFunction(createFunctionOptions, previousHTTPPort)
			if deployErr != nil {
				reportCreationError(deployErr) // nolint: errcheck
				return nil, deployErr
			}

			functionStatus = functionconfig.Status{
				HTTPPort: createFunctionResult.Port,
				State:    functionconfig.FunctionStateReady,
			}
		} else {
			p.Logger.Info("Skipping function deployment")
			createFunctionResult = &platform.CreateFunctionResult{
				CreateFunctionBuildResult: platform.CreateFunctionBuildResult{
					Image:                 createFunctionOptions.FunctionConfig.Spec.Image,
					UpdatedFunctionConfig: createFunctionOptions.FunctionConfig,
				},
			}
		}

		// update the function
		if err = p.localStore.createOrUpdateFunction(&functionconfig.ConfigWithStatus{
			Config: createFunctionOptions.FunctionConfig,
			Status: functionStatus,
		}); err != nil {
			return nil, errors.Wrap(err, "Failed to update function with state")
		}

		return createFunctionResult, nil
	}

	// If needed, load any docker image from archive into docker
	if createFunctionOptions.InputImageFile != "" {
		p.Logger.InfoWith("Loading docker image from archive", "input", createFunctionOptions.InputImageFile)
		err := p.dockerClient.Load(createFunctionOptions.InputImageFile)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to load docker image from archive")
		}
	}

	// wrap the deployer's deploy with the base HandleDeployFunction to provide lots of
	// common functionality
	return p.HandleDeployFunction(existingFunctionConfig, createFunctionOptions, onAfterConfigUpdated, onAfterBuild)
}

// GetFunctions will return deployed functions
func (p *Platform) GetFunctions(getFunctionsOptions *platform.GetFunctionsOptions) ([]platform.Function, error) {
	var functions []platform.Function

	// get project filter
	projectName := common.StringToStringMap(getFunctionsOptions.Labels, "=")["nuclio.io/project-name"]

	// get all the functions in the store. these functions represent both functions that are deployed
	// and functions that failed to build
	localStoreFunctions, err := p.localStore.getFunctions(&functionconfig.Meta{
		Name:      getFunctionsOptions.Name,
		Namespace: getFunctionsOptions.Namespace,
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to read functions from local store")
	}

	// filter by project name
	for _, localStoreFunction := range localStoreFunctions {
		if projectName != "" && localStoreFunction.GetConfig().Meta.Labels["nuclio.io/project-name"] != projectName {
			continue
		}
		functions = append(functions, localStoreFunction)
	}

	// enrich with build logs
	p.EnrichFunctionsWithDeployLogStream(functions)

	return functions, nil
}

// UpdateFunction will update a previously deployed function
func (p *Platform) UpdateFunction(updateFunctionOptions *platform.UpdateFunctionOptions) error {
	return nil
}

// DeleteFunction will delete a previously deployed function
func (p *Platform) DeleteFunction(deleteFunctionOptions *platform.DeleteFunctionOptions) error {

	// delete the function from the local store
	err := p.localStore.deleteFunction(&deleteFunctionOptions.FunctionConfig.Meta)
	if err != nil && err != nuclio.ErrNotFound {
		p.Logger.WarnWith("Failed to delete function from local store", "err", err.Error())
	}

	getFunctionEventsOptions := &platform.FunctionEventMeta{
		Labels: map[string]string{
			"nuclio.io/function-name": deleteFunctionOptions.FunctionConfig.Meta.Name,
		},
		Namespace: deleteFunctionOptions.FunctionConfig.Meta.Namespace,
	}
	functionEvents, err := p.localStore.getFunctionEvents(getFunctionEventsOptions)
	if err != nil {
		return errors.Wrap(err, "Failed to get function events")
	}

	p.Logger.InfoWith("Got function events", "num", len(functionEvents))

	errGroup, _ := errgroup.WithContext(context.TODO())
	for _, functionEvent := range functionEvents {

		errGroup.Go(func() error {
			err = p.localStore.deleteFunctionEvent(&functionEvent.GetConfig().Meta)
			if err != nil {
				return errors.Wrap(err, "Failed to delete function event")
			}
			return nil
		})
	}

	// wait for all errgroup goroutines
	if err := errGroup.Wait(); err != nil {
		return errors.Wrap(err, "Failed to delete function events")
	}

	getContainerOptions := &dockerclient.GetContainerOptions{
		Labels: map[string]string{
			"nuclio.io/platform":      "local",
			"nuclio.io/namespace":     deleteFunctionOptions.FunctionConfig.Meta.Namespace,
			"nuclio.io/function-name": deleteFunctionOptions.FunctionConfig.Meta.Name,
		},
	}

	containersInfo, err := p.dockerClient.GetContainers(getContainerOptions)
	if err != nil {
		return errors.Wrap(err, "Failed to get containers")
	}

	// iterate over contains and delete them. It's possible that under some weird circumstances
	// there are a few instances of this function in the namespace
	for _, containerInfo := range containersInfo {
		if err := p.dockerClient.RemoveContainer(containerInfo.ID); err != nil {
			return err
		}
	}

	p.Logger.InfoWith("Function deleted", "name", deleteFunctionOptions.FunctionConfig.Meta.Name)

	return nil
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
func (p *Platform) CreateProject(createProjectOptions *platform.CreateProjectOptions) error {
	return p.localStore.createOrUpdateProject(&createProjectOptions.ProjectConfig)
}

// UpdateProject will update an existing project
func (p *Platform) UpdateProject(updateProjectOptions *platform.UpdateProjectOptions) error {
	return p.localStore.createOrUpdateProject(&updateProjectOptions.ProjectConfig)
}

// DeleteProject will delete an existing project
func (p *Platform) DeleteProject(deleteProjectOptions *platform.DeleteProjectOptions) error {
	if err := p.Platform.ValidateDeleteProjectOptions(deleteProjectOptions); err != nil {
		return err
	}
	return p.localStore.deleteProject(&deleteProjectOptions.Meta)
}

// GetProjects will list existing projects
func (p *Platform) GetProjects(getProjectsOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	return p.localStore.getProjects(&getProjectsOptions.Meta)
}

// CreateFunctionEvent will create a new function event that can later be used as a template from
// which to invoke functions
func (p *Platform) CreateFunctionEvent(createFunctionEventOptions *platform.CreateFunctionEventOptions) error {
	return p.localStore.createOrUpdateFunctionEvent(&createFunctionEventOptions.FunctionEventConfig)
}

// UpdateFunctionEvent will update a previously existing function event
func (p *Platform) UpdateFunctionEvent(updateFunctionEventOptions *platform.UpdateFunctionEventOptions) error {
	return p.localStore.createOrUpdateFunctionEvent(&updateFunctionEventOptions.FunctionEventConfig)
}

// DeleteFunctionEvent will delete a previously existing function event
func (p *Platform) DeleteFunctionEvent(deleteFunctionEventOptions *platform.DeleteFunctionEventOptions) error {
	return p.localStore.deleteFunctionEvent(&deleteFunctionEventOptions.Meta)
}

// GetFunctionEvents will list existing function events
func (p *Platform) GetFunctionEvents(getFunctionEventsOptions *platform.GetFunctionEventsOptions) ([]platform.FunctionEvent, error) {
	return p.localStore.getFunctionEvents(&getFunctionEventsOptions.Meta)
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
func (p *Platform) GetNamespaces() ([]string, error) {
	return []string{"nuclio"}, nil
}

func (p *Platform) GetDefaultInvokeIPAddresses() ([]string, error) {
	return []string{"172.17.0.1"}, nil
}

func (p *Platform) SaveFunctionDeployLogs(functionName, namespace string) error {
	functions, err := p.GetFunctions(&platform.GetFunctionsOptions{
		Name:      functionName,
		Namespace: namespace,
	})
	if err != nil || len(functions) == 0 {
		return errors.Wrap(err, "Failed to get existing functions")
	}

	// enrich with build logs
	p.EnrichFunctionsWithDeployLogStream(functions)

	function := functions[0]

	return p.localStore.createOrUpdateFunction(&functionconfig.ConfigWithStatus{
		Config: *function.GetConfig(),
		Status: *function.GetStatus(),
	})
}

func (p *Platform) deployFunction(createFunctionOptions *platform.CreateFunctionOptions,
	previousHTTPPort int) (*platform.CreateFunctionResult, error) {

	// get function platform specific configuration
	functionPlatformConfiguration, err := newFunctionPlatformConfiguration(&createFunctionOptions.FunctionConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create function platform configuration")
	}

	volumesMap, err := p.compileDeployFunctionVolumesMap(createFunctionOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to compile volumes map")
	}
	labels := p.compileDeployFunctionLabels(createFunctionOptions)
	envMap := p.compileDeployFunctionEnvMap(createFunctionOptions)

	// get function port - either from configuration, from the previous deployment or from a free port
	functionExternalHTTPPort, err := p.getFunctionHTTPPort(createFunctionOptions, previousHTTPPort)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function HTTP port")
	}

	// run the docker image
	runContainerOptions := &dockerclient.RunOptions{
		ContainerName: p.GetContainerNameByCreateFunctionOptions(createFunctionOptions),
		Ports: map[int]int{
			functionExternalHTTPPort: abstract.FunctionContainerHTTPPort,
		},
		Env:           envMap,
		Labels:        labels,
		Volumes:       volumesMap,
		Network:       functionPlatformConfiguration.Network,
		RestartPolicy: functionPlatformConfiguration.RestartPolicy,
	}

	containerID, err := p.dockerClient.RunContainer(createFunctionOptions.FunctionConfig.Spec.Image,
		runContainerOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to run docker container")
	}

	timeout := createFunctionOptions.FunctionConfig.Spec.ReadinessTimeoutSeconds
	if err := p.waitForContainer(containerID, timeout); err != nil {
		return nil, err
	}

	functionExternalHTTPPort, err = p.resolveDeployedFunctionHTTPPort(containerID)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to resolve deployed function HTTP port")
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

func (p *Platform) createProcessorConfig(createFunctionOptions *platform.CreateFunctionOptions) (string, error) {

	configWriter, err := processorconfig.NewWriter()
	if err != nil {
		return "", errors.Wrap(err, "Failed to create processor configuration writer")
	}

	// must specify "/tmp" here so that it's available on docker for mac
	processorConfigFile, err := ioutil.TempFile("/tmp", "processor-config-")
	if err != nil {
		return "", errors.Wrap(err, "Failed to create temporary processor config")
	}

	defer processorConfigFile.Close() // nolint: errcheck

	if err = configWriter.Write(processorConfigFile, &processor.Configuration{
		Config: createFunctionOptions.FunctionConfig,
	}); err != nil {
		return "", errors.Wrap(err, "Failed to write processor config")
	}

	// make it readable by other users, in case user use different USER directive on function image
	if err := os.Chmod(processorConfigFile.Name(), 0644); err != nil {
		return "", errors.Wrap(err, "Failed to change processor config file permission")
	}

	p.Logger.DebugWith("Wrote processor configuration", "path", processorConfigFile.Name())

	// read the file once for logging
	processorConfigContents, err := ioutil.ReadFile(processorConfigFile.Name())
	if err != nil {
		return "", errors.Wrap(err, "Failed to read processor configuration file")
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

func (p *Platform) GetContainerNameByCreateFunctionOptions(createFunctionOptions *platform.CreateFunctionOptions) string {
	return fmt.Sprintf("nuclio-%s-%s",
		createFunctionOptions.FunctionConfig.Meta.Namespace,
		createFunctionOptions.FunctionConfig.Meta.Name)
}

func (p *Platform) resolveDeployedFunctionHTTPPort(containerID string) (int, error) {
	containers, err := p.dockerClient.GetContainers(&dockerclient.GetContainerOptions{
		ID: containerID,
	})
	if err != nil || len(containers) == 0 {
		return 0, errors.Wrap(err, "Failed to get container")
	}
	return p.getContainerHTTPTriggerPort(&containers[0])
}

func (p *Platform) getContainerHTTPTriggerPort(container *dockerclient.Container) (int, error) {
	functionHostPort := dockerclient.Port(fmt.Sprintf("%d/tcp", abstract.FunctionContainerHTTPPort))

	portBindings := container.HostConfig.PortBindings[functionHostPort]
	ports := container.NetworkSettings.Ports[functionHostPort]
	if len(portBindings) == 0 && len(ports) == 0 {
		return 0, nil
	}

	if len(portBindings) != 0 && portBindings[0].HostPort != "" {

		// by default take the port binding, as if the user requested
		return strconv.Atoi(portBindings[0].HostPort)
	} else if len(ports) != 0 && ports[0].HostPort != "" {

		// in case the user did not set an explicit port, take the random port assigned by docker
		return strconv.Atoi(ports[0].HostPort)
	} else {

		// something bad happened if we got here
		return 0, errors.New("No port was assigned")
	}
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

	createFunctionOptions.Logger.InfoWith("Cleaning up before deployment")

	getContainerOptions := &dockerclient.GetContainerOptions{
		Name:    p.GetContainerNameByCreateFunctionOptions(createFunctionOptions),
		Stopped: true,
	}

	containers, err := p.dockerClient.GetContainers(getContainerOptions)

	if err != nil {
		return 0, errors.Wrap(err, "Failed to get function")
	}

	// if the function exists, delete it
	if len(containers) > 0 {
		createFunctionOptions.Logger.InfoWith("Function already exists, deleting")

		// iterate over containers and delete
		for _, container := range containers {
			previousHTTPPort, err = p.getContainerHTTPTriggerPort(&container)
			if err != nil {
				return 0, errors.Wrap(err, "Failed to get container http trigger port")
			}

			err = p.dockerClient.RemoveContainer(container.Name)
			if err != nil {
				return 0, errors.Wrap(err, "Failed to delete existing function")
			}
		}
	}

	return previousHTTPPort, nil
}

func (p *Platform) ValidateFunctionContainersHealthiness() {
	namespaces, err := p.GetNamespaces()
	if err != nil {
		p.Logger.WarnWith("Cannot not get namespaces", "err", err)
		return
	}

	for _, namespace := range namespaces {

		// get functions for that namespace
		functions, err := p.GetFunctions(&platform.GetFunctionsOptions{
			Namespace: namespace,
		})
		if err != nil {
			p.Logger.WarnWith("Failed to get namespaced functions",
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
			functionWasSetAsUnhealthy := functionStatus.State == functionconfig.FunctionStateError &&
				strings.EqualFold(UnhealthyContainerErrorMessage, functionStatus.Message)

			if !(functionIsReady || functionWasSetAsUnhealthy) {

				// cannot be monitored
				continue
			}

			// get function container name
			containerName := p.GetContainerNameByCreateFunctionOptions(&platform.CreateFunctionOptions{
				FunctionConfig: functionconfig.Config{
					Meta: functionconfig.Meta{
						Name:      functionName,
						Namespace: namespace,
					},
				},
			})

			// get function container by name
			containers, err := p.dockerClient.GetContainers(&dockerclient.GetContainerOptions{
				Name: containerName,
			})
			if err != nil {
				p.Logger.WarnWith("Failed to get containers by name",
					"err", err,
					"containerName", containerName)
				continue
			}

			// if function container does not exists, mark as unhealthy
			if len(containers) == 0 {
				p.Logger.WarnWith("No containers were found", "functionName", functionName)

				// no running containers were found for function, set function unhealthy
				if err := p.setFunctionUnhealthy(function); err != nil {
					p.Logger.ErrorWith("Failed to set function unhealthy",
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
					p.Logger.ErrorWith("Failed to check and set function unhealthy",
						"err", err,
						"functionName", functionName,
						"namespace", namespace)
				}
			}

			// check unhealthy function to see if its container id is healthy again
			if functionWasSetAsUnhealthy {
				if err := p.checkAndSetFunctionHealthy(container.ID, function); err != nil {
					p.Logger.ErrorWith("Failed to check and set function healthy",
						"err", err,
						"functionName", functionName,
						"namespace", namespace)
				}
			}
		}
	}
}

func (p *Platform) checkAndSetFunctionUnhealthy(containerID string, function platform.Function) error {
	if err := p.dockerClient.AwaitContainerHealth(containerID,
		&p.functionContainersHealthinessTimeout); err != nil {
		return p.setFunctionUnhealthy(function)
	}
	return nil
}

func (p *Platform) setFunctionUnhealthy(function platform.Function) error {
	functionStatus := function.GetStatus()

	// set function state to error
	functionStatus.State = functionconfig.FunctionStateError

	// set unhealthy error message
	functionStatus.Message = UnhealthyContainerErrorMessage

	p.Logger.WarnWith("Setting function state as unhealthy",
		"functionName", function.GetConfig().Meta.Name,
		"functionStatus", functionStatus)

	// function container is not healthy or missing, set function state as error
	return p.localStore.createOrUpdateFunction(&functionconfig.ConfigWithStatus{
		Config: *function.GetConfig(),
		Status: *functionStatus,
	})
}

func (p *Platform) checkAndSetFunctionHealthy(containerID string, function platform.Function) error {
	if err := p.dockerClient.AwaitContainerHealth(containerID,
		&p.functionContainersHealthinessTimeout); err != nil {
		return errors.Wrapf(err, "Failed to ensure healthiness for container id %s", containerID)
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
	return p.localStore.createOrUpdateFunction(&functionconfig.ConfigWithStatus{
		Config: *function.GetConfig(),
		Status: *functionStatus,
	})
}

func (p *Platform) waitForContainer(containerID string, timeout int) error {
	p.Logger.InfoWith("Waiting for function to be ready",
		"timeout", timeout)

	var readinessTimeout time.Duration
	if timeout != 0 {
		readinessTimeout = time.Duration(timeout) * time.Second
	} else {
		readinessTimeout = abstract.DefaultReadinessTimeoutSeconds * time.Second
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

func (p *Platform) compileDeployFunctionVolumesMap(createFunctionOptions *platform.CreateFunctionOptions) (map[string]string, error) {

	// create processor configuration at a temporary location unless user specified a configuration
	localProcessorConfigPath, err := p.createProcessorConfig(createFunctionOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create processor configuration")
	}

	// create volumes string[string] map for volumes
	volumesMap := map[string]string{
		localProcessorConfigPath: path.Join("/", "etc", "nuclio", "config", "processor", "processor.yaml"),
	}

	for _, volume := range createFunctionOptions.FunctionConfig.Spec.Volumes {

		// only add hostpath volumes
		if volume.Volume.HostPath != nil {
			volumesMap[volume.Volume.HostPath.Path] = volume.VolumeMount.MountPath
		}
	}
	return volumesMap, nil
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
		"nuclio.io/platform":      "local",
		"nuclio.io/namespace":     createFunctionOptions.FunctionConfig.Meta.Namespace,
		"nuclio.io/function-name": createFunctionOptions.FunctionConfig.Meta.Name,
		"nuclio.io/function-spec": p.encodeFunctionSpec(&createFunctionOptions.FunctionConfig.Spec),
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
