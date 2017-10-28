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
	"io"
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

func (p *Platform) BuildFunction(buildOptions *platform.BuildOptions) (*platform.BuildResult, error) {

	// called before staging objects are copied
	buildOptions.OnBeforeCopyObjectsToStagingDir = func() error {
		return p.createAndAddProcessorConfig(buildOptions,
			func(writer io.Writer, configWriter *config.Writer) error {

				return configWriter.Write(writer,
					buildOptions.Handler,
					buildOptions.Runtime,
					"debug",
					nil,
					nil)
			})
	}

	return p.AbstractPlatform.BuildFunction(buildOptions)
}

// DeployFunction will simply run a docker image
func (p *Platform) DeployFunction(deployOptions *platform.DeployOptions) (*platform.DeployResult, error) {

	// local currently doesn't support registries of any kind. remove push / run registry
	deployOptions.RunRegistry = ""
	deployOptions.Build.Registry = ""

	// if there's a configuration, populate the build/deploy options with its values
	deployOptions.Build.OnFunctionConfigFound = func(converter platform.FunctionConfigConverter) error {
		if err := converter.ToDeployOptions(deployOptions); err != nil {
			return errors.Wrap(err, "Failed to read function configuration (after build)")
		}

		return nil
	}

	// called before staging objects are copied
	deployOptions.Build.OnBeforeCopyObjectsToStagingDir = func() error {
		p.Logger.Debug("Creating processor configuration")

		return p.createAndAddProcessorConfig(&deployOptions.Build,
			func(writer io.Writer, configWriter *config.Writer) error {

				return configWriter.Write(writer,
					deployOptions.Build.Handler,
					deployOptions.Build.Runtime,
					"debug",
					deployOptions.DataBindings,
					deployOptions.Triggers)
		})
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

func (p *Platform) createAndAddProcessorConfig(buildOptions *platform.BuildOptions,
	configWriter func(io.Writer, *config.Writer) error) error {

	writer := config.NewWriter()

	processorConfigFile, err := ioutil.TempFile("", "processor-config-")
	if err != nil {
		return errors.Wrap(err, "Failed to create temporary processor config")
	}

	if err = configWriter(processorConfigFile, writer); err != nil {
		return errors.Wrap(err, "Failed to write processor config")
	}

	p.Logger.DebugWith("Wrote processor configuration", "path", processorConfigFile.Name())

	// read the file once for logging
	processorConfigContents, err := ioutil.ReadFile(processorConfigFile.Name())
	if err != nil {
		return errors.Wrap(err, "Failed to read processor configuration file")
	}

	// log
	p.Logger.DebugWith("Wrote processor configuration file", "contents", string(processorConfigContents))

	// add the processor.yaml we just created so that the image will be self-contained. other platforms
	// inject this at runtime but we don't want to risk it with volumes and such, for robustness
	buildOptions.AddedObjectPaths = map[string]string{
		processorConfigFile.Name(): path.Join("etc", "nuclio", "processor.yaml"),
	}

	return nil
}
