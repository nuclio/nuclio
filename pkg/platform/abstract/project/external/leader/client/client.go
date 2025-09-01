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

package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

const (
	defaultRequestTimeout = 60 * time.Second
)

type Client struct {
	logger                logger.Logger
	httpClient            *http.Client
	leader                leader.LeaderOps
	platformConfiguration *platformconfig.Config
	apiAddress            string
}

// NewClient creates a new project leader client for communicating with the external leader service.
func NewClient(parentLogger logger.Logger,
	skipTLSVerification bool,
	platformConfiguration *platformconfig.Config,
	leaderOps leader.LeaderOps,
) (*Client, error) {
	if platformConfiguration.ProjectsLeader == nil {
		return nil, errors.New("Projects leader configuration is missing")
	}

	return &Client{
		logger: parentLogger.GetChild("project-leader"),
		httpClient: &http.Client{
			Timeout: defaultRequestTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: skipTLSVerification},
			},
		},
		leader:                leaderOps,
		platformConfiguration: platformConfiguration,
		apiAddress:            platformConfiguration.ProjectsLeader.APIAddress,
	}, nil
}

// Get retrieves projects from the leader
func (c *Client) Get(ctx context.Context, getProjectOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	projectName := getProjectOptions.Meta.Name
	requestHeaders, cookies := c.generateRequestHeadersAndCookies(getProjectOptions.AuthSession, getProjectOptions.SessionCookie)
	getSingleProject := projectName != ""
	requestURL := c.leader.GenerateGetProjectsRequestURL(c.apiAddress, projectName)

	c.logger.DebugWithCtx(ctx,
		"Fetching projects from leader",
		"getProjectOptionsMeta", getProjectOptions.Meta)
	responseBody, response, err := common.SendHTTPRequestWithContext(ctx,
		c.httpClient,
		http.MethodGet,
		requestURL,
		nil,
		requestHeaders,
		cookies,
		http.StatusOK)
	if err != nil {
		c.logLeaderResponseError(ctx, response, "Failed to get project from leader")
		return nil, errors.Wrap(err, "Failed to send request to leader")
	}
	return c.leader.ResolveGetProjectResponse(getSingleProject, responseBody)
}

// Create sends a request to the leader to create a new project
func (c *Client) Create(ctx context.Context, createProjectOptions *platform.CreateProjectOptions) error {
	projectName := createProjectOptions.ProjectConfig.Meta.Name
	projectNamespace := createProjectOptions.ProjectConfig.Meta.Namespace
	requestBody, err := c.leader.GenerateProjectRequestBody(createProjectOptions.ProjectConfig)
	if err != nil {
		return errors.Wrap(err, "Failed to generate project request body")
	}

	requestURL := c.leader.GenerateCreateProjectRequestURL(c.apiAddress)
	requestHeaders, cookies := c.generateRequestHeadersAndCookies(createProjectOptions.AuthSession, createProjectOptions.SessionCookie)

	c.logger.DebugWithCtx(ctx,
		"Sending create project request to leader",
		"name", projectName,
		"namespace", projectNamespace)
	responseBody, response, err := common.SendHTTPRequestWithContext(ctx,
		c.httpClient,
		http.MethodPost,
		requestURL,
		requestBody,
		requestHeaders,
		cookies,
		http.StatusCreated)
	if err != nil {
		c.logLeaderResponseError(ctx, response, "Failed to create project on leader")
		return c.leader.HandleCreateResponseErr(ctx, responseBody, response, err)
	}

	// resolve project
	project, err := c.leader.ResolveCreateProjectResponse(ctx, responseBody)
	if err != nil {
		return errors.Wrap(err, "Failed to resolve project from response body")
	}

	if c.leader.ShouldWaitForCreateCompletion() && createProjectOptions.WaitForCreateCompletion {
		if err = c.waitForJobCompletion(ctx, project.GetLastJobID(), projectName); err != nil {
			return errors.Wrap(err, "Failed waiting for create project job completion")
		}
	}

	c.logger.DebugWithCtx(ctx,
		"Successfully sent create project request to leader",
		"name", projectName,
		"namespace", projectNamespace)
	return nil
}

// Update sends a request to the leader to update an existing project
func (c *Client) Update(ctx context.Context, updateProjectOptions *platform.UpdateProjectOptions) error {
	projectName := updateProjectOptions.ProjectConfig.Meta.Name
	projectNamespace := updateProjectOptions.ProjectConfig.Meta.Namespace
	requestURL := c.leader.GenerateUpdateProjectRequestURL(c.apiAddress, projectName)
	requestHeaders, cookies := c.generateRequestHeadersAndCookies(updateProjectOptions.AuthSession, updateProjectOptions.SessionCookie)
	requestBody, err := c.leader.GenerateProjectRequestBody(&updateProjectOptions.ProjectConfig)
	if err != nil {
		return errors.Wrap(err, "Failed to generate project request body")
	}

	c.logger.DebugWithCtx(ctx,
		"Sending update project request to leader",
		"name", projectName,
		"namespace", projectNamespace)
	responseBody, response, err := common.SendHTTPRequestWithContext(ctx,
		c.httpClient,
		http.MethodPut,
		requestURL,
		requestBody,
		requestHeaders,
		cookies,
		http.StatusOK)
	if err != nil {
		c.logLeaderResponseError(ctx, response, "Failed to update project on leader")
		return errors.Wrap(err, "Failed to send update project request to leader")
	}

	c.logger.DebugWithCtx(ctx,
		"Successfully sent update project request to leader",
		"name", projectName,
		"namespace", projectNamespace,
		"responseBody", string(responseBody))
	return nil
}

// Delete sends a request to the leader to delete a project
func (c *Client) Delete(ctx context.Context, deleteProjectOptions *platform.DeleteProjectOptions) error {
	projectName := deleteProjectOptions.Meta.Name
	projectNamespace := deleteProjectOptions.Meta.Namespace
	requestURL := c.leader.GenerateDeleteProjectRequestURL(c.apiAddress, projectName)
	requestHeaders, cookies := c.generateRequestHeadersAndCookies(deleteProjectOptions.AuthSession, deleteProjectOptions.SessionCookie)
	headerName := c.leader.GetDeleteStrategyHeaderName()
	requestHeaders[headerName] = string(deleteProjectOptions.Strategy)

	requestBody, err := c.leader.GenerateProjectDeletionRequestBody(projectName)
	if err != nil {
		return errors.Wrap(err, "Failed to generate project deletion request body")
	}

	c.logger.DebugWithCtx(ctx,
		"Sending delete project request to leader",
		"name", projectName,
		"namespace", projectNamespace)
	if _, response, err := common.SendHTTPRequestWithContext(ctx,
		c.httpClient,
		http.MethodDelete,
		requestURL,
		requestBody,
		requestHeaders,
		cookies,
		c.leader.GetDeleteExpectedStatusCode()); err != nil {
		c.logLeaderResponseError(ctx, response, "Failed to delete project on leader")
		return errors.Wrap(err, "Failed to send delete project request to leader")
	}

	c.logger.DebugWithCtx(ctx,
		"Successfully sent delete project request to leader",
		"name", projectName,
		"namespace", projectNamespace)
	return nil
}

// GetUpdatedAfter retrieves projects from the leader that were updated after the specified time.
func (c *Client) GetUpdatedAfter(ctx context.Context, updatedAfterTime *time.Time) ([]platform.Project, error) {
	requestURL := c.leader.GenerateGetUpdatedAfterRequestURL(c.apiAddress)
	requestHeaders := c.generateCommonRequestHeaders()
	if updatedAfterTime != nil && updatedAfterTime.IsZero() {
		updatedAfterTime = nil
	}

	responseBody, err := c.getUpdatedAfter(ctx, requestURL, requestHeaders, updatedAfterTime)
	if err != nil {
		c.logger.DebugWithCtx(ctx,
			"Retrying with no update-at",
			"updatedAfterTime", updatedAfterTime,
			"err", err.Error())
		responseBody, err = c.getUpdatedAfter(ctx, requestURL, requestHeaders, nil)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get projects from leader")
		}
	}
	return c.leader.ResolveGetProjectResponse(false, responseBody)
}

func (c *Client) logLeaderResponseError(ctx context.Context,
	response *http.Response,
	errMessage string) {
	if response == nil {
		c.logger.WarnWithCtx(ctx, "Got an empty response", "errMessage", errMessage)
		return
	}
	if response.StatusCode >= 500 {
		c.logger.WarnWithCtx(ctx,
			errMessage,
			"statusCode", response.StatusCode,
			"response", response,
		)
		return
	}

	c.logger.DebugWithCtx(ctx,
		errMessage,
		"statusCode", response.StatusCode,
		"response", response,
	)
}

func (c *Client) generateCommonRequestHeaders() map[string]string {
	return map[string]string{
		headers.ProjectsRole: "nuclio",
		"Content-Type":       "application/json",
	}
}

func (c *Client) generateRequestHeadersAndCookies(
	authSession auth.Session,
	sessionCookie *http.Cookie,
) (map[string]string, []*http.Cookie) {
	var cookies []*http.Cookie

	requestHeaders := c.generateCommonRequestHeaders()
	if authSession != nil {
		requestHeaders["authorization"] = authSession.CompileAuthorizationBasicHeader()
		cookies = append(cookies, &http.Cookie{
			Name:  "session",
			Value: url.QueryEscape(fmt.Sprintf(`j:{"sid":"%s"}`, authSession.GetPassword())),
		})
	}

	// attach session cookie
	if sessionCookie != nil {
		cookies = append(cookies, sessionCookie)
	}

	return requestHeaders, cookies
}

func (c *Client) waitForJobCompletion(ctx context.Context, jobID, projectName string) error {
	c.logger.DebugWithCtx(ctx, "Waiting for job completion", "jobID", jobID)
	var job leader.JobResponse
	requestCookies := c.leader.GetJobStatusRequestCookies(c.platformConfiguration)

	err := common.RetryUntilSuccessful(time.Minute*5,
		time.Second*5,
		func() bool {
			responseBody, response, err := common.SendHTTPRequestWithContext(ctx,
				c.httpClient,
				http.MethodGet,
				c.leader.GetJobIdUrl(c.apiAddress, jobID),
				nil,
				c.generateCommonRequestHeaders(),
				requestCookies,
				http.StatusOK)
			if err != nil {
				c.logLeaderResponseError(ctx, response, "Failed to get job status")
				c.logger.DebugWithCtx(ctx,
					"Failed to get job state",
					"responseBody", string(responseBody))
				return false
			}

			var isTerminated bool
			job, isTerminated = c.leader.ParseJobStatusResponse(ctx, responseBody)
			return isTerminated
		})
	if err != nil {
		return errors.Wrap(err, "Exhausting waiting for job completion")
	}

	return c.leader.IsJobCompleted(ctx, job, projectName)
}

func (c *Client) getUpdatedAfter(ctx context.Context,
	requestURL string,
	requestHeaders map[string]string,
	updatedAfterTime *time.Time) ([]byte, error) {
	var requestURLFilterByURL string
	requestCookies := c.leader.GetJobStatusRequestCookies(c.platformConfiguration)
	if updatedAfterTime != nil {
		requestURLFilterByURL = c.leader.GetJobRequestFilter(updatedAfterTime)
	}

	responseBody, response, err := common.SendHTTPRequestWithContext(ctx,
		c.httpClient,
		http.MethodGet,
		requestURL+requestURLFilterByURL,
		nil,
		requestHeaders,
		requestCookies,
		http.StatusOK)
	if err != nil {
		c.logLeaderResponseError(ctx, response, "Failed to get updated after from leader")
		return nil, errors.Wrap(err, "Failed to send request to leader")
	}
	return responseBody, nil
}
