package mock

import (
	"time"

	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/stretchr/testify/mock"
)

type Client struct {
	mock.Mock
}

func NewClient() (*Client, error) {
	return &Client{}, nil
}

func (c *Client) Get(getProjectOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	args := c.Called(getProjectOptions)
	return args.Get(0).([]platform.Project), args.Error(1)
}

func (c *Client) Create(createProjectOptions *platform.CreateProjectOptions) error {
	args := c.Called(createProjectOptions)
	return args.Error(0)
}

func (c *Client) Update(updateProjectOptions *platform.UpdateProjectOptions) error {
	args := c.Called(updateProjectOptions)
	return args.Error(0)
}

func (c *Client) Delete(deleteProjectOptions *platform.DeleteProjectOptions) error {
	args := c.Called(deleteProjectOptions)
	return args.Error(0)
}

func (c *Client) GetUpdatedAfter(updatedAfterTime *time.Time) ([]platform.Project, error) {
	args := c.Called(updatedAfterTime)
	return args.Get(0).([]platform.Project), args.Error(1)
}
