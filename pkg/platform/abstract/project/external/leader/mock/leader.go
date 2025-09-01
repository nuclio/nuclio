/*
Copyright 2025 The Nuclio Authors.

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

	"github.com/nuclio/nuclio/pkg/platform"
	leaderCommon "github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader"

	"github.com/stretchr/testify/mock"
)

type Leader struct {
	mock.Mock
}

func NewLeader() *Leader {
	return &Leader{}
}

// LeaderOps methods

func (l *Leader) GenerateProjectRequestBody(projectConfig *platform.ProjectConfig) ([]byte, error) {
	args := l.Called(projectConfig)
	return args.Get(0).([]byte), args.Error(1)
}

func (l *Leader) GenerateCreateProjectRequestURL(projectName string) string {
	args := l.Called(projectName)
	return args.String(0)
}

func (l *Leader) HandleCreateResponseErr(ctx context.Context, body []byte, resp *http.Response, err error) error {
	args := l.Called(ctx, body, resp, err)
	return args.Error(0)
}

func (l *Leader) ResolveCreateProjectResponse(_ context.Context, _ []byte) (leaderCommon.CreateProjectResponse, error) {
	return &CreateProjectResponseMock{}, nil
}

func (l *Leader) ShouldWaitForCreateCompletion() bool {
	args := l.Called()
	return args.Bool(0)
}

func (l *Leader) GetJobIdUrl(projectName, jobID string) string {
	args := l.Called(projectName, jobID)
	return args.String(0)
}

func (l *Leader) IsJobTerminated(ctx context.Context, body []byte) (leaderCommon.JobResponse, bool) {
	args := l.Called(ctx, body)
	return args.Get(0).(leaderCommon.JobResponse), args.Bool(1)
}

func (l *Leader) ValidateJobState(ctx context.Context, jobResponse leaderCommon.JobResponse, expectedState string) error {
	args := l.Called(ctx, jobResponse, expectedState)
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

func (l *Leader) GenerateUpdateProjectRequestURL(projectName, projectID string) string {
	args := l.Called(projectName, projectID)
	return args.String(0)
}

func (l *Leader) GenerateDeleteProjectRequestURL(projectName, projectID string) string {
	args := l.Called(projectName, projectID)
	return args.String(0)
}

func (l *Leader) GenerateProjectDeletionRequestBody(projectID string) ([]byte, error) {
	args := l.Called(projectID)
	return args.Get(0).([]byte), args.Error(1)
}

func (l *Leader) GetDeleteExpectedStatusCode() int {
	args := l.Called()
	return args.Int(0)
}

func (l *Leader) GetDeleteStrategyHeaderName() string {
	args := l.Called()
	return args.String(0)
}

func (l *Leader) GenerateGetProjectsRequestURL(projectName, projectID string) string {
	args := l.Called(projectName, projectID)
	return args.String(0)
}

func (l *Leader) ResolveGetProjectResponse(isSingle bool, body []byte) ([]platform.Project, error) {
	args := l.Called(isSingle, body)
	return args.Get(0).([]platform.Project), args.Error(1)
}

func (l *Leader) GenerateGetUpdatedAfterRequestURL(updatedAfter string) string {
	args := l.Called(updatedAfter)
	return args.String(0)
}
