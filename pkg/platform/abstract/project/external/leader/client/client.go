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
	"io"
	"net/http"
	"time"

	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/auth/iguazio/v4/serviceaccounttoken"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

const (
	defaultRequestTimeout = 60 * time.Second
)

type Client struct {
	logger                    logger.Logger
	httpClient                *http.Client
	leaderOps                 leader.LeaderOps
	platformConfiguration     *platformconfig.Config
	apiAddress                string
	serviceAccountTokenClient serviceaccounttoken.ServiceAccountTokenClient
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

	serviceAccountTokenClient, err := serviceaccounttoken.NewServiceAccountTokenClient(&platformConfiguration.ServiceAccountConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create service account token client")
	}

	return &Client{
		logger: parentLogger.GetChild("project-leader"),
		httpClient: &http.Client{
			Timeout: defaultRequestTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: skipTLSVerification},
			},
		},
		leaderOps:                 leaderOps,
		platformConfiguration:     platformConfiguration,
		apiAddress:                platformConfiguration.ProjectsLeader.APIAddress,
		serviceAccountTokenClient: serviceAccountTokenClient,
	}, nil
}

// Get retrieves projects from the leader
func (c *Client) Get(ctx context.Context, getProjectOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	projectName := getProjectOptions.Meta.Name
	requestHeaders, cookies, err := c.generateRequestHeadersAndCookies(ctx,
		getProjectOptions.AuthSession,
		getProjectOptions.SessionCookie,
		getProjectOptions.ServiceAccountAuthentication)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to generate request headers and cookies")
	}
	getSingleProject := projectName != ""
	requestURL := c.leaderOps.GenerateGetProjectsRequestURL(c.apiAddress, projectName)

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
	return c.leaderOps.ResolveGetProjectResponse(getSingleProject, responseBody)
}

// Create sends a request to the leader to create a new project
func (c *Client) Create(ctx context.Context, createProjectOptions *platform.CreateProjectOptions) error {
	projectName := createProjectOptions.ProjectConfig.Meta.Name
	projectNamespace := createProjectOptions.ProjectConfig.Meta.Namespace
	requestBody, err := c.leaderOps.GenerateProjectRequestBody(createProjectOptions.ProjectConfig)
	if err != nil {
		return errors.Wrap(err, "Failed to generate project request body")
	}

	requestURL := c.leaderOps.GenerateCreateProjectRequestURL(c.apiAddress)
	requestHeaders, cookies, err := c.generateRequestHeadersAndCookies(ctx,
		createProjectOptions.AuthSession,
		createProjectOptions.SessionCookie,
		createProjectOptions.ServiceAccountAuthentication)
	if err != nil {
		return errors.Wrap(err, "Failed to generate request headers and cookies")
	}

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
		return c.leaderOps.HandleCreateResponseErr(ctx, responseBody, response, err)
	}

	// resolve project
	project, err := c.leaderOps.ResolveCreateProjectResponse(ctx, responseBody)
	if err != nil {
		return errors.Wrap(err, "Failed to resolve project from response body")
	}

	if c.leaderOps.ShouldWaitForCreateCompletion() && createProjectOptions.WaitForCreateCompletion {
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
	requestURL := c.leaderOps.GenerateUpdateProjectRequestURL(c.apiAddress, projectName)
	requestHeaders, cookies, err := c.generateRequestHeadersAndCookies(ctx,
		updateProjectOptions.AuthSession,
		updateProjectOptions.SessionCookie,
		updateProjectOptions.ServiceAccountAuthentication)
	if err != nil {
		return errors.Wrap(err, "Failed to generate request headers and cookies")
	}
	requestBody, err := c.leaderOps.GenerateProjectRequestBody(&updateProjectOptions.ProjectConfig)
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
	requestURL := c.leaderOps.GenerateDeleteProjectRequestURL(c.apiAddress, projectName)
	requestHeaders, cookies, err := c.generateRequestHeadersAndCookies(ctx,
		deleteProjectOptions.AuthSession,
		deleteProjectOptions.SessionCookie,
		deleteProjectOptions.ServiceAccountAuthentication)
	if err != nil {
		return errors.Wrap(err, "Failed to generate request headers and cookies")
	}
	headerName := c.leaderOps.GetDeleteStrategyHeaderName()
	requestHeaders[headerName] = string(deleteProjectOptions.Strategy)

	requestBody, err := c.leaderOps.GenerateProjectDeletionRequestBody(projectName)
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
		c.leaderOps.GetDeleteExpectedStatusCode()); err != nil {
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
	requestURL := c.leaderOps.GenerateGetUpdatedAfterRequestURL(c.apiAddress)
	requestHeaders, _, err := c.generateRequestHeadersAndCookies(ctx, nil, nil, true)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to generate request headers")
	}
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
	return c.leaderOps.ResolveGetProjectResponse(false, responseBody)
}

// EvaluateLeaderRequest determines the 2PC phase from labels and delegates to the
// configured LeaderOps implementation, returning whether the caller should apply the change.
func (c *Client) EvaluateLeaderRequest(ctx context.Context, labels map[string]string, existingProject platform.Project) (bool, error) {
	return c.leaderOps.EvaluateLeaderRequest(ctx, labels, existingProject)
}

// ProjectSync2PCEnabled delegates to the configured LeaderOps so callers can decide
// whether to fetch the existing CRD before invoking EvaluateLeaderRequest.
func (c *Client) ProjectSync2PCEnabled() bool {
	return c.leaderOps.ProjectSync2PCEnabled()
}

// Follower operations delegate to the configured LeaderOps. See leader.LeaderOps for docs.

// PrepareCreateProject2PC delegates to the configured LeaderOps.
func (c *Client) PrepareCreateProject2PC(ctx context.Context,
	options *platform.PrepareCreateProjectOptions) (*platform.Project2PCState, error) {
	return c.leaderOps.PrepareCreateProject2PC(ctx, options)
}

// CommitCreateProject2PC delegates to the configured LeaderOps.
func (c *Client) CommitCreateProject2PC(ctx context.Context,
	options *platform.CommitCreateProjectOptions) (*platform.Project2PCState, error) {
	return c.leaderOps.CommitCreateProject2PC(ctx, options)
}

// UpdateProject2PC delegates to the configured LeaderOps.
func (c *Client) UpdateProject2PC(ctx context.Context,
	options *platform.CommitUpdateProjectOptions) (*platform.Project2PCState, error) {
	return c.leaderOps.UpdateProject2PC(ctx, options)
}

// PrepareDeleteProject2PC delegates to the configured LeaderOps.
func (c *Client) PrepareDeleteProject2PC(ctx context.Context,
	options *platform.PrepareDeleteProjectOptions) (*platform.Project2PCState, error) {
	return c.leaderOps.PrepareDeleteProject2PC(ctx, options)
}

// CommitDeleteProject2PC delegates to the configured LeaderOps.
func (c *Client) CommitDeleteProject2PC(ctx context.Context,
	options *platform.CommitDeleteProjectOptions) (*platform.Project2PCState, error) {
	return c.leaderOps.CommitDeleteProject2PC(ctx, options)
}

// ListProject2PCStates delegates to the configured LeaderOps.
func (c *Client) ListProject2PCStates(ctx context.Context,
	options *platform.ListProjectStatesOptions) (*platform.Project2PCStatesPage, error) {
	return c.leaderOps.ListProject2PCStates(ctx, options)
}

func (c *Client) logLeaderResponseError(ctx context.Context,
	response *http.Response,
	errMessage string) {
	if response == nil {
		c.logger.WarnWithCtx(ctx, "Got an empty response", "errMessage", errMessage)
		return
	}

	// Try to read response body for additional context
	var responseBody string
	if response.Body != nil {
		bodyBytes, err := io.ReadAll(response.Body)
		if err == nil {
			responseBody = string(bodyBytes)
		}
		// Close the body after reading
		response.Body.Close() // nolint: errcheck
	}

	logFields := []interface{}{
		"statusCode", response.StatusCode,
	}
	if responseBody != "" {
		logFields = append(logFields, "responseBody", responseBody)
	}

	if response.StatusCode >= 500 {
		c.logger.WarnWithCtx(ctx, errMessage, logFields...)
		return
	}

	c.logger.DebugWithCtx(ctx, errMessage, logFields...)
}

func (c *Client) generateCommonRequestHeaders(ctx context.Context) map[string]string {
	commonHeaders := map[string]string{
		headers.ProjectsRole: "nuclio",
		"Content-Type":       "application/json",
	}

	if contextID := ctx.Value(middleware.RequestIDKey); contextID != nil {
		commonHeaders[headers.IguazioContext] = contextID.(string)
	}
	return commonHeaders
}

func (c *Client) generateRequestHeadersAndCookies(
	ctx context.Context,
	authSession auth.Session,
	sessionCookie *http.Cookie,
	serviceAccount bool,
) (map[string]string, []*http.Cookie, error) {
	var cookies []*http.Cookie

	requestHeaders := c.generateCommonRequestHeaders(ctx)
	if authSession != nil {
		c.leaderOps.AddAuthSessionHeaders(requestHeaders, authSession)
		if cookie := c.leaderOps.GetAuthSessionCookie(authSession); cookie != nil {
			cookies = append(cookies, cookie)
		}
	}

	// attach session cookie
	if sessionCookie != nil {
		cookies = append(cookies, sessionCookie)
	}

	if serviceAccount {
		// escalate service account auth headers
		err := c.serviceAccountTokenClient.EscalateAuthHeaders(requestHeaders)
		if err != nil {
			return nil, nil, errors.Wrap(err, "Failed to escalate service account token headers")
		}
	}

	return requestHeaders, cookies, nil
}

func (c *Client) waitForJobCompletion(ctx context.Context, jobID, projectName string) error {
	c.logger.DebugWithCtx(ctx, "Waiting for job completion", "jobID", jobID)
	var job leader.JobResponse
	requestCookies := c.leaderOps.GetJobStatusRequestCookies(c.platformConfiguration)

	err := common.RetryUntilSuccessful(time.Minute*5,
		time.Second*5,
		func() bool {
			responseBody, response, err := common.SendHTTPRequestWithContext(ctx,
				c.httpClient,
				http.MethodGet,
				c.leaderOps.GetJobIdUrl(c.apiAddress, jobID),
				nil,
				c.generateCommonRequestHeaders(ctx),
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
			job, isTerminated = c.leaderOps.ParseJobStatusResponse(ctx, responseBody)
			return isTerminated
		})
	if err != nil {
		return errors.Wrap(err, "Exhausting waiting for job completion")
	}

	return c.leaderOps.IsJobCompleted(ctx, job, projectName)
}

func (c *Client) getUpdatedAfter(ctx context.Context,
	requestURL string,
	requestHeaders map[string]string,
	updatedAfterTime *time.Time) ([]byte, error) {
	var requestURLFilterByURL string
	requestCookies := c.leaderOps.GetJobStatusRequestCookies(c.platformConfiguration)
	if updatedAfterTime != nil {
		requestURLFilterByURL = c.leaderOps.GetJobRequestFilter(updatedAfterTime)
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
