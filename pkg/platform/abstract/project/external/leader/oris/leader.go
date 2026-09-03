/*
Copyright 2026 The Nuclio Authors.

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

package oris

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nuclio/nuclio/pkg/platform"
	leaderCommon "github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader"
	leaderabstract "github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader/abstract"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

// LeaderOps implements leader.LeaderOps for the Oris projects leader.
type LeaderOps struct {
	*leaderabstract.LeaderOps
	logger logger.Logger
	// namespace is used to enrich Oris responses that omit the namespace.
	namespace string
}

// NewLeaderOps creates a new Oris LeaderOps.
func NewLeaderOps(parentLogger logger.Logger, namespace string) *LeaderOps {
	return &LeaderOps{
		LeaderOps: leaderabstract.NewLeaderOps(),
		logger:    parentLogger.GetChild("oris"),
		namespace: namespace,
	}
}

func (l *LeaderOps) GenerateProjectRequestBody(projectConfig *platform.ProjectConfig) ([]byte, error) {
	project, err := NewProjectRequestFromProjectConfig(projectConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create project request from project config")
	}
	return json.Marshal(project)
}

func (l *LeaderOps) GenerateProjectDeletionRequestBody(projectName string) ([]byte, error) {
	return []byte{}, nil
}

func (l *LeaderOps) ResolveCreateProjectResponse(ctx context.Context, body []byte) (leaderCommon.CreateProjectResponse, error) {
	project := OrisProject{}
	if err := json.Unmarshal(body, &project); err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal response body")
	}

	l.logger.DebugWithCtx(ctx,
		"Successfully sent create project request to leader",
		"projectName", project.Metadata.Name)
	return &project, nil
}

func (l *LeaderOps) ResolveGetProjectResponse(_ bool, body []byte) ([]platform.Project, error) {
	var projects OrisProjectList
	if err := json.Unmarshal(body, &projects); err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal response body")
	}

	return projects.ToProjectList(l.namespace), nil
}

// ProjectRequestURL overrides the shared default: Oris routes projects under its
// "projects" subdomain, so the path is .../projects/projects[/<name>].
func (l *LeaderOps) ProjectRequestURL(apiAddress string, apiVersion leaderCommon.APIVersion, projectName string) string {
	url := fmt.Sprintf("%s/%s/projects/projects", apiAddress, apiVersion)
	if projectName != "" {
		url += fmt.Sprintf("/%s", projectName)
	}
	return url
}

func (l *LeaderOps) GenerateCreateProjectRequestURL(apiAddress string) string {
	return l.ProjectRequestURL(apiAddress, leaderCommon.APIVersionV1, "")
}

func (l *LeaderOps) HandleCreateResponseErr(ctx context.Context, responseBody []byte, response *http.Response, err error) error {
	var projectResponse OrisProject

	if unmarshalErr := json.Unmarshal(responseBody, &projectResponse); unmarshalErr != nil {
		l.logger.WarnWithCtx(ctx,
			"Failed to unmarshal leader error response body",
			"err", err,
			"unmarshalErr", unmarshalErr)
		return errors.Wrap(unmarshalErr, "Failed to unmarshal response body")
	}

	if projectResponse.Status.ErrorMessage != "" {
		l.logger.ErrorWithCtx(ctx,
			"Create project has failed",
			"err", err,
			"responseError", projectResponse.Status.ErrorMessage,
			"responseStackTrace", projectResponse.Status.StackTrace)
		if response == nil {
			return errors.New("Failed to get response from leader, response is nil")
		}
		return nuclio.GetByStatusCode(int(projectResponse.Status.StatusCode))(projectResponse.Status.ErrorMessage)
	}
	return errors.Wrap(err, "Failed to send request to leader")
}

func (l *LeaderOps) GetDeleteExpectedStatusCode() int {
	return http.StatusNoContent
}

func (l *LeaderOps) GenerateUpdateProjectRequestURL(apiAddress, projectName string) string {
	return l.ProjectRequestURL(apiAddress, leaderCommon.APIVersionV1, projectName)
}

func (l *LeaderOps) GenerateGetProjectsRequestURL(apiAddress, projectName string) string {
	return l.ProjectRequestURL(apiAddress, leaderCommon.APIVersionV1, projectName)
}

func (l *LeaderOps) GenerateGetUpdatedAfterRequestURL(apiAddress string) string {
	return l.ProjectRequestURL(apiAddress, leaderCommon.APIVersionV1, "")
}

func (l *LeaderOps) GenerateDeleteProjectRequestURL(apiAddress, projectName string) string {
	return l.ProjectRequestURL(apiAddress, leaderCommon.APIVersionV1, projectName)
}
