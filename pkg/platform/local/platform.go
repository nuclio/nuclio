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
	"encoding/json"
	"io/ioutil"
	"net"
	"path"
	"strconv"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/config"

	"github.com/nuclio/nuclio-sdk"
)

type Platform struct {
	*abstract.Platform
	cmdRunner    cmdrunner.CmdRunner
	dockerClient dockerclient.Client
}

// NewPlatform instantiates a new local platform
func NewPlatform(parentLogger nuclio.Logger) (*Platform, error) {
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

	return newPlatform, nil
}

// DeployFunction will simply run a docker image
func (p *Platform) DeployFunction(deployOptions *platform.DeployOptions) (*platform.DeployResult, error) {

	// local currently doesn't support registries of any kind. remove push / run registry
	deployOptions.FunctionConfig.Spec.RunRegistry = ""
	deployOptions.FunctionConfig.Spec.Build.Registry = ""

	// wrap the deployer's deploy with the base HandleDeployFunction to provide lots of
	// common functionality
	return p.HandleDeployFunction(deployOptions, func() (*platform.DeployResult, error) {
		return p.deployFunction(deployOptions)
	})
}

// GetFunctions will return deployed functions
func (p *Platform) GetFunctions(getOptions *platform.GetOptions) ([]platform.Function, error) {
	var functions []platform.Function

	// Iterate over getOptions.MatchCriterias and append functions accordingly
	for _, matchCriteria := range getOptions.MatchCriterias {

		// make default Docker ContainerOptions
		currentDockerContainerOptions := &dockerclient.GetContainerOptions{
			Labels: map[string]string{
				"nuclio-platform":  "local",
				"nuclio-namespace": getOptions.Namespace,
			},
		}

		// check if given function has a name, insert accordingly (needed because containerOptions.Labels needs to be
		// added only when full)
		if matchCriteria.Name != "" {
			currentDockerContainerOptions.Labels["nuclio-function-name"] = matchCriteria.Name
		}

		// try get containers
		containersInfo, err := p.dockerClient.GetContainers(currentDockerContainerOptions)

		// alert if failed to get containers
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get containers")
		}

		// iterate over containersInfo, for every info create a function accordingly
		for _, containerInfo := range containersInfo {
			httpPort, _ := strconv.Atoi(containerInfo.HostConfig.PortBindings["8080/tcp"][0].HostPort)
			var functionSpec functionconfig.Spec

			// get the JSON encoded spec
			encodedFunctionSpec, encodedFunctionSpecFound := containerInfo.Config.Labels["nuclio-function-spec"]
			if encodedFunctionSpecFound {

				// try to unmarshal the spec
				json.Unmarshal([]byte(encodedFunctionSpec), &functionSpec)
			}

			// set default values
			functionSpec.Version = -1
			functionSpec.HTTPPort = httpPort

			// delete info no longer needed in containerInfo
			delete(containerInfo.Config.Labels, "nuclio-function-spec")

			// initialize new function
			function, err := newFunction(p.Logger,
				p,
				&functionconfig.Config{
					Meta: functionconfig.Meta{
						Name:      containerInfo.Config.Labels["nuclio-function-name"],
						Namespace: "n/a",
						Labels:    containerInfo.Config.Labels,
					},
					Spec: functionSpec,
				}, &containerInfo)

			if err != nil {
				return nil, errors.Wrap(err, "Failed to create function")
			}

			// create a local.function object which wraps a dockerclient.containerInfo and
			// implements platform.Function
			functions = append(functions, function)
		}
	}

	return functions, nil
}

// UpdateFunction will update a previously deployed function
func (p *Platform) UpdateFunction(updateOptions *platform.UpdateOptions) error {
	return nil
}

// DeleteFunctions will delete a previously deployed function
func (p *Platform) DeleteFunctions(deleteOptions *platform.DeleteOptions) error {
	for _, functionConfig := range deleteOptions.FunctionConfigs {
		containerOptions := &dockerclient.GetContainerOptions{
			Labels: map[string]string{
				"nuclio-platform":  "local",
				"nuclio-namespace": functionConfig.Meta.Namespace,
			},
		}

		// check if given function has a name, insert accordingly (needed because containerOptions.Labels needs to be
		// added only when full)
		if functionConfig.Meta.Name != "" {
			containerOptions.Labels["nuclio-function-name"] = functionConfig.Meta.Name
		}

		containersInfo, err := p.dockerClient.GetContainers(containerOptions)
		if err != nil {
			return errors.Wrap(err, "Failed to get containers")
		}

		// iterate over contains and delete them. It's possible that under some weird circumstances
		// there are a few instances of this function in the namespace
		for _, containerInfo := range containersInfo {
			if err := p.dockerClient.RemoveContainer(containerInfo.ID); err == nil {
				p.Logger.InfoWith("Function deleted", "name", containerInfo.Config.Labels["nuclio-function-name"])
			}
		}
	}

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

func (p *Platform) getFreeLocalPort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}

	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func (p *Platform) deployFunction(deployOptions *platform.DeployOptions) (*platform.DeployResult, error) {

	// get function port - either from configuration or from a free port
	functionHTTPPort, err := p.getFunctionHTTPPort(deployOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function HTTP port")
	}

	labels := map[string]string{
		"nuclio-platform":      "local",
		"nuclio-namespace":     deployOptions.FunctionConfig.Meta.Namespace,
		"nuclio-function-name": deployOptions.FunctionConfig.Meta.Name,
		"nuclio-function-spec": p.encodeFunctionSpec(&deployOptions.FunctionConfig.Spec),
	}

	for labelName, labelValue := range deployOptions.FunctionConfig.Meta.Labels {
		labels[labelName] = labelValue
	}

	// create processor configuration at a temporary location unless user specified a configuration
	localProcessorConfigPath, err := p.createProcessorConfig(deployOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create processor configuration")
	}

	envMap := map[string]string{}
	for _, env := range deployOptions.FunctionConfig.Spec.Env {
		envMap[env.Name] = env.Value
	}

	// run the docker image
	containerID, err := p.dockerClient.RunContainer(deployOptions.FunctionConfig.Spec.ImageName, &dockerclient.RunOptions{
		Ports:  map[int]int{functionHTTPPort: 8080},
		Env:    envMap,
		Labels: labels,
		Volumes: map[string]string{
			localProcessorConfigPath: path.Join("/", "etc", "nuclio", "config", "processor", "processor.yaml"),
		},
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to run docker container")
	}

	return &platform.DeployResult{
		Port:        functionHTTPPort,
		ContainerID: containerID,
	}, nil
}

func (p *Platform) createProcessorConfig(deployOptions *platform.DeployOptions) (string, error) {

	configWriter, err := processorconfig.NewWriter()
	if err != nil {
		return "", errors.Wrap(err, "Failed to create processor configuration writer")
	}

	// must specify "/tmp" here so that it's available on docker for mac
	processorConfigFile, err := ioutil.TempFile("/tmp", "processor-config-")
	if err != nil {
		return "", errors.Wrap(err, "Failed to create temporary processor config")
	}

	defer processorConfigFile.Close()

	if err = configWriter.Write(processorConfigFile, &processor.Configuration{
		Config: deployOptions.FunctionConfig,
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

func (p *Platform) getFunctionHTTPPort(deployOptions *platform.DeployOptions) (int, error) {

	// if the configuration specified an HTTP port - use that
	if deployOptions.FunctionConfig.Spec.HTTPPort != 0 {
		p.Logger.DebugWith("Configuration specified HTTP port",
			"port",
			deployOptions.FunctionConfig.Spec.HTTPPort)

		return deployOptions.FunctionConfig.Spec.HTTPPort, nil
	}

	// get a free local port
	freeLocalPort, err := p.getFreeLocalPort()
	if err != nil {
		return -1, errors.Wrap(err, "Failed to get free local port")
	}

	p.Logger.DebugWith("Found free local port", "port", freeLocalPort)

	return freeLocalPort, nil
}
