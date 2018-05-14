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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"strconv"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/config"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
)

type Platform struct {
	*abstract.Platform
	cmdRunner    cmdrunner.CmdRunner
	dockerClient dockerclient.Client
	localStore   *store
}

// NewPlatform instantiates a new local platform
func NewPlatform(parentLogger logger.Logger) (*Platform, error) {
	newPlatform := &Platform{}

	// create base
	newAbstractPlatform, err := abstract.NewPlatform(parentLogger, newPlatform)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract platform")
	}

	// init platform
	newPlatform.Platform = newAbstractPlatform

	// create a command runner
	if newPlatform.cmdRunner, err = cmdrunner.NewShellRunner(newPlatform.Logger); err != nil {
		return nil, errors.Wrap(err, "Failed to create command runner")
	}

	// create a docker client
	if newPlatform.dockerClient, err = dockerclient.NewShellClient(newPlatform.Logger, nil); err != nil {
		return nil, errors.Wrap(err, "Failed to create docker client")
	}

	// create a local store for configs and stuff
	if newPlatform.localStore, err = newStore(parentLogger, newPlatform, newPlatform.dockerClient); err != nil {
		return nil, errors.Wrap(err, "Failed to create local store")
	}

	return newPlatform, nil
}

// CreateFunction will simply run a docker image
func (p *Platform) CreateFunction(createFunctionOptions *platform.CreateFunctionOptions) (*platform.CreateFunctionResult, error) {
	var previousHTTPPort int
	var err error

	// wrap logger
	logStream, err := abstract.NewLogStream("deployer", nucliozap.InfoLevel, createFunctionOptions.Logger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create log stream")
	}

	// save the log stream for the name
	p.DeployLogStreams[createFunctionOptions.FunctionConfig.Meta.GetUniqueID()] = logStream

	// replace logger
	createFunctionOptions.Logger = logStream.GetLogger()

	// local currently doesn't support registries of any kind. remove push / run registry
	createFunctionOptions.FunctionConfig.Spec.RunRegistry = ""
	createFunctionOptions.FunctionConfig.Spec.Build.Registry = ""

	reportCreationError := func(creationError error) error {
		createFunctionOptions.Logger.WarnWith("Create function failed, setting function status",
			"err", creationError)

		errorStack := bytes.Buffer{}
		errors.PrintErrorStack(&errorStack, creationError, 20)

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

		createFunctionResult, deployErr := p.deployFunction(createFunctionOptions, previousHTTPPort)
		if deployErr != nil {
			reportCreationError(deployErr) // nolint: errcheck
			return nil, deployErr
		}

		// update the function
		if err = p.localStore.createOrUpdateFunction(&functionconfig.ConfigWithStatus{
			Config: createFunctionOptions.FunctionConfig,
			Status: functionconfig.Status{
				HTTPPort: createFunctionResult.Port,
				State:    functionconfig.FunctionStateReady,
			},
		}); err != nil {
			return nil, errors.Wrap(err, "Failed to update function with state")
		}

		return createFunctionResult, nil
	}

	// wrap the deployer's deploy with the base HandleDeployFunction to provide lots of
	// common functionality
	return p.HandleDeployFunction(createFunctionOptions, onAfterConfigUpdated, onAfterBuild)
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

	// return a map of functions by name
	for _, localStoreFunction := range localStoreFunctions {

		// filter by project name
		if projectName != "" && localStoreFunction.GetConfig().Meta.Labels["nuclio.io/project-name"] != projectName {
			continue
		}

		// enrich with build logs
		if deployLogStream, exists := p.DeployLogStreams[localStoreFunction.GetConfig().Meta.GetUniqueID()]; exists {
			deployLogStream.ReadLogs(nil, &localStoreFunction.GetStatus().Logs)
		}

		functions = append(functions, localStoreFunction)
	}

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
	if err != nil {
		p.Logger.WarnWith("Failed to delete function from local store", "err", err.Error())
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

	if len(containersInfo) == 0 {
		return nil
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

// GetDeployRequiresRegistry returns true if a registry is required for deploy, false otherwise
func (p *Platform) GetDeployRequiresRegistry() bool {
	return false
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
	getFunctionsOptions := &platform.GetFunctionsOptions{
		Namespace: deleteProjectOptions.Meta.Namespace,
		Labels:    fmt.Sprintf("nuclio.io/project-name=%s", deleteProjectOptions.Meta.Name),
	}

	functions, err := p.GetFunctions(getFunctionsOptions)
	if err != nil {
		return errors.Wrap(err, "Failed to get functions")
	}

	if len(functions) != 0 {
		return fmt.Errorf("Project has %d functions, can't delete", len(functions))
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

func (p *Platform) getFreeLocalPort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}

	defer l.Close() // nolint: errcheck
	return l.Addr().(*net.TCPAddr).Port, nil
}

func (p *Platform) deployFunction(createFunctionOptions *platform.CreateFunctionOptions,
	previousHTTPPort int) (*platform.CreateFunctionResult, error) {

	// get function platform specific configuration
	functionPlatformConfiguration, err := newFunctionPlatformConfiguration(&createFunctionOptions.FunctionConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create function platform configuration")
	}

	// get function port - either from configuration, from the previous deployment or from a free port
	functionHTTPPort, err := p.getFunctionHTTPPort(createFunctionOptions, previousHTTPPort)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function HTTP port")
	}

	createFunctionOptions.Logger.DebugWith("Function port allocated",
		"port", functionHTTPPort,
		"previousHTTPPort", previousHTTPPort)

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
		volumesMap[volume.Volume.HostPath.Path] = volume.VolumeMount.MountPath
	}

	envMap := map[string]string{}
	for _, env := range createFunctionOptions.FunctionConfig.Spec.Env {
		envMap[env.Name] = env.Value
	}

	// run the docker image
	containerID, err := p.dockerClient.RunContainer(createFunctionOptions.FunctionConfig.Spec.Image, &dockerclient.RunOptions{
		ContainerName: p.getContainerNameByCreateFunctionOptions(createFunctionOptions),
		Ports:         map[int]int{functionHTTPPort: 8080},
		Env:           envMap,
		Labels:        labels,
		Volumes:       volumesMap,
		Network:       functionPlatformConfiguration.Network,
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to run docker container")
	}

	// TODO: you can't log a nil pointer without panicing - maybe this should be a logger-wide behavior
	var logReadinessTimeout interface{}
	if createFunctionOptions.ReadinessTimeout == nil {
		logReadinessTimeout = "nil"
	} else {
		logReadinessTimeout = createFunctionOptions.ReadinessTimeout
	}

	p.Logger.InfoWith("Waiting for function to be ready", "timeout", logReadinessTimeout)

	if err = p.dockerClient.AwaitContainerHealth(containerID, createFunctionOptions.ReadinessTimeout); err != nil {
		var errMessage string

		// try to get error logs
		containerLogs, getContainerLogsErr := p.dockerClient.GetContainerLogs(containerID)
		if getContainerLogsErr == nil {
			errMessage = fmt.Sprintf("Function wasn't ready in time. Logs:\n%s", containerLogs)
		} else {
			errMessage = fmt.Sprintf("Function wasn't ready in time (couldn't fetch logs: %s)", getContainerLogsErr.Error())
		}

		return nil, errors.Wrap(err, errMessage)
	}

	return &platform.CreateFunctionResult{
		Port:        functionHTTPPort,
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
		return previousHTTPPort, nil
	}

	// get a free local port
	freeLocalPort, err := p.getFreeLocalPort()
	if err != nil {
		return -1, errors.Wrap(err, "Failed to get free local port")
	}

	p.Logger.DebugWith("Found free local port", "port", freeLocalPort)

	return freeLocalPort, nil
}

func (p *Platform) getContainerNameByCreateFunctionOptions(createFunctionOptions *platform.CreateFunctionOptions) string {
	return fmt.Sprintf("%s-%s",
		createFunctionOptions.FunctionConfig.Meta.Namespace,
		createFunctionOptions.FunctionConfig.Meta.Name)
}

func (p *Platform) getContainerHTTPTriggerPort(container *dockerclient.Container) int {
	ports := container.HostConfig.PortBindings["8080/tcp"]
	if len(ports) == 0 {
		return 0
	}

	httpPort, _ := strconv.Atoi(ports[0].HostPort)

	return httpPort
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
		Name:    p.getContainerNameByCreateFunctionOptions(createFunctionOptions),
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
			previousHTTPPort = p.getContainerHTTPTriggerPort(&container)

			err = p.dockerClient.RemoveContainer(container.Name)
			if err != nil {
				return 0, errors.Wrap(err, "Failed to delete existing function")
			}
		}
	}

	return previousHTTPPort, nil
}
