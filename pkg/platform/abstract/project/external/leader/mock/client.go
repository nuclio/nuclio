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
