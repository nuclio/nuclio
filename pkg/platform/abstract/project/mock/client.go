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
