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
	"time"

	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/platform"
	leaderCommon "github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/stretchr/testify/mock"
)

type LeaderOps struct {
	mock.Mock
}

func NewLeaderOps() *LeaderOps {
	return &LeaderOps{}
}

// LeaderOps methods

func (l *LeaderOps) EvaluateLeaderRequest(ctx context.Context, labels map[string]string, existingProject platform.Project) (bool, error) {
	args := l.Called(ctx, labels, existingProject)
	return args.Bool(0), args.Error(1)
}

func (l *LeaderOps) ProjectSync2PCEnabled() bool {
	args := l.Called()
	return args.Bool(0)
}

// PrepareCreateProject2PC mocks leader.LeaderOps.PrepareCreateProject2PC.
func (l *LeaderOps) PrepareCreateProject2PC(ctx context.Context,
	options *platform.PrepareCreateProjectOptions) (*platform.Project2PCState, error) {
	args := l.Called(ctx, options)
	return args.Get(0).(*platform.Project2PCState), args.Error(1)
}

// CommitCreateProject2PC mocks leader.LeaderOps.CommitCreateProject2PC.
func (l *LeaderOps) CommitCreateProject2PC(ctx context.Context,
	options *platform.CommitCreateProjectOptions) (*platform.Project2PCState, error) {
	args := l.Called(ctx, options)
	return args.Get(0).(*platform.Project2PCState), args.Error(1)
}

// UpdateProject2PC mocks leader.LeaderOps.UpdateProject2PC.
func (l *LeaderOps) UpdateProject2PC(ctx context.Context,
	options *platform.CommitUpdateProjectOptions) (*platform.Project2PCState, error) {
	args := l.Called(ctx, options)
	return args.Get(0).(*platform.Project2PCState), args.Error(1)
}

// PrepareDeleteProject2PC mocks leader.LeaderOps.PrepareDeleteProject2PC.
func (l *LeaderOps) PrepareDeleteProject2PC(ctx context.Context,
	options *platform.PrepareDeleteProjectOptions) (*platform.Project2PCState, error) {
	args := l.Called(ctx, options)
	return args.Get(0).(*platform.Project2PCState), args.Error(1)
}

// CommitDeleteProject2PC mocks leader.LeaderOps.CommitDeleteProject2PC.
func (l *LeaderOps) CommitDeleteProject2PC(ctx context.Context,
	options *platform.CommitDeleteProjectOptions) (*platform.Project2PCState, error) {
	args := l.Called(ctx, options)
	return args.Get(0).(*platform.Project2PCState), args.Error(1)
}

// ListProject2PCStates mocks leader.LeaderOps.ListProject2PCStates.
func (l *LeaderOps) ListProject2PCStates(ctx context.Context,
	options *platform.ListProjectStatesOptions) (*platform.Project2PCStatesPage, error) {
	args := l.Called(ctx, options)
	return args.Get(0).(*platform.Project2PCStatesPage), args.Error(1)
}

func (l *LeaderOps) GenerateProjectRequestBody(projectConfig *platform.ProjectConfig) ([]byte, error) {
	args := l.Called(projectConfig)
	return args.Get(0).([]byte), args.Error(1)
}

func (l *LeaderOps) GenerateCreateProjectRequestURL(apiAddress string) string {
	args := l.Called(apiAddress)
	return args.String(0)
}

func (l *LeaderOps) HandleCreateResponseErr(ctx context.Context, body []byte, resp *http.Response, err error) error {
	args := l.Called(ctx, body, resp, err)
	return args.Error(0)
}

func (l *LeaderOps) ResolveCreateProjectResponse(_ context.Context, _ []byte) (leaderCommon.CreateProjectResponse, error) {
	return &CreateProjectResponseMock{}, nil
}

func (l *LeaderOps) ShouldWaitForCreateCompletion() bool {
	args := l.Called()
	return args.Bool(0)
}

func (l *LeaderOps) GetJobIdUrl(projectName, jobID string) string {
	args := l.Called(projectName, jobID)
	return args.String(0)
}

func (l *LeaderOps) ParseJobStatusResponse(ctx context.Context, body []byte) (leaderCommon.JobResponse, bool) {
	args := l.Called(ctx, body)
	return args.Get(0).(leaderCommon.JobResponse), args.Bool(1)
}

func (l *LeaderOps) IsJobCompleted(ctx context.Context, jobResponse leaderCommon.JobResponse, expectedState string) error {
	args := l.Called(ctx, jobResponse, expectedState)
	// If the return value is a function, call it with the arguments
	if fn, ok := args.Get(0).(func(context.Context, leaderCommon.JobResponse, string) error); ok {
		return fn(ctx, jobResponse, expectedState)
	}
	// Otherwise, treat as static error value
	if err, ok := args.Get(0).(error); ok {
		return err
	}
	return fmt.Errorf("unexpected return type from IsJobCompleted mock: %T", args.Get(0))
}

func (l *LeaderOps) GenerateUpdateProjectRequestURL(projectName, projectID string) string {
	args := l.Called(projectName, projectID)
	return args.String(0)
}

func (l *LeaderOps) GenerateDeleteProjectRequestURL(projectName, projectID string) string {
	args := l.Called(projectName, projectID)
	return args.String(0)
}

func (l *LeaderOps) GenerateProjectDeletionRequestBody(projectID string) ([]byte, error) {
	args := l.Called(projectID)
	return args.Get(0).([]byte), args.Error(1)
}

func (l *LeaderOps) GetDeleteExpectedStatusCode() int {
	args := l.Called()
	return args.Int(0)
}

func (l *LeaderOps) GetDeleteStrategyHeaderName() string {
	args := l.Called()
	return args.String(0)
}

func (l *LeaderOps) GenerateGetProjectsRequestURL(projectName, projectID string) string {
	args := l.Called(projectName, projectID)
	return args.String(0)
}

func (l *LeaderOps) ResolveGetProjectResponse(isSingle bool, body []byte) ([]platform.Project, error) {
	args := l.Called(isSingle, body)
	return args.Get(0).([]platform.Project), args.Error(1)
}

func (l *LeaderOps) GenerateGetUpdatedAfterRequestURL(updatedAfter string) string {
	args := l.Called(updatedAfter)
	return args.String(0)
}

func (l *LeaderOps) GetJobStatusRequestCookies(_ *platformconfig.Config) []*http.Cookie {
	args := l.Called()
	return args.Get(0).([]*http.Cookie)
}

func (l *LeaderOps) GetJobRequestFilter(updatedAfterTime *time.Time) string {
	args := l.Called(updatedAfterTime)
	return args.String(0)
}

func (l *LeaderOps) GetAuthSessionCookie(authSession auth.Session) *http.Cookie {
	args := l.Called(authSession)
	return args.Get(0).(*http.Cookie)
}

func (l *LeaderOps) AddAuthSessionHeaders(headers map[string]string, authSession auth.Session) {
	l.Called(headers, authSession)
}
