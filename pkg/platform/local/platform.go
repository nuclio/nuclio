package local

import (
	"io/ioutil"
	"net"
	"path"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"

	"bytes"
	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/platform/abstract"
	"github.com/nuclio/nuclio/pkg/processor/config"
)

type Platform struct {
	*abstract.AbstractPlatform
	cmdRunner    cmdrunner.CmdRunner
	dockerClient *dockerclient.Client
}

// NewPlatform instantiates a new local platform
func NewPlatform(parentLogger nuclio.Logger) (*Platform, error) {
	newPlatform := &Platform{}

	// create base
	newAbstractPlatform, err := abstract.NewAbstractPlatform(parentLogger, newPlatform)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract platform")
	}

	// init platform
	newPlatform.AbstractPlatform = newAbstractPlatform

	// create a command runner
	if newPlatform.cmdRunner, err = cmdrunner.NewShellRunner(newPlatform.Logger); err != nil {
		return nil, errors.Wrap(err, "Failed to create command runner")
	}

	// create a docker client
	if newPlatform.dockerClient, err = dockerclient.NewClient(newPlatform.Logger); err != nil {
		return nil, errors.Wrap(err, "Failed to create docker client")
	}

	return newPlatform, nil
}

// DeployFunction will simply run a docker image
func (p *Platform) DeployFunction(deployOptions *platform.DeployOptions) (*platform.DeployResult, error) {
	functionConfigFound := false

	// local currently doesn't support registries of any kind. remove push / run registry
	deployOptions.RunRegistry = ""
	deployOptions.Build.Registry = ""

	// if there's a configuration, populate the build/deploy options with its values
	deployOptions.Build.OnFunctionConfigFound = func(functionConfigContents []byte) error {
		functionConfigFound = true

		// if there's a function yaml - update the deploy options with it
		err := deployOptions.ReadFunctionConfig(bytes.NewBuffer(functionConfigContents))
		if err != nil {
			return errors.Wrap(err, "Failed to read function configuration (after build)")
		}

		p.Logger.Debug("Creating processor configuration from function config")

		return p.createAndAddProcessorConfig(deployOptions)
	}

	// called before staging objects are copied
	deployOptions.Build.OnBeforeCopyObjectsToStagingDir = func() error {

		// if we already populated processor config, no need to create a processor
		// config
		if functionConfigFound {
			return nil
		}

		p.Logger.Debug("Creating processor configuration from defaults")

		return p.createAndAddProcessorConfig(deployOptions)
	}

	// wrap the deployer's deploy with the base HandleDeployFunction to provide lots of
	// common functionality
	return p.HandleDeployFunction(deployOptions, func() (*platform.DeployResult, error) {
		return p.deployFunction(deployOptions)
	})
}

// GetFunctions will return deployed functions
func (p *Platform) GetFunctions(getOptions *platform.GetOptions) ([]platform.Function, error) {
	getContainerOptions := &dockerclient.GetContainerOptions{
		Labels: map[string]string{
			"nuclio-platform":  "local",
			"nuclio-namespace": getOptions.Common.Namespace,
		},
	}

	// if we need to get only one function, specify its function name
	if getOptions.Common.Identifier != "" {
		getContainerOptions.Labels["nuclio-function-name"] = getOptions.Common.Identifier
	}

	containersInfo, err := p.dockerClient.GetContainers(getContainerOptions)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get containers")
	}

	var functions []platform.Function
	for _, containerInfo := range containersInfo {

		// create a local.function object which wraps a dockerclient.containerInfo and
		// implements platform.Function
		functions = append(functions, &function{containerInfo})
	}

	return functions, nil
}

// UpdateFunction will update a previously deployed function
func (p *Platform) UpdateFunction(updateOptions *platform.UpdateOptions) error {
	return nil
}

// DeleteFunction will delete a previously deployed function
func (p *Platform) DeleteFunction(deleteOptions *platform.DeleteOptions) error {
	getContainerOptions := &dockerclient.GetContainerOptions{
		Labels: map[string]string{
			"nuclio-platform":      "local",
			"nuclio-namespace":     deleteOptions.Common.Namespace,
			"nuclio-function-name": deleteOptions.Common.Identifier,
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

	p.Logger.InfoWith("Function deleted", "name", deleteOptions.Common.Identifier)

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

	// get a free local port
	// TODO: retry docker run if fails - there is a race on the local port since once getFreeLocalPort returns
	// the port becomes available
	freeLocalPort, err := p.getFreeLocalPort()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get free local port")
	}

	p.Logger.DebugWith("Found free local port", "port", freeLocalPort)

	labels := map[string]string{
		"nuclio-platform":      "local",
		"nuclio-namespace":     deployOptions.Common.Namespace,
		"nuclio-function-name": deployOptions.Common.Identifier,
	}

	for labelName, labelValue := range common.StringToStringMap(deployOptions.Labels) {
		labels[labelName] = labelValue
	}

	// run the docker image
	_, err = p.dockerClient.RunContainer(deployOptions.ImageName, &dockerclient.RunOptions{
		Ports:  map[int]int{freeLocalPort: 8080},
		Env:    common.StringToStringMap(deployOptions.Env),
		Labels: labels,
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to run docker container")
	}

	return &platform.DeployResult{
		Port: freeLocalPort,
	}, nil
}

func (p *Platform) createProcessorConfig(deployOptions *platform.DeployOptions) (string, error) {
	writer := config.NewWriter()

	processorYAMLFile, err := ioutil.TempFile("", "processor-yaml-")
	if err != nil {
		return "", errors.Wrap(err, "Failed to create temporary processor YAML")
	}

	// TODO: support logging
	err = writer.Write(processorYAMLFile,
		deployOptions.Build.Handler,
		deployOptions.Build.Runtime,
		"debug",
		deployOptions.DataBindings,
		deployOptions.Triggers)

	if err == nil {
		p.Logger.DebugWith("Wrote processor.yaml", "path", processorYAMLFile.Name())
	}

	return processorYAMLFile.Name(), err
}

func (p *Platform) createAndAddProcessorConfig(deployOptions *platform.DeployOptions) error {

	// create a temporary file holding the processor.yaml
	processorConfigPath, err := p.createProcessorConfig(deployOptions)
	if err != nil {
		return errors.Wrap(err, "Failed to create processor YAML")
	}

	// add the processor.yaml we just created so that the image will be self-contained. other platforms
	// inject this at runtime but we don't want to risk it with volumes and such, for robustness
	deployOptions.Build.AddedObjectPaths = map[string]string{
		processorConfigPath: path.Join("etc", "nuclio", "processor.yaml"),
	}

	return nil
}
