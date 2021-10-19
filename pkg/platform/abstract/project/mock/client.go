package mock

import (
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/stretchr/testify/mock"
)

type Client struct {
	mock.Mock
}

func (c *Client) Initialize() error {
	args := c.Called()
	return args.Error(0)
}

func (c *Client) Get(getProjectsOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	args := c.Called(getProjectsOptions)
	return args.Get(0).([]platform.Project), args.Error(1)
}

func (c *Client) Create(createProjectOptions *platform.CreateProjectOptions) (platform.Project, error) {
	args := c.Called(createProjectOptions)
	return args.Get(0).(platform.Project), args.Error(1)
}

func (c *Client) Update(updateProjectOptions *platform.UpdateProjectOptions) (platform.Project, error) {
	args := c.Called(updateProjectOptions)
	return args.Get(0).(platform.Project), args.Error(1)
}

func (c *Client) Delete(deleteProjectOptions *platform.DeleteProjectOptions) error {
	args := c.Called(deleteProjectOptions)
	return args.Error(0)
}
