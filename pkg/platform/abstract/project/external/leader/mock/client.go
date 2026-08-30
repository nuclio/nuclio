/*
Copyright 2023 The Nuclio Authors.

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
	"time"

	"github.com/nuclio/nuclio/pkg/platform"
	leaderCommon "github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader"

	"github.com/stretchr/testify/mock"
)

type MockProject struct {
	mock.Mock
}

func (c *MockProject) GetConfig() *platform.ProjectConfig {
	args := c.Called()
	return args.Get(0).(*platform.ProjectConfig)
}

func (c *MockProject) IsProjectOnline() bool {
	args := c.Called()
	return args.Bool(0)
}

type CreateProjectResponseMock struct {
	mock.Mock
}

func (c *CreateProjectResponseMock) GetLastJobID() string {
	return "test-job-id"
}

type JobResponseMock struct {
	mock.Mock
}

func (j *JobResponseMock) GetState() leaderCommon.JobState {
	args := j.Called()
	return leaderCommon.JobState(args.String(0))
}

func (j *JobResponseMock) GetResult() string {
	args := j.Called()
	return args.String(0)
}

func (j *JobResponseMock) GetJobCreationCtx() string {
	args := j.Called()
	return args.String(0)
}

type Client struct {
	mock.Mock
}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) Get(ctx context.Context, getProjectOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	args := c.Called(ctx, getProjectOptions)
	return args.Get(0).([]platform.Project), args.Error(1)
}

func (c *Client) Create(ctx context.Context, createProjectOptions *platform.CreateProjectOptions) error {
	args := c.Called(ctx, createProjectOptions)
	return args.Error(0)
}

func (c *Client) Update(ctx context.Context, updateProjectOptions *platform.UpdateProjectOptions) error {
	args := c.Called(ctx, updateProjectOptions)
	return args.Error(0)
}

func (c *Client) Delete(ctx context.Context, deleteProjectOptions *platform.DeleteProjectOptions) error {
	args := c.Called(ctx, deleteProjectOptions)
	return args.Error(0)
}

func (c *Client) GetUpdatedAfter(ctx context.Context, updatedAfterTime *time.Time) ([]platform.Project, error) {
	args := c.Called(ctx, updatedAfterTime)
	return args.Get(0).([]platform.Project), args.Error(1)
}

func (c *Client) EvaluateLeaderRequest(ctx context.Context, labels map[string]string, existingProject platform.Project) (bool, error) {
	args := c.Called(ctx, labels, existingProject)
	return args.Bool(0), args.Error(1)
}

func (c *Client) ProjectSync2PCEnabled() bool {
	args := c.Called()
	return args.Bool(0)
}

// PrepareCreateProject2PC mocks leader.Client.PrepareCreateProject2PC.
func (c *Client) PrepareCreateProject2PC(ctx context.Context,
	options *platform.PrepareCreateProjectOptions) (*platform.Project2PCState, error) {
	args := c.Called(ctx, options)
	return args.Get(0).(*platform.Project2PCState), args.Error(1)
}

// CommitCreateProject2PC mocks leader.Client.CommitCreateProject2PC.
func (c *Client) CommitCreateProject2PC(ctx context.Context,
	options *platform.CommitCreateProjectOptions) (*platform.Project2PCState, error) {
	args := c.Called(ctx, options)
	return args.Get(0).(*platform.Project2PCState), args.Error(1)
}

// UpdateProject2PC mocks leader.Client.UpdateProject2PC.
func (c *Client) UpdateProject2PC(ctx context.Context,
	options *platform.CommitUpdateProjectOptions) (*platform.Project2PCState, error) {
	args := c.Called(ctx, options)
	return args.Get(0).(*platform.Project2PCState), args.Error(1)
}

// PrepareDeleteProject2PC mocks leader.Client.PrepareDeleteProject2PC.
func (c *Client) PrepareDeleteProject2PC(ctx context.Context,
	options *platform.PrepareDeleteProjectOptions) (*platform.Project2PCState, error) {
	args := c.Called(ctx, options)
	return args.Get(0).(*platform.Project2PCState), args.Error(1)
}

// CommitDeleteProject2PC mocks leader.Client.CommitDeleteProject2PC.
func (c *Client) CommitDeleteProject2PC(ctx context.Context,
	options *platform.CommitDeleteProjectOptions) (*platform.Project2PCState, error) {
	args := c.Called(ctx, options)
	return args.Get(0).(*platform.Project2PCState), args.Error(1)
}

// ListProject2PCStates mocks leader.Client.ListProject2PCStates.
func (c *Client) ListProject2PCStates(ctx context.Context,
	options *platform.ListProjectStatesOptions) (*platform.Project2PCStatesPage, error) {
	args := c.Called(ctx, options)
	return args.Get(0).(*platform.Project2PCStatesPage), args.Error(1)
}
