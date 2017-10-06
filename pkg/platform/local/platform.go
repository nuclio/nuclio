package local

import (
	"io"
	"net"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/dockerclient"

	"github.com/nuclio/nuclio-sdk"
)

type Platform struct {
	*platform.AbstractPlatform
	cmdRunner *cmdrunner.CmdRunner
	dockerClient *dockerclient.Client
}

// NewPlatform instantiates a new local platform
func NewPlatform(parentLogger nuclio.Logger) (*Platform, error) {

	// create base
	newAbstractPlatform, err := platform.NewAbstractPlatform(parentLogger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract platform")
	}

	// create platform
	newPlatform := &Platform{
		AbstractPlatform: newAbstractPlatform,
	}

	// create a command runner
	if newPlatform.cmdRunner, err = cmdrunner.NewCmdRunner(newPlatform.Logger); err != nil {
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
	var err error

	// first, check if the function exists so that we can delete it
	functions, err := p.GetFunctions(&platform.GetOptions{
		Common: &platform.CommonOptions{
			Identifier: deployOptions.Common.Identifier,
		},
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function")
	}

	// if the function exists, delete it
	if len(functions) > 0 {
		p.Logger.InfoWith("Function already exists, deleting")

		err = p.DeleteFunction(&platform.DeleteOptions{
			Common: &platform.CommonOptions{
				Identifier: deployOptions.Common.Identifier,
			},
		})

		if err != nil {
			return nil, errors.Wrap(err, "Failed to delete existing function")
		}
	}

	// if the image is not set, we need to build
	if deployOptions.ImageName == "" {
		deployOptions.ImageName, err = p.BuildFunction(&deployOptions.Build)
		if err != nil {
			return nil, errors.Wrap(err, "Faild to build image")
		}
	}

	// get a free local port
	// TODO: retry docker run if fails - there is a race on the local port since once getFreeLocalPort returns
	// the port becomes available
	freeLocalPort, err := p.getFreeLocalPort()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get free local port")
	}

	p.Logger.DebugWith("Found free local port", "port", freeLocalPort)

	// run the docker image
	_, err = p.dockerClient.RunContainer(deployOptions.ImageName, &dockerclient.RunOptions{
		Ports: map[int]int{freeLocalPort:8080},
		Labels: map[string]string{
			"nuclio-platform": "local",
			"nuclio-namespace": deployOptions.Common.Namespace,
			"nuclio-function-name": deployOptions.Common.Identifier,
		},
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to run docker container")
	}

	return &platform.DeployResult{
		Port: freeLocalPort,
	}, nil
}

// InvokeFunction will invoke a previously deployed function
func (p *Platform) InvokeFunction(invokeOptions *platform.InvokeOptions, writer io.Writer) error {
	return nil
}

// GetFunctions will return deployed functions
func (p *Platform) GetFunctions(getOptions *platform.GetOptions) ([]platform.Function, error) {
	getContainerOptions := &dockerclient.GetContainerOptions{
		Labels: map[string]string{
			"nuclio-platform": "local",
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
			"nuclio-platform": "local",
			"nuclio-namespace": deleteOptions.Common.Namespace,
			"nuclio-function-name": deleteOptions.Common.Identifier,
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

	return nil
}

func (p *Platform) getFreeLocalPort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
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
