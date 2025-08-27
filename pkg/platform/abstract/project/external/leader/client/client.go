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

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

const (
	defaultRequestTimeout = 60 * time.Second
)

type Client struct {
	logger              logger.Logger
	httpClient          *http.Client
	SkipTLSVerification bool
}

func NewClient(parentLogger logger.Logger,
	skipTLSVerification bool,
) *Client {
	return &Client{
		logger:              parentLogger,
		SkipTLSVerification: skipTLSVerification,
		httpClient: &http.Client{
			Timeout: defaultRequestTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: skipTLSVerification},
			},
		},
	}
}

func (c *Client) LogLeaderInternalServerResponseError(ctx context.Context,
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
	}
}

func (c *Client) GetHTTPClient() *http.Client {
	return c.httpClient
}

func (c *Client) CreateProject(ctx context.Context,
	createProjectOptions *platform.CreateProjectOptions,
	requestBody []byte,
	requestURL string,
) ([]byte, *http.Response, error) {
	c.logger.DebugWithCtx(ctx,
		"Sending create project request to leader",
		"name", createProjectOptions.ProjectConfig.Meta.Name,
		"namespace", createProjectOptions.ProjectConfig.Meta.Namespace)

	requestHeaders, cookies := c.generateRequestHeadersAndCookies(createProjectOptions.AuthSession, createProjectOptions.SessionCookie)

	// send the request
	c.logger.DebugWithCtx(ctx,
		"Creating project request to leader",
		"body", string(requestBody))
	return common.SendHTTPRequestWithContext(ctx,
		c.httpClient,
		http.MethodPost,
		requestURL,
		requestBody,
		requestHeaders,
		cookies,
		http.StatusCreated)
}

func (c *Client) UpdateProject(ctx context.Context,
	updateProjectOptions *platform.UpdateProjectOptions,
	requestBody []byte,
	requestURL string,
) error {
	updateProjectName := updateProjectOptions.ProjectConfig.Meta.Name
	updateProjectNamespace := updateProjectOptions.ProjectConfig.Meta.Namespace

	c.logger.DebugWithCtx(ctx,
		"Sending update project request to leader",
		"name", updateProjectName,
		"namespace", updateProjectNamespace)

	requestHeaders, cookies := c.generateRequestHeadersAndCookies(updateProjectOptions.AuthSession, updateProjectOptions.SessionCookie)

	// send the request
	responseBody, response, err := common.SendHTTPRequestWithContext(ctx,
		c.httpClient,
		http.MethodPut,
		requestURL,
		requestBody,
		requestHeaders,
		cookies,
		http.StatusOK)
	if err != nil {
		c.LogLeaderInternalServerResponseError(ctx, response, "Failed to update project on leader")
		return errors.Wrap(err, "Failed to send update project request to leader")
	}

	c.logger.DebugWithCtx(ctx,
		"Successfully sent update project request to leader",
		"name", updateProjectName,
		"namespace", updateProjectNamespace,
		"responseBody", string(responseBody))
	return nil
}

func (c *Client) DeleteProject(ctx context.Context,
	deleteProjectOptions *platform.DeleteProjectOptions,
	requestBody []byte,
	requestURL string,
	expectedStatusCode int,
	isIgzProject bool,
) error {
	deleteProjectName := deleteProjectOptions.Meta.Name
	deleteProjectNamespace := deleteProjectOptions.Meta.Namespace

	c.logger.DebugWithCtx(ctx,
		"Sending delete project request to leader",
		"name", deleteProjectName,
		"namespace", deleteProjectNamespace)

	requestHeaders, cookies := c.generateRequestHeadersAndCookies(deleteProjectOptions.AuthSession, deleteProjectOptions.SessionCookie)
	if isIgzProject {
		requestHeaders["igz-project-deletion-strategy"] = string(deleteProjectOptions.Strategy)
	}

	if _, response, err := common.SendHTTPRequestWithContext(ctx,
		c.httpClient,
		http.MethodDelete,
		requestURL,
		requestBody,
		requestHeaders,
		cookies,
		expectedStatusCode); err != nil {
		c.LogLeaderInternalServerResponseError(ctx, response, "Failed to delete project on leader")
		return errors.Wrap(err, "Failed to send delete project request to leader")
	}

	c.logger.DebugWithCtx(ctx,
		"Successfully sent delete project request to leader",
		"name", deleteProjectName,
		"namespace", deleteProjectNamespace)
	return nil
}

func (c *Client) GenerateCommonRequestHeaders() map[string]string {
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

	requestHeaders := c.GenerateCommonRequestHeaders()
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
