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

package leader

import (
	"context"
	"net/http"
	"time"

	"github.com/nuclio/nuclio/pkg/platform"
)

type Client interface {

	// Get delegate the project get to leader
	Get(context.Context, *platform.GetProjectsOptions) ([]platform.Project, error)

	// Create delegates project creation to leader
	Create(context.Context, *platform.CreateProjectOptions) error

	// Update delegates project update to leader
	Update(context.Context, *platform.UpdateProjectOptions) error

	// Delete delegates project deletion to leader
	Delete(context.Context, *platform.DeleteProjectOptions) error

	// GetUpdatedAfter gets all projects from the leader that updated after the given time (to get all, pass nil time)
	GetUpdatedAfter(context.Context, *time.Time) ([]platform.Project, error)
}

type LeaderOps interface {

	// Create operations

	// GenerateProjectRequestBody generates the request body for project creation
	GenerateProjectRequestBody(*platform.ProjectConfig) ([]byte, error)

	// GenerateCreateProjectRequestURL generates the URL for project creation
	GenerateCreateProjectRequestURL(string) string

	// HandleCreateResponseErr handles errors in the response of project creation
	HandleCreateResponseErr(context.Context, []byte, *http.Response, error) error

	// ResolveCreateProjectResponse resolves the response from project creation
	ResolveCreateProjectResponse(context.Context, []byte) (CreateProjectResponse, error)

	// ShouldWaitForCreateCompletion indicates whether to wait for job completion after project creation
	ShouldWaitForCreateCompletion() bool

	// WaitForJobCompletion operations

	// GetJobIdUrl generates the URL to get job status
	GetJobIdUrl(string, string) string

	// IsJobTerminated parses the job status response and returns whether the job and if it is terminated
	IsJobTerminated(context.Context, []byte) (JobResponse, bool)

	// ValidateJobState validates the job state
	ValidateJobState(context.Context, JobResponse, string) error

	// Update operations

	// GenerateUpdateProjectRequestURL generates the request URL for project update
	GenerateUpdateProjectRequestURL(string, string) string

	// Delete operations

	// GenerateDeleteProjectRequestURL generates the request URL for project deletion
	GenerateDeleteProjectRequestURL(string, string) string

	// GenerateProjectDeletionRequestBody generates the request body for project deletion
	GenerateProjectDeletionRequestBody(string) ([]byte, error)

	// GetDeleteExpectedStatusCode returns the expected status code from the http response
	GetDeleteExpectedStatusCode() int

	// GetDeleteStrategyHeaderName gets the delete strategy header to the request
	GetDeleteStrategyHeaderName() string

	// Get operations

	// GenerateGetProjectsRequestURL generates the request URL for getting projects
	GenerateGetProjectsRequestURL(string, string) string

	// ResolveGetProjectResponse resolves the response from getting projects
	ResolveGetProjectResponse(bool, []byte) ([]platform.Project, error)

	// GetUpdatedAfter operations

	// GenerateGetUpdatedAfterRequestURL generates the request URL for getting projects
	GenerateGetUpdatedAfterRequestURL(string) string
}

type CreateProjectResponse interface {
	GetLastJobID() string
}

type JobResponse interface {
	GetState() JobState
	GetResult() string
	GetJobCreationCtx() string
}

type JobState string

const (
	JobStateCompleted JobState = "completed"
	JobStateCanceled  JobState = "canceled"
	JobStateFailed    JobState = "failed"
)

const (
	ProjectTimeLayout = "2006-01-02T15:04:05.000000+00:00"
)

func ParseTimeFromTimestamp(timestamp string) time.Time {
	t, _ := time.Parse(ProjectTimeLayout, timestamp)
	return t
}
