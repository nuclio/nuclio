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
	"fmt"
	"net/http"
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

// ClientOps methods

func (c *Client) GenerateProjectRequestBody(projectConfig *platform.ProjectConfig) ([]byte, error) {
	args := c.Called(projectConfig)
	return args.Get(0).([]byte), args.Error(1)
}

func (c *Client) GenerateCreateProjectRequestURL(projectName string) string {
	args := c.Called(projectName)
	return args.String(0)
}

func (c *Client) HandleCreateResponseErr(ctx context.Context, body []byte, resp *http.Response, err error) error {
	args := c.Called(ctx, body, resp, err)
	return args.Error(0)
}

func (c *Client) ResolveCreateProjectResponse(_ context.Context, _ []byte) (leaderCommon.CreateProjectResponse, error) {
	return &CreateProjectResponseMock{}, nil
}

func (c *Client) ShouldWaitForCreateCompletion() bool {
	args := c.Called()
	return args.Bool(0)
}

func (c *Client) GetJobIdUrl(projectName, jobID string) string {
	args := c.Called(projectName, jobID)
	return args.String(0)
}

func (c *Client) IsJobTerminated(ctx context.Context, body []byte) (leaderCommon.JobResponse, bool) {
	args := c.Called(ctx, body)
	return args.Get(0).(leaderCommon.JobResponse), args.Bool(1)
}

func (c *Client) ValidateJobState(ctx context.Context, jobResponse leaderCommon.JobResponse, expectedState string) error {
	args := c.Called(ctx, jobResponse, expectedState)
	// If the return value is a function, call it with the arguments
	if fn, ok := args.Get(0).(func(context.Context, leaderCommon.JobResponse, string) error); ok {
		return fn(ctx, jobResponse, expectedState)
	}
	// Otherwise, treat as static error value
	if err, ok := args.Get(0).(error); ok {
		return err
	}
	return fmt.Errorf("unexpected return type from ValidateJobState mock: %T", args.Get(0))
}

func (c *Client) GenerateUpdateProjectRequestURL(projectName, projectID string) string {
	args := c.Called(projectName, projectID)
	return args.String(0)
}

func (c *Client) GenerateDeleteProjectRequestURL(projectName, projectID string) string {
	args := c.Called(projectName, projectID)
	return args.String(0)
}

func (c *Client) GenerateProjectDeletionRequestBody(projectID string) ([]byte, error) {
	args := c.Called(projectID)
	return args.Get(0).([]byte), args.Error(1)
}

func (c *Client) GetDeleteExpectedStatusCode() int {
	args := c.Called()
	return args.Int(0)
}

func (c *Client) AddDeleteStrategyHeader(headers map[string]string, strategy platform.DeleteProjectStrategy) {
	c.Called(headers, strategy)
}

func (c *Client) GenerateGetProjectsRequestURL(projectName, projectID string) string {
	args := c.Called(projectName, projectID)
	return args.String(0)
}

func (c *Client) ResolveGetProjectResponse(isSingle bool, body []byte) ([]platform.Project, error) {
	args := c.Called(isSingle, body)
	return args.Get(0).([]platform.Project), args.Error(1)
}

func (c *Client) GenerateGetUpdatedAfterRequestURL(updatedAfter string) string {
	args := c.Called(updatedAfter)
	return args.String(0)
}
