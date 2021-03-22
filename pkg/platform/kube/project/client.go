package project

import (
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/nuclio/nuclio/pkg/platform"
)

type AbstractClient struct {
	Client
	Logger logger.Logger
}

func NewAbstractClient(parentLogger logger.Logger) (Client, error) {
	newClient := AbstractClient{}
	newClient.Logger = parentLogger.GetChild("projects-client")

	return newClient, nil
}

func (c AbstractClient) Initialize(p platform.Platform) error {
	return nil
}

func (c AbstractClient) Create(newProject *nuclioio.NuclioProject) (*nuclioio.NuclioProject, error) {
	return nil, nuclio.ErrNotImplemented
}

func (c AbstractClient) Update(project *nuclioio.NuclioProject) (*nuclioio.NuclioProject, error) {
	return nil, nuclio.ErrNotImplemented
}

func (c AbstractClient) Delete(deleteProjectOptions *platform.DeleteProjectOptions) error {
	return nuclio.ErrNotImplemented
}

func (c AbstractClient) Get(getProjectsOptions *platform.GetProjectsOptions) ([]nuclioio.NuclioProject, error) {
	return nil, nuclio.ErrNotImplemented
}
