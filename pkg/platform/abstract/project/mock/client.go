package mock

import (
	"context"

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

func (c *Client) Get(ctx context.Context, getProjectsOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	args := c.Called(ctx, getProjectsOptions)
	return args.Get(0).([]platform.Project), args.Error(1)
}

func (c *Client) Create(ctx context.Context, createProjectOptions *platform.CreateProjectOptions) (platform.Project, error) {
	args := c.Called(ctx, createProjectOptions)
	return args.Get(0).(platform.Project), args.Error(1)
}

func (c *Client) Update(ctx context.Context, updateProjectOptions *platform.UpdateProjectOptions) (platform.Project, error) {
	args := c.Called(ctx, updateProjectOptions)
	return args.Get(0).(platform.Project), args.Error(1)
}

func (c *Client) Delete(ctx context.Context, deleteProjectOptions *platform.DeleteProjectOptions) error {
	args := c.Called(ctx, deleteProjectOptions)
	return args.Error(0)
}
