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

	"github.com/nuclio/nuclio/pkg/platform"
	leaderCommon "github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

type Client struct {
	logger logger.Logger
}

func NewClient(parentLogger logger.Logger) *Client {
	return &Client{
		logger: parentLogger.GetChild("mlrun"),
	}
}

func (c *Client) GenerateProjectRequestBody(projectConfig *platform.ProjectConfig) ([]byte, error) {
	project, err := NewProjectFromProjectConfig(projectConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create project from project config")
	}
	return json.Marshal(project)
}

func (c *Client) GenerateProjectDeletionRequestBody(projectName string) ([]byte, error) {
	return json.Marshal(MLRunProject{
		Metadata: ProjectMetadata{
			Name: projectName,
		},
	})
}

func (c *Client) ResolveCreateProjectResponse(ctx context.Context, body []byte) (leaderCommon.CreateProjectResponse, error) {
	project := MLRunProject{}
	if err := json.Unmarshal(body, &project); err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal response body")
	}

	c.logger.DebugWithCtx(ctx,
		"Successfully sent create project request to leader",
		"project name", project.Metadata.Name)
	return &project, nil
}

func (c *Client) ResolveGetProjectResponse(_ bool, _ []byte) ([]platform.Project, error) {
	// will be implemented as part of synchronizer task
	return nil, nuclio.ErrNotImplemented
}

func (c *Client) IsJobTerminated(_ context.Context, _ []byte) (leaderCommon.JobResponse, bool) {
	// MLRun does not have async job handling, so this is a placeholder
	return nil, false
}

func (c *Client) GenerateCreateProjectRequestURL(apiAddress string) string {
	return fmt.Sprintf("%s/%s", apiAddress, "projects")
}

func (c *Client) HandleCreateResponseErr(ctx context.Context, responseBody []byte, response *http.Response, err error) error {
	// Try to parse MLRun error response
	var mlrunError MlrunError

	// try peek at error response
	if unmarshalErr := json.Unmarshal(responseBody, &mlrunError); unmarshalErr == nil {
		c.logger.ErrorWithCtx(ctx,
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

func (c *Client) GetJobIdUrl(_, _ string) string {
	// MLRun does not have async job handling, so this is a placeholder
	return ""
}

func (c *Client) ValidateJobState(_ context.Context, _ leaderCommon.JobResponse, _ string) error {
	// MLRun does not have async job handling, so this is a placeholder
	return nil
}

func (c *Client) GenerateUpdateProjectRequestURL(apiAddress, projectName string) string {
	return c.projectRequestURL(apiAddress, projectName)
}

func (c *Client) GetDeleteExpectedStatusCode() int {
	return http.StatusNoContent
}

func (c *Client) GetDeleteStrategyHeaderName() string {
	return "x-mlrun-deletion-strategy"
}

func (c *Client) GenerateGetProjectsRequestURL(_, _ string) string {
	return ""
}

func (c *Client) GenerateGetUpdatedAfterRequestURL(_ string) string {
	return ""
}

func (c *Client) GenerateDeleteProjectRequestURL(apiAddress, projectName string) string {
	return c.projectRequestURL(apiAddress, projectName)
}

func (c *Client) ShouldWaitForCreateCompletion() bool { return false }

func (c *Client) projectRequestURL(apiAddress, projectName string) string {
	return fmt.Sprintf("%s/%s/%s", apiAddress, "projects", projectName)
}
