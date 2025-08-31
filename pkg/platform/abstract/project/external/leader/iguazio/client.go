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

	"github.com/nuclio/nuclio/pkg/platform"
	leadercommon "github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

type Client struct {
	logger logger.Logger
}

func NewClient(parentLogger logger.Logger) *Client {
	return &Client{
		logger: parentLogger.GetChild("iguazio"),
	}
}

func (c *Client) GenerateProjectRequestBody(projectConfig *platform.ProjectConfig) ([]byte, error) {
	if projectConfig == nil {
		return nil, errors.New("Project config is missing")
	}

	project := NewProjectFromProjectConfig(projectConfig)
	return json.Marshal(project)
}

func (c *Client) GenerateProjectDeletionRequestBody(projectName string) ([]byte, error) {
	return json.Marshal(IguazioProject{
		Data: ProjectData{
			Type: ProjectType,
			Attributes: ProjectAttributes{
				Name: projectName,
			},
		},
	})
}

func (c *Client) ResolveCreateProjectResponse(ctx context.Context, body []byte) (leadercommon.CreateProjectResponse, error) {
	project := ProjectDetailResponse{}
	if err := json.Unmarshal(body, &project); err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal response body")
	}

	c.logger.DebugWithCtx(ctx,
		"Successfully sent create project request to leader",
		"igzCtx", project.Meta.Ctx,
		"projectData", project.Data)

	return &project, nil
}

func (c *Client) ResolveGetProjectResponse(detail bool, body []byte) ([]platform.Project, error) {

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

func (c *Client) IsJobTerminated(ctx context.Context, responseBody []byte) (leadercommon.JobResponse, bool) {
	var job JobDetailResponse
	if err := json.Unmarshal(responseBody, &job); err != nil {
		c.logger.DebugWithCtx(ctx, "Failed to unmarshal response body",
			"responseBody", responseBody)
		return nil, false
	}

	c.logger.DebugWithCtx(ctx,
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

func (c *Client) GenerateCreateProjectRequestURL(apiAddress string) string {
	return c.projectRequestURL(apiAddress)
}

func (c *Client) HandleCreateResponseErr(ctx context.Context, responseBody []byte, _ *http.Response, err error) error {
	var responseError CreateProjectErrorResponse

	// try peek at error response
	if unmarshalErr := json.Unmarshal(responseBody, &responseError); unmarshalErr == nil {
		c.logger.ErrorWithCtx(ctx,
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

func (c *Client) GetJobIdUrl(apiAddress, jobID string) string {
	return fmt.Sprintf("%s/%s/%s", apiAddress, "jobs", jobID)
}

func (c *Client) ValidateJobState(ctx context.Context, job leadercommon.JobResponse, projectName string) error {
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
			c.logger.ErrorWithCtx(ctx, "Create project has failed", "jobResult", jobResult)

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
	c.logger.DebugWithCtx(ctx, "Successfully created project",
		"projectName", projectName,
		"projectJobCreationCtx", job.GetJobCreationCtx())

	return nil
}

func (c *Client) GenerateUpdateProjectRequestURL(apiAddress, projectName string) string {
	return fmt.Sprintf("%s/%s/%s", apiAddress, "projects/__name__", projectName)
}

func (c *Client) GetDeleteExpectedStatusCode() int {
	return http.StatusAccepted
}

func (c *Client) AddDeleteStrategyHeader(headers map[string]string, strategy platform.DeleteProjectStrategy) {
	headers["igz-project-deletion-strategy"] = string(strategy)
}

func (c *Client) GenerateGetProjectsRequestURL(apiAddress, projectName string) string {
	requestURL := c.projectRequestURL(apiAddress)
	if projectName != "" {
		requestURL += fmt.Sprintf("/__name__/%s", projectName)
	}

	// include namespace and username
	requestURL += "?include=owner&enrich_namespace=true"
	return requestURL
}

func (c *Client) GenerateGetUpdatedAfterRequestURL(apiAddress string) string {
	requestURL := c.projectRequestURL(apiAddress)
	requestURL += "?include=owner&enrich_namespace=true"
	return requestURL
}

func (c *Client) GenerateDeleteProjectRequestURL(apiAddress string, _ string) string {
	return c.projectRequestURL(apiAddress)
}

func (c *Client) ShouldWaitForCreateCompletion() bool { return true }

func (c *Client) projectRequestURL(apiAddress string) string {
	return fmt.Sprintf("%s/%s", apiAddress, "projects")
}
