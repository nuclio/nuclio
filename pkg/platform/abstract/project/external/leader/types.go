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

	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platformconfig"
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

	// EvaluateLeaderRequest determines the 2PC phase from labels, validates it against
	// the current CRD state, and signals whether the caller should apply the change.
	// existingProject is nil when no CRD exists.
	// Returns (true, nil)  – validation passed, caller should write the CRD.
	// Returns (false, nil) – idempotent replay, caller should skip the write and return existing.
	// Returns (false, err) – validation failed (400 / 409 / 412).
	EvaluateLeaderRequest(ctx context.Context, labels map[string]string, existingProject platform.Project) (bool, error)

	// ProjectSync2PCEnabled reports whether the configured leader runs the two-phase-commit
	// project sync protocol. When false, EvaluateLeaderRequest is an unconditional
	// pass-through and callers can skip fetching the existing CRD before invoking it.
	// True only for MLRun with the 2PC feature flag on; Iguazio and disabled-MLRun return false.
	ProjectSync2PCEnabled() bool
}

type LeaderOps interface {
	// EvaluateLeaderRequest determines the 2PC phase purely from labels, validates it
	// against the current CRD state, and signals whether the caller should apply the change.
	// existingProject is nil when no CRD exists.
	// Returns (true, nil)  – validation passed, caller should write the CRD.
	// Returns (false, nil) – idempotent replay, caller should skip the write.
	// Returns (false, err) – validation failed (400 / 409 / 412).
	EvaluateLeaderRequest(ctx context.Context, labels map[string]string, existingProject platform.Project) (bool, error)

	// ProjectSync2PCEnabled reports whether this leader runs the two-phase-commit project
	// sync protocol. When false, EvaluateLeaderRequest is an unconditional pass-through
	// and the existing CRD is never inspected — callers can use this to skip a redundant
	// Kubernetes Get before each leader-origin write.
	ProjectSync2PCEnabled() bool

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

	// GetAuthSessionCookie returns the authentication session cookie for the request
	GetAuthSessionCookie(auth.Session) *http.Cookie

	// AddAuthSessionHeaders adds authentication session headers to the request
	AddAuthSessionHeaders(map[string]string, auth.Session)

	// WaitForJobCompletion operations

	// GetJobIdUrl generates the URL to get job status
	GetJobIdUrl(string, string) string

	// ParseJobStatusResponse parses the job status response and returns whether the job and if it is terminated
	ParseJobStatusResponse(context.Context, []byte) (JobResponse, bool)

	// IsJobCompleted validates the job state
	IsJobCompleted(context.Context, JobResponse, string) error

	// GetJobStatusRequestCookies returns any cookies needed for the job status request
	GetJobStatusRequestCookies(*platformconfig.Config) []*http.Cookie

	// GetJobRequestFilter returns any filter needed for the job status request URL
	GetJobRequestFilter(*time.Time) string

	// Update operations

	// GenerateUpdateProjectRequestURL generates the request URL for project update
	GenerateUpdateProjectRequestURL(string, string) string

	// Delete operations

	// GenerateDeleteProjectRequestURL generates the request URL for project deletion
	GenerateDeleteProjectRequestURL(string, string) string

	// GenerateProjectDeletionRequestBody generates the request body for project deletion
	GenerateProjectDeletionRequestBody(string) ([]byte, error)

	// GetExpectedStatusCode returns the expected status code from the http response for
	// the given project write operation
	GetExpectedStatusCode(ProjectOperation) int

	// GetDeleteStrategyHeaderName gets the delete strategy header to the request
	GetDeleteStrategyHeaderName() string

	// Get operations

	// GenerateGetProjectsRequestURL generates the request URL for getting projects or a single project
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

type APIVersion string

const (
	APIVersionV1 APIVersion = "v1"
	APIVersionV2 APIVersion = "v2"
)

// ProjectOperation identifies a project write operation, so a leader can report the
// HTTP status code it expects to see for that specific operation.
type ProjectOperation string

const (
	ProjectOperationCreate ProjectOperation = "create"
	ProjectOperationUpdate ProjectOperation = "update"
	ProjectOperationDelete ProjectOperation = "delete"
)

const (
	ProjectTimeLayout   = "2006-01-02T15:04:05.000000+00:00"
	ProjectOnlineStatus = "online"
)

// 2PC sync label keys written on NuclioProject CRDs by the external client.
const (
	MLRunLabelKeySyncStatus  = "mlrun/sync-status"
	MLRunLabelKeyOpID        = "mlrun/op-id"
	MLRunLabelKeyCurrentOpID = "mlrun/current-op-id"
)

// 2PC sync status values stored under MLRunLabelKeySyncStatus.
const (
	MLRunSyncStatusCreating = "creating"
	MLRunSyncStatusOnline   = "online"
	MLRunSyncStatusDeleting = "deleting"
)

// 2PC sync label keys written on NuclioProject CRDs by internalc/kube.Client's follower
// operations (the dedicated /api/v1/follower/projects/* surface).
const (
	OrisLabelKeySyncStatus = "oris/sync-status"
	OrisLabelKeyOpID       = "oris/op-id"
)

// OrisSyncStatus is the 2PC phase stamped on a NuclioProject CRD's labels by internalc/kube.Client's
// follower operations.
type OrisSyncStatus string

const (
	OrisSyncStatusCreating OrisSyncStatus = "creating"
	OrisSyncStatusOnline   OrisSyncStatus = "online"
	OrisSyncStatusDeleting OrisSyncStatus = "deleting"
)

func ParseTimeFromTimestamp(timestamp string) time.Time {
	t, _ := time.Parse(ProjectTimeLayout, timestamp)
	return t
}
