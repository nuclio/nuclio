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
	"github.com/nuclio/nuclio/pkg/platform/abstract"
	"github.com/nuclio/nuclio/pkg/processor/config"

	"github.com/nuclio/nuclio-sdk"
)

type Platform struct {
	*abstract.Platform
	cmdRunner    cmdrunner.CmdRunner
	dockerClient *dockerclient.Client
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
	if newPlatform.dockerClient, err = dockerclient.NewClient(newPlatform.Logger); err != nil {
		return nil, errors.Wrap(err, "Failed to create docker client")
	}

	return newPlatform, nil
}

// DeployFunction will simply run a docker image
func (p *Platform) DeployFunction(deployOptions *platform.DeployOptions) (*platform.DeployResult, error) {

	// local currently doesn't support registries of any kind. remove push / run registry
	deployOptions.RunRegistry = ""
	deployOptions.Build.Registry = ""

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
			"nuclio-namespace": getOptions.Namespace,
		},
	}

	// if we need to get only one function, specify its function name
	if getOptions.Identifier != "" {
		getContainerOptions.Labels["nuclio-function-name"] = getOptions.Identifier
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
			"nuclio-namespace":     deleteOptions.Namespace,
			"nuclio-function-name": deleteOptions.Identifier,
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

	p.Logger.InfoWith("Function deleted", "name", deleteOptions.Identifier)

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
		"nuclio-namespace":     deployOptions.Namespace,
		"nuclio-function-name": deployOptions.Identifier,
	}

	for labelName, labelValue := range common.StringToStringMap(deployOptions.Labels) {
		labels[labelName] = labelValue
	}

	// use function config path which is either passed by the user or detected during build (e.g. inline)
	if deployOptions.Build.FunctionConfigPath != "" {
		p.FunctionConfigToDeployOptions(deployOptions.Build.FunctionConfigPath, deployOptions)
	}

	// create processor configuration at a temporary location unless user specified a configuration
	localProcessorConfigPath, err := p.createProcessorConfig(deployOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create processor configuration")
	}

	// run the docker image
	_, err = p.dockerClient.RunContainer(deployOptions.ImageName, &dockerclient.RunOptions{
		Ports:  map[int]int{freeLocalPort: 8080},
		Env:    common.StringToStringMap(deployOptions.Env),
		Labels: labels,
		Volumes: map[string]string{
			localProcessorConfigPath: path.Join("/", "etc", "nuclio", "processor.yaml"),
		},
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to run docker container")
	}

	return &platform.DeployResult{
		Port: freeLocalPort,
	}, nil
}

func (p *Platform) createProcessorConfig(deployOptions *platform.DeployOptions) (string, error) {

	configWriter := config.NewWriter()

	// must specify "/tmp" here so that it's available on docker for mac
	processorConfigFile, err := ioutil.TempFile("/tmp", "processor-config-")
	if err != nil {
		return "", errors.Wrap(err, "Failed to create temporary processor config")
	}

	defer processorConfigFile.Close()

	if err = configWriter.Write(processorConfigFile,
		deployOptions.Build.Handler,
		deployOptions.Build.Runtime,
		"debug",
		deployOptions.DataBindings,
		deployOptions.Triggers); err != nil {
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
