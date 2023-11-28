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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

const (
	ProjectsRoleHeaderValueNuclio = "nuclio"
	DefaultRequestTimeout         = 60 * time.Second

	// ProjectsRoleHeaderKey not prefixed with "x-nuclio.." this header is used across Iguazio components
	ProjectsRoleHeaderKey = "x-projects-role"
)

type Client struct {
	logger                logger.Logger
	platformConfiguration *platformconfig.Config
	httpClient            *http.Client
}

func NewClient(parentLogger logger.Logger, platformConfiguration *platformconfig.Config) (*Client, error) {
	newClient := Client{
		logger:                parentLogger.GetChild("leader-client-iguazio"),
		platformConfiguration: platformConfiguration,
		httpClient: &http.Client{
			Timeout: DefaultRequestTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}

	return &newClient, nil
}

func (c *Client) Get(ctx context.Context, getProjectOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	c.logger.DebugWithCtx(ctx,
		"Fetching projects from leader",
		"getProjectOptionsMeta", getProjectOptions.Meta)

	headers := c.generateCommonRequestHeaders()
	var cookies []*http.Cookie
	if getProjectOptions.SessionCookie != nil {
		cookies = append(cookies, getProjectOptions.SessionCookie)
	}

	if getProjectOptions.AuthSession != nil {
		headers["authorization"] = getProjectOptions.AuthSession.CompileAuthorizationBasic()
		cookies = append(cookies, &http.Cookie{
			Name:  "session",
			Value: url.QueryEscape(fmt.Sprintf(`j:{"sid":"%s"}`, getProjectOptions.AuthSession.GetPassword())),
		})
	}

	getSingleProject := getProjectOptions.Meta.Name != ""

	requestURL := fmt.Sprintf("%s/%s", c.platformConfiguration.ProjectsLeader.APIAddress, "projects")
	if getSingleProject {
		requestURL += fmt.Sprintf("/__name__/%s", getProjectOptions.Meta.Name)
	}

	// include namespace and username
	requestURL += "?include=owner&enrich_namespace=true"

	// send the request
	responseBody, response, err := common.SendHTTPRequestWithContext(ctx,
		c.httpClient,
		http.MethodGet,
		requestURL,
		nil,
		headers,
		cookies,
		http.StatusOK)
	if err != nil {
		c.logLeaderInternalServerResponseError(ctx, response, "Failed to get project from leader")
		return nil, errors.Wrap(err, "Failed to send request to leader")
	}

	return c.resolveGetProjectResponse(getSingleProject, responseBody)
}

func (c *Client) Create(ctx context.Context, createProjectOptions *platform.CreateProjectOptions) error {
	var cookies []*http.Cookie

	c.logger.DebugWithCtx(ctx,
		"Sending create project request to leader",
		"name", createProjectOptions.ProjectConfig.Meta.Name,
		"namespace", createProjectOptions.ProjectConfig.Meta.Namespace)

	headers := c.generateCommonRequestHeaders()
	if createProjectOptions.AuthSession != nil {
		headers["authorization"] = createProjectOptions.AuthSession.CompileAuthorizationBasic()
		cookies = append(cookies, &http.Cookie{
			Name:  "session",
			Value: url.QueryEscape(fmt.Sprintf(`j:{"sid":"%s"}`, createProjectOptions.AuthSession.GetPassword())),
		})
	}

	// generate request body
	body, err := c.generateProjectRequestBody(createProjectOptions.ProjectConfig)
	if err != nil {
		return errors.Wrap(err, "Failed to generate project request body")
	}

	// attach session cookie
	if createProjectOptions.SessionCookie != nil {
		cookies = append(cookies, createProjectOptions.SessionCookie)
	}

	// send the request
	c.logger.DebugWithCtx(ctx,
		"Creating project",
		"body", string(body))
	responseBody, response, err := common.SendHTTPRequestWithContext(ctx,
		c.httpClient,
		http.MethodPost,
		fmt.Sprintf("%s/%s", c.platformConfiguration.ProjectsLeader.APIAddress, "projects"),
		body,
		headers,
		cookies,
		http.StatusCreated)
	if err != nil {
		c.logLeaderInternalServerResponseError(ctx, response, "Failed to create project on leader")
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

	// resolve project
	project, err := c.resolveCreateProjectResponse(responseBody)
	if err != nil {
		return errors.Wrap(err, "Failed to resolve project from response body")
	}

	c.logger.DebugWithCtx(ctx,
		"Successfully sent create project request to leader",
		"igzCtx", project.Meta.Ctx,
		"projectData", project.Data)

	if createProjectOptions.WaitForCreateCompletion {
		job, err := c.waitForJobCompletion(ctx, project.Data.Relationships.LastJob.Data.ID)
		if err != nil {
			return errors.Wrap(err, "Failed waiting for create project job completion")
		}

		if job.Data.Attributes.State != JobStateCompleted {
			var jobResult struct {
				ProjectID string `json:"project_id,omitempty"`
				Status    int    `json:"status,omitempty"`
				Message   string `json:"message,omitempty"`
			}

			// try peek at job results to see if it has a meaningful error message
			if err := json.Unmarshal([]byte(job.Data.Attributes.Result), &jobResult); err == nil {
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
				job.Data.Attributes.State)
		}
		c.logger.DebugWithCtx(ctx, "Successfully created project",
			"projectName", project.Data.Attributes.Name,
			"projectJobCreationCtx", job.Meta.Ctx)
	}
	return nil
}

func (c *Client) Update(ctx context.Context, updateProjectOptions *platform.UpdateProjectOptions) error {
	var cookies []*http.Cookie

	c.logger.DebugWithCtx(ctx,
		"Sending update project request to leader",
		"name", updateProjectOptions.ProjectConfig.Meta.Name,
		"namespace", updateProjectOptions.ProjectConfig.Meta.Namespace)

	headers := c.generateCommonRequestHeaders()
	if updateProjectOptions.AuthSession != nil {
		headers["authorization"] = updateProjectOptions.AuthSession.CompileAuthorizationBasic()
		cookies = append(cookies, &http.Cookie{
			Name:  "session",
			Value: url.QueryEscape(fmt.Sprintf(`j:{"sid":"%s"}`, updateProjectOptions.AuthSession.GetPassword())),
		})
	}

	// generate request body
	body, err := c.generateProjectRequestBody(&updateProjectOptions.ProjectConfig)
	if err != nil {
		return errors.Wrap(err, "Failed to generate project request body")
	}

	// attach session cookie
	if updateProjectOptions.SessionCookie != nil {
		cookies = append(cookies, updateProjectOptions.SessionCookie)
	}

	// send the request
	responseBody, response, err := common.SendHTTPRequestWithContext(ctx,
		c.httpClient,
		http.MethodPut,
		fmt.Sprintf("%s/%s/%s",
			c.platformConfiguration.ProjectsLeader.APIAddress,
			"projects/__name__",
			updateProjectOptions.ProjectConfig.Meta.Name),
		body,
		headers,
		cookies,
		http.StatusOK)
	if err != nil {
		c.logLeaderInternalServerResponseError(ctx, response, "Failed to update project on leader")
		return errors.Wrap(err, "Failed to send request to leader")
	}

	c.logger.DebugWithCtx(ctx,
		"Successfully sent update project request to leader",
		"name", updateProjectOptions.ProjectConfig.Meta.Name,
		"namespace", updateProjectOptions.ProjectConfig.Meta.Namespace,
		"responseBody", string(responseBody))

	return nil
}

func (c *Client) Delete(ctx context.Context, deleteProjectOptions *platform.DeleteProjectOptions) error {
	var cookies []*http.Cookie

	c.logger.DebugWithCtx(ctx,
		"Sending delete project request to leader",
		"name", deleteProjectOptions.Meta.Name)

	// send the request
	headers := c.generateCommonRequestHeaders()
	if deleteProjectOptions.AuthSession != nil {
		headers["authorization"] = deleteProjectOptions.AuthSession.CompileAuthorizationBasic()
		cookies = append(cookies, &http.Cookie{
			Name:  "session",
			Value: url.QueryEscape(fmt.Sprintf(`j:{"sid":"%s"}`, deleteProjectOptions.AuthSession.GetPassword())),
		})
	}
	headers["igz-project-deletion-strategy"] = string(deleteProjectOptions.Strategy)

	// generate request body
	body, err := c.generateProjectDeletionRequestBody(deleteProjectOptions.Meta.Name)
	if err != nil {
		return errors.Wrap(err, "Failed to generate project deletion request body")
	}

	// attach session cookie
	if deleteProjectOptions.SessionCookie != nil {
		cookies = append(cookies, deleteProjectOptions.SessionCookie)
	}

	if _, response, err := common.SendHTTPRequestWithContext(ctx,
		c.httpClient,
		http.MethodDelete,
		fmt.Sprintf("%s/%s", c.platformConfiguration.ProjectsLeader.APIAddress, "projects"),
		body,
		headers,
		cookies,
		http.StatusAccepted); err != nil {
		c.logLeaderInternalServerResponseError(ctx, response, "Failed to delete project on leader")
		return errors.Wrap(err, "Failed to send request to leader")
	}

	c.logger.DebugWithCtx(ctx,
		"Successfully sent delete project request to leader",
		"name", deleteProjectOptions.Meta.Name,
		"namespace", deleteProjectOptions.Meta.Namespace)

	return nil
}

func (c *Client) GetUpdatedAfter(ctx context.Context, updatedAfterTime *time.Time) ([]platform.Project, error) {
	requestURL := fmt.Sprintf("%s/%s", c.platformConfiguration.ProjectsLeader.APIAddress, "projects")
	requestURL += "?include=owner&enrich_namespace=true"
	if updatedAfterTime != nil && updatedAfterTime.IsZero() {
		updatedAfterTime = nil
	}

	responseBody, err := c.getUpdatedAfter(ctx, requestURL, updatedAfterTime)
	if err != nil {
		c.logger.DebugWithCtx(ctx,

			"Retrying with no update-at",
			"updatedAfterTime", updatedAfterTime,
			"err", err.Error())
		responseBody, err = c.getUpdatedAfter(ctx, requestURL, nil)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get projects from leader")
		}
	}

	return c.resolveGetProjectResponse(false, responseBody)
}

func (c *Client) getUpdatedAfter(ctx context.Context,
	requestURL string,
	updatedAfterTime *time.Time) ([]byte, error) {
	var requestURLFilterByURL string
	if updatedAfterTime != nil {
		requestURLFilterByURL = fmt.Sprintf("&filter[updated_at]=[$gt]%s",
			updatedAfterTime.Format(time.RFC3339Nano))
	}

	// send the request
	headers := c.generateCommonRequestHeaders()
	responseBody, response, err := common.SendHTTPRequestWithContext(ctx,
		c.httpClient,
		http.MethodGet,
		requestURL+requestURLFilterByURL,
		nil,
		headers,
		[]*http.Cookie{{Name: "session", Value: c.platformConfiguration.IguazioSessionCookie}},
		http.StatusOK)
	if err != nil {
		c.logLeaderInternalServerResponseError(ctx, response, "Failed to get updated after from leader")
		return nil, errors.Wrap(err, "Failed to send request to leader")
	}
	return responseBody, nil
}

func (c *Client) generateCommonRequestHeaders() map[string]string {
	return map[string]string{
		ProjectsRoleHeaderKey: ProjectsRoleHeaderValueNuclio,
		"Content-Type":        "application/json",
	}
}

func (c *Client) generateProjectRequestBody(projectConfig *platform.ProjectConfig) ([]byte, error) {
	project := NewProjectFromProjectConfig(projectConfig)
	c.enrichProjectWithNuclioFields(&project)
	return json.Marshal(project)
}

func (c *Client) generateProjectDeletionRequestBody(projectName string) ([]byte, error) {
	return json.Marshal(Project{
		Data: ProjectData{
			Type: ProjectType,
			Attributes: ProjectAttributes{
				Name: projectName,
			},
		},
	})
}

func (c *Client) waitForJobCompletion(ctx context.Context, jobID string) (*JobDetailResponse, error) {

	// send the request
	headers := c.generateCommonRequestHeaders()
	var job JobDetailResponse

	c.logger.DebugWithCtx(ctx, "Waiting for job completion", "jobID", jobID)
	err := common.RetryUntilSuccessful(time.Minute*5,
		time.Second*5,
		func() bool {
			responseBody, response, err := common.SendHTTPRequestWithContext(ctx,
				c.httpClient,
				http.MethodGet,
				fmt.Sprintf("%s/%s/%s",
					c.platformConfiguration.ProjectsLeader.APIAddress,
					"jobs",
					jobID),
				nil,
				headers,
				[]*http.Cookie{{Name: "session", Value: c.platformConfiguration.IguazioSessionCookie}},
				http.StatusOK)
			if err != nil {
				c.logLeaderInternalServerResponseError(ctx, response, "Failed to get job status")
				c.logger.DebugWithCtx(ctx,
					"Failed to get job state",
					"responseBody", string(responseBody))
				return false
			}

			if err := json.Unmarshal(responseBody, &job); err != nil {
				c.logger.DebugWithCtx(ctx, "Failed to unmarshal response body",
					"responseBody", responseBody)
				return false
			}

			c.logger.DebugWithCtx(ctx,
				"Inspecting job state",
				"jobId", jobID,
				"igzCtx", job.Meta.Ctx,
				"jobAttributes", job.Data.Attributes)
			return JobStateInSlice(job.Data.Attributes.State, []JobState{
				JobStateCompleted,
				JobStateCanceled,
				JobStateFailed,
			})
		})
	if err != nil {
		return nil, errors.Wrap(err, "Exhausting waiting for job completion")
	}

	c.logger.DebugWithCtx(ctx,
		"Completed waiting for job completion",
		"igzCtx", job.Meta.Ctx,
		"jobAttributes", job.Data.Attributes,
		"jobID", jobID)
	return &job, nil
}

func (c *Client) enrichProjectWithNuclioFields(project *Project) {

	// TODO: update this function when nuclio fields are added
	//project.Data.Attributes.NuclioProject = NuclioProject{}
}

func (c *Client) resolveCreateProjectResponse(body []byte) (*ProjectDetailResponse, error) {
	project := ProjectDetailResponse{}
	if err := json.Unmarshal(body, &project); err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal response body")
	}

	return &project, nil
}

func (c *Client) resolveGetProjectResponse(detail bool, body []byte) ([]platform.Project, error) {

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

func (c *Client) logLeaderInternalServerResponseError(ctx context.Context,
	response *http.Response,
	errMessage string) {
	if response == nil {
		c.logger.WarnWithCtx(ctx, "Failed to get response", "errMessage", errMessage)
		return
	}
	if response != nil && response.StatusCode >= 500 {
		c.logger.WarnWithCtx(ctx,
			errMessage,
			"statusCode", response.StatusCode,
		)
	}
}
