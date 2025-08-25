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

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader/httpclient"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

type Client struct {
	logger                logger.Logger
	platformConfiguration *platformconfig.Config
	httpClient            *httpclient.Client
}

func NewClient(parentLogger logger.Logger, platformConfiguration *platformconfig.Config) (*Client, error) {
	// skip TLS verification for mlrun
	skipTLSVerification := platformConfiguration.ProjectsLeader.Kind == platformconfig.ProjectsLeaderKindMlrun
	clientLogger := parentLogger.GetChild("leader-client-mlrun")

	newClient := Client{
		logger:                clientLogger,
		platformConfiguration: platformConfiguration,
		httpClient:            httpclient.NewClient(clientLogger, skipTLSVerification),
	}

	return &newClient, nil
}

func (c *Client) Get(ctx context.Context, getProjectOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	return nil, nuclio.ErrNotImplemented
}

func (c *Client) Create(ctx context.Context, createProjectOptions *platform.CreateProjectOptions) error {
	body, err := c.generateProjectRequestBody(createProjectOptions.ProjectConfig)
	if err != nil {
		return errors.Wrap(err, "Failed to generate project request body")
	}

	requestURL := fmt.Sprintf("%s/%s", c.platformConfiguration.ProjectsLeader.APIAddress, "projects")
	responseBody, response, err := c.httpClient.CreateProject(ctx,
		createProjectOptions,
		body,
		requestURL)
	if err != nil {
		c.httpClient.LogLeaderInternalServerResponseError(ctx, response, "Failed to create project on leader")
		// Try to parse MLRun error response
		var mlrunError MlrunError

		// try peek at error response
		if unmarshalErr := json.Unmarshal(responseBody, &mlrunError); unmarshalErr == nil {
			if response == nil {
				return errors.New("Failed to get response from leader, response is nil")
			}
			return nuclio.GetByStatusCode(response.StatusCode)(mlrunError.Detail)
		}
		return errors.Wrap(err, "Failed to send request to leader")
	}

	// resolve project
	project, err := c.resolveCreateProjectResponse(responseBody)
	if err != nil {
		return errors.Wrap(err, "Failed to resolve project from response body")
	}

	c.logger.DebugWithCtx(ctx,
		"Successfully sent create project request to leader",
		"project name", project.Metadata.Name)
	return nil
}

func (c *Client) Update(ctx context.Context, updateProjectOptions *platform.UpdateProjectOptions) error {
	body, err := c.generateProjectRequestBody(&updateProjectOptions.ProjectConfig)
	if err != nil {
		return errors.Wrap(err, "Failed to generate project request body")
	}

	requestURL := fmt.Sprintf("%s/%s/%s",
		c.platformConfiguration.ProjectsLeader.APIAddress,
		"projects",
		updateProjectOptions.ProjectConfig.Meta.Name)

	return c.httpClient.UpdateProject(ctx, updateProjectOptions, body, requestURL)
}

func (c *Client) Delete(ctx context.Context, deleteProjectOptions *platform.DeleteProjectOptions) error {
	body, err := c.generateProjectDeletionRequestBody(deleteProjectOptions.Meta.Name)
	if err != nil {
		return errors.Wrap(err, "Failed to generate project deletion request body")
	}

	requestURL := fmt.Sprintf("%s/%s/%s",
		c.platformConfiguration.ProjectsLeader.APIAddress,
		"projects",
		deleteProjectOptions.Meta.Name)

	return c.httpClient.DeleteProject(ctx, deleteProjectOptions, body, requestURL, http.StatusNoContent, false)
}

func (c *Client) GetUpdatedAfter(ctx context.Context, updatedAfterTime *time.Time) ([]platform.Project, error) {
	return nil, nuclio.ErrNotImplemented
}

func (c *Client) generateProjectRequestBody(projectConfig *platform.ProjectConfig) ([]byte, error) {
	project, err := NewProjectFromProjectConfig(projectConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create project from project config")
	}
	return json.Marshal(project)
}

func (c *Client) generateProjectDeletionRequestBody(projectName string) ([]byte, error) {
	return json.Marshal(Project{
		Metadata: ProjectMetadata{
			Name: projectName,
		},
	})
}

func (c *Client) resolveCreateProjectResponse(body []byte) (*Project, error) {
	project := Project{}
	if err := json.Unmarshal(body, &project); err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal response body")
	}

	return &project, nil
}
