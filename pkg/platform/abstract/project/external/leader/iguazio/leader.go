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

package iguazio

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/platform"
	leadercommon "github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader"
	leaderabstract "github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader/abstract"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

type LeaderOps struct {
	*leaderabstract.LeaderOps
	logger logger.Logger
}

func NewLeaderOps(parentLogger logger.Logger) *LeaderOps {
	return &LeaderOps{
		LeaderOps: leaderabstract.NewLeaderOps(),
		logger:    parentLogger.GetChild("iguazio"),
	}
}

func (l *LeaderOps) GenerateProjectRequestBody(projectConfig *platform.ProjectConfig) ([]byte, error) {
	if projectConfig == nil {
		return nil, errors.New("Project config is missing")
	}

	project := NewProjectFromProjectConfig(projectConfig)
	return json.Marshal(project)
}

func (l *LeaderOps) GenerateProjectDeletionRequestBody(projectName string) ([]byte, error) {
	return json.Marshal(IguazioProject{
		Data: ProjectData{
			Type: ProjectType,
			Attributes: ProjectAttributes{
				Name: projectName,
			},
		},
	})
}

func (l *LeaderOps) ResolveCreateProjectResponse(ctx context.Context, body []byte) (leadercommon.CreateProjectResponse, error) {
	project := ProjectDetailResponse{}
	if err := json.Unmarshal(body, &project); err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal response body")
	}

	l.logger.DebugWithCtx(ctx,
		"Successfully sent create project request to leader",
		"igzCtx", project.Meta.Ctx,
		"projectData", project.Data)

	return &project, nil
}

func (l *LeaderOps) ResolveGetProjectResponse(detail bool, body []byte) ([]platform.Project, error) {

	var projectStructure GetProjectResponse
	if detail {
		projectStructure = &ProjectDetail{}
	} else {
		projectStructure = &ProjectList{}
	}

	if err := json.Unmarshal(body, projectStructure); err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal response body")
	}

	return projectStructure.ToSingleProjectList(), nil
}

func (l *LeaderOps) ParseJobStatusResponse(ctx context.Context, responseBody []byte) (leadercommon.JobResponse, bool) {
	var job JobDetailResponse
	if err := json.Unmarshal(responseBody, &job); err != nil {
		l.logger.DebugWithCtx(ctx, "Failed to unmarshal response body",
			"responseBody", responseBody)
		return nil, false
	}

	l.logger.DebugWithCtx(ctx,
		"Inspecting job state",
		"jobId", job.Meta,
		"igzCtx", job.Meta.Ctx,
		"jobAttributes", job.Data.Attributes)
	return &job, JobStateInSlice(job.Data.Attributes.State, []leadercommon.JobState{
		leadercommon.JobStateCompleted,
		leadercommon.JobStateCanceled,
		leadercommon.JobStateFailed,
	})
}

func (l *LeaderOps) GenerateCreateProjectRequestURL(apiAddress string) string {
	return l.projectRequestURL(apiAddress)
}

func (l *LeaderOps) HandleCreateResponseErr(ctx context.Context, responseBody []byte, _ *http.Response, err error) error {
	var responseError CreateProjectErrorResponse

	// try peek at error response
	if unmarshalErr := json.Unmarshal(responseBody, &responseError); unmarshalErr == nil {
		l.logger.ErrorWithCtx(ctx,
			"Create project has failed",
			"err", err,
			"responseError", responseError)
		if len(responseError.Errors) > 0 {
			firstError := responseError.Errors[0]

			// if no status was given, set as internal server error
			if firstError.Status == 0 {
				firstError.Status = http.StatusInternalServerError
			}
			return nuclio.GetByStatusCode(firstError.Status)(firstError.Detail)
		}
	}
	return errors.Wrap(err, "Failed to send request to leader")
}

func (l *LeaderOps) GetJobIdUrl(apiAddress, jobID string) string {
	return fmt.Sprintf("%s/%s/%s", apiAddress, "jobs", jobID)
}

func (l *LeaderOps) IsJobCompleted(ctx context.Context, job leadercommon.JobResponse, projectName string) error {
	if job == nil {
		return errors.New("JobResponse is nil")
	}
	if job.GetState() != leadercommon.JobStateCompleted {
		var jobResult struct {
			ProjectID string `json:"project_id,omitempty"`
			Status    int    `json:"status,omitempty"`
			Message   string `json:"message,omitempty"`
		}

		// try peek at job results to see if it has a meaningful error message
		if err := json.Unmarshal([]byte(job.GetResult()), &jobResult); err == nil {
			l.logger.ErrorWithCtx(ctx, "Create project has failed", "jobResult", jobResult)

			// assume server internal error if no status was given
			if jobResult.Status == 0 {
				jobResult.Status = http.StatusInternalServerError
			}
			if jobResult.Message == "" {
				jobResult.Message = "Failed to create project"
			}
			return nuclio.GetByStatusCode(jobResult.Status)(jobResult.Message)
		}

		return errors.Errorf("Create project has failed with unexpected state: %s",
			job.GetState())
	}
	l.logger.DebugWithCtx(ctx, "Successfully created project",
		"projectName", projectName,
		"projectJobCreationCtx", job.GetJobCreationCtx())

	return nil
}

func (l *LeaderOps) GenerateUpdateProjectRequestURL(apiAddress, projectName string) string {
	return fmt.Sprintf("%s/%s/%s", apiAddress, "projects/__name__", projectName)
}

func (l *LeaderOps) GetDeleteExpectedStatusCode() int {
	return http.StatusAccepted
}

func (l *LeaderOps) GetDeleteStrategyHeaderName() string {
	return "igz-project-deletion-strategy"
}

func (l *LeaderOps) GenerateGetProjectsRequestURL(apiAddress, projectName string) string {
	requestURL := l.projectRequestURL(apiAddress)
	if projectName != "" {
		requestURL += fmt.Sprintf("/__name__/%s", projectName)
	}

	// include namespace and username
	requestURL += "?include=owner&enrich_namespace=true"
	return requestURL
}

func (l *LeaderOps) GenerateGetUpdatedAfterRequestURL(apiAddress string) string {
	requestURL := l.projectRequestURL(apiAddress)
	requestURL += "?include=owner&enrich_namespace=true"
	return requestURL
}

func (l *LeaderOps) GenerateDeleteProjectRequestURL(apiAddress string, _ string) string {
	return l.projectRequestURL(apiAddress)
}

func (l *LeaderOps) EvaluateLeaderRequest(_ context.Context, _ map[string]string, _ platform.Project) (bool, error) {
	return true, nil
}

// ProjectSync2PCEnabled is always false for the Iguazio leader: the 2PC protocol is an
// MLRun-only concept. EvaluateLeaderRequest is an unconditional pass-through here.
func (l *LeaderOps) ProjectSync2PCEnabled() bool {
	return false
}

func (l *LeaderOps) ShouldWaitForCreateCompletion() bool { return true }

func (l *LeaderOps) GetJobStatusRequestCookies(config *platformconfig.Config) []*http.Cookie {
	var cookies []*http.Cookie
	if config.IguazioSessionCookie != "" {
		cookies = []*http.Cookie{{Name: "session", Value: config.IguazioSessionCookie}}
	}
	return cookies
}

func (l *LeaderOps) GetJobRequestFilter(updatedAfterTime *time.Time) string {
	return fmt.Sprintf("&filter[updated_at]=[$gt]%s", updatedAfterTime.Format(time.RFC3339Nano))
}

func (l *LeaderOps) GetAuthSessionCookie(authSession auth.Session) *http.Cookie {
	return &http.Cookie{
		Name:  "session",
		Value: url.QueryEscape(fmt.Sprintf(`j:{"sid":"%s"}`, authSession.GetPassword())),
	}
}

func (l *LeaderOps) AddAuthSessionHeaders(headers map[string]string, authSession auth.Session) {
	headers["authorization"] = authSession.CompileAuthorizationHeader()
}

func (l *LeaderOps) projectRequestURL(apiAddress string) string {
	return fmt.Sprintf("%s/%s", apiAddress, "projects")
}
