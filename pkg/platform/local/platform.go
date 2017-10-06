package local

import (
	"io"
	"net"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/renderer"

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
func (p *Platform) GetFunctions(getOptions *platform.GetOptions, writer io.Writer) error {
	containersInfo, err := p.dockerClient.GetContainers(&dockerclient.GetContainerOptions{
		Labels: map[string]string{
			"nuclio-platform": "local",
		},
	})

	if err != nil {
		return errors.Wrap(err, "Failed to get containers")
	}

	rendererInstance := renderer.NewRenderer(writer)

	switch getOptions.Format {
	case "text", "wide":
		header := []string{"Name", "State", "Node Port", "Replicas"}
		if getOptions.Format == "wide" {
			header = append(header, "Labels")
		}

		functionRecords := [][]string{}

		// for each field
		for _, containerInfo := range containersInfo {

			// get its fields
			functionFields := []string{
				containerInfo.Config.Labels["nuclio-function-name"],
				"RUNNING",
				containerInfo.HostConfig.PortBindings["8080/tcp"][0].HostPort,
				"1/1",
			}

			// add to records
			functionRecords = append(functionRecords, functionFields)
		}

		rendererInstance.RenderTable(header, functionRecords)
	}

	return nil
}

// UpdateFunction will update a previously deployed function
func (p *Platform) UpdateFunction(updateOptions *platform.UpdateOptions) error {
	return nil
}

// DeleteFunction will delete a previously deployed function
func (p *Platform) DeleteFunction(deleteOptions *platform.DeleteOptions) error {
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
