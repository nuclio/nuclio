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

package mlrun

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/platform"
	leaderCommon "github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

type LeaderOps struct {
	logger logger.Logger
}

func NewLeaderOps(parentLogger logger.Logger) *LeaderOps {
	return &LeaderOps{
		logger: parentLogger.GetChild("mlrun"),
	}
}

func (l *LeaderOps) GenerateProjectRequestBody(projectConfig *platform.ProjectConfig) ([]byte, error) {
	project, err := NewProjectFromProjectConfig(projectConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create project from project config")
	}
	return json.Marshal(project)
}

func (l *LeaderOps) GenerateProjectDeletionRequestBody(projectName string) ([]byte, error) {
	return json.Marshal(MLRunProject{
		Metadata: ProjectMetadata{
			Name: projectName,
		},
	})
}

func (l *LeaderOps) ResolveCreateProjectResponse(ctx context.Context, body []byte) (leaderCommon.CreateProjectResponse, error) {
	project := MLRunProject{}
	if err := json.Unmarshal(body, &project); err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal response body")
	}

	l.logger.DebugWithCtx(ctx,
		"Successfully sent create project request to leader",
		"project name", project.Metadata.Name)
	return &project, nil
}

func (l *LeaderOps) ResolveGetProjectResponse(_ bool, body []byte) ([]platform.Project, error) {
	var projects MLRunProjectList
	if err := json.Unmarshal(body, &projects); err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal response body")
	}
	return projects.ToProjectList(), nil
}

func (l *LeaderOps) ParseJobStatusResponse(_ context.Context, _ []byte) (leaderCommon.JobResponse, bool) {
	// MLRun does not have async job handling, so this is a placeholder
	return nil, false
}

func (l *LeaderOps) GenerateCreateProjectRequestURL(apiAddress string) string {
	return fmt.Sprintf("%s/%s", apiAddress, "projects")
}

func (l *LeaderOps) HandleCreateResponseErr(ctx context.Context, responseBody []byte, response *http.Response, err error) error {
	// Try to parse MLRun error response
	var mlrunError MlrunError

	// try peek at error response
	if unmarshalErr := json.Unmarshal(responseBody, &mlrunError); unmarshalErr == nil {
		l.logger.ErrorWithCtx(ctx,
			"Create project has failed",
			"err", err,
			"responseError", mlrunError)
		if response == nil {
			return errors.New("Failed to get response from leader, response is nil")
		}
		return nuclio.GetByStatusCode(response.StatusCode)(mlrunError.Detail)
	}
	return errors.Wrap(err, "Failed to send request to leader")
}

func (l *LeaderOps) GetJobIdUrl(_, _ string) string {
	// MLRun does not have async job handling, so this is a placeholder
	return ""
}

func (l *LeaderOps) IsJobCompleted(_ context.Context, _ leaderCommon.JobResponse, _ string) error {
	// MLRun does not have async job handling, so this is a placeholder
	return nil
}

func (l *LeaderOps) GenerateUpdateProjectRequestURL(apiAddress, projectName string) string {
	return l.projectRequestURL(apiAddress, projectName)
}

func (l *LeaderOps) GetDeleteExpectedStatusCode() int {
	return http.StatusNoContent
}

func (l *LeaderOps) GetDeleteStrategyHeaderName() string {
	return "x-mlrun-deletion-strategy"
}

func (l *LeaderOps) GenerateGetProjectsRequestURL(_, _ string) string {
	return ""
}

func (l *LeaderOps) GenerateGetUpdatedAfterRequestURL(apiAddress string) string {
	// TODO - for now there is no filter addition to the URL, should be added when MLRun supports updated_at
	return fmt.Sprintf("%s/%s", apiAddress, "projects")
}

func (l *LeaderOps) GenerateDeleteProjectRequestURL(apiAddress, projectName string) string {
	return l.projectRequestURL(apiAddress, projectName)
}

func (l *LeaderOps) ShouldWaitForCreateCompletion() bool { return false }

func (l *LeaderOps) GetJobStatusRequestCookies(_ *platformconfig.Config) []*http.Cookie { return nil }

func (l *LeaderOps) GetJobRequestFilter(_ *time.Time) string { return "" }

func (l *LeaderOps) GetAuthSessionCookie(_ auth.Session) *http.Cookie { return nil }

func (l *LeaderOps) AddAuthSessionHeaders(_ map[string]string, _ auth.Session) {}

func (l *LeaderOps) projectRequestURL(apiAddress, projectName string) string {
	return fmt.Sprintf("%s/%s/%s", apiAddress, "projects", projectName)
}
