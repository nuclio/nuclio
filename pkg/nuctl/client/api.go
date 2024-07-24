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

package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuctlcommon "github.com/nuclio/nuclio/pkg/nuctl/command/common"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"k8s.io/apimachinery/pkg/labels"
)

const DefaultRequestTimeout = "5m"

type NuclioAPIClient struct {
	logger         logger.Logger
	httpClient     *http.Client
	apiURL         string
	requestTimeout string
	username       string
	accessKey      string
	authHeaders    map[string]string
}

func NewNuclioAPIClient(parentLogger logger.Logger,
	apiURL string,
	requestTimeout string,
	username string,
	accessKey string,
	skipTLSVerify bool) (*NuclioAPIClient, error) {

	// validate and enrich credentials
	if username == "" {
		username = common.GetEnvOrDefaultString("NUCLIO_USERNAME", "")
	}
	if accessKey == "" {
		accessKey = common.GetEnvOrDefaultString("NUCLIO_ACCESS_KEY", "")
	}

	// if access key is still empty, fail
	if accessKey == "" {
		return nil, errors.New("Access key must be provided")
	}

	// ensure that apiURL is a correct URL
	baseURL, err := url.Parse(apiURL)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to parse API URL: %s", apiURL))
	}

	// ensure that API URL contains the /api suffix
	if !strings.HasSuffix(baseURL.Path, "api") && !strings.HasSuffix(baseURL.Path, "api/") {
		parentLogger.InfoWith("Adding `api` suffix to API URL",
			"apiURL", apiURL)
		if apiURL, err = url.JoinPath(apiURL, "api"); err != nil {
			return nil, errors.Wrap(err, "Failed to add `api` suffix to API URL")
		}
	}

	newAPIClient := &NuclioAPIClient{
		logger:         parentLogger.GetChild("api-client"),
		apiURL:         apiURL,
		requestTimeout: requestTimeout,
		username:       username,
		accessKey:      accessKey,
	}

	// parse the request timeout
	if requestTimeout == "" {
		requestTimeout = DefaultRequestTimeout
	}
	requestTimeoutDuration, err := time.ParseDuration(requestTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse request timeout")
	}

	// create HTTP client
	newAPIClient.httpClient = &http.Client{
		Timeout: requestTimeoutDuration,
	}
	if skipTLSVerify {
		newAPIClient.httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: skipTLSVerify},
		}
	}

	return newAPIClient, nil
}

// GetFunctions returns a map of function name to function config for all functions in the given namespace
func (c *NuclioAPIClient) GetFunctions(ctx context.Context, namespace string) (map[string]*functionconfig.ConfigWithStatus, error) {

	url := fmt.Sprintf("%s/%s", c.apiURL, FunctionsEndpoint)
	requestHeaders := map[string]string{
		headers.FunctionNamespace: namespace,
	}
	_, responseBody, err := c.sendRequest(ctx,
		http.MethodGet, // method
		url,            // url
		nil,            // body
		requestHeaders, // headers
		http.StatusOK,  // expectedStatusCode
		true)           // returnResponseBody
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get functions")
	}

	c.logger.DebugWithCtx(ctx, "Got functions", "numOfFunctions", len(responseBody))

	functions := map[string]*functionconfig.ConfigWithStatus{}

	for functionName, functionMap := range responseBody {
		functionConfigWithStatus, err := nuctlcommon.ConvertMapToFunctionConfigWithStatus(functionMap.(map[string]interface{}))
		if err != nil {
			return nil, errors.Wrap(err, "Failed to convert function config")
		}

		functions[functionName] = functionConfigWithStatus
	}

	return functions, nil
}

// GetFunction returns a single function with the given name and namespace
func (c *NuclioAPIClient) GetFunction(ctx context.Context, name, namespace string) (*functionconfig.ConfigWithStatus, error) {

	url := fmt.Sprintf("%s/%s/%s", c.apiURL, FunctionsEndpoint, name)
	requestHeaders := map[string]string{
		headers.FunctionNamespace:         namespace,
		headers.FunctionEnrichApiGateways: "true",
	}
	_, responseBody, err := c.sendRequest(ctx,
		http.MethodGet, // method
		url,            // url
		nil,            // body
		requestHeaders, // headers
		http.StatusOK,  // expectedStatusCode
		true)           // returnResponseBody
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get functions")
	}

	return nuctlcommon.ConvertMapToFunctionConfigWithStatus(responseBody)
}

// PatchFunction patches a single function with the given options
func (c *NuclioAPIClient) PatchFunction(ctx context.Context,
	functionName,
	namespace string,
	optionsPayload []byte,
	patchHeaders map[string]string) error {

	c.logger.DebugWithCtx(ctx, "Patching function", "function", functionName)

	url := fmt.Sprintf("%s/%s/%s", c.apiURL, FunctionsEndpoint, functionName)

	if _, _, err := c.sendRequest(ctx,
		http.MethodPatch,
		url,
		optionsPayload,
		patchHeaders,
		http.StatusAccepted,
		false); err != nil {
		switch typedError := err.(type) {
		case *nuclio.ErrorWithStatusCode:
			return nuclio.GetWrapByStatusCode(typedError.StatusCode())(errors.Wrap(err, "Failed to send patch API request"))
		default:
			return errors.Wrap(typedError, "Failed to send patch API request")
		}
	}

	return nil
}

// sendRequest sends an API request to the nuclio API
func (c *NuclioAPIClient) sendRequest(ctx context.Context,
	method,
	url string,
	requestBody []byte,
	requestHeaders map[string]string,
	expectedStatusCode int,
	returnResponseBody bool) (*http.Response, map[string]interface{}, error) {
	c.logger.DebugWithCtx(ctx,
		"Sending API request",
		"method", method,
		"url", url,
		"headers", requestHeaders,
		"body", string(requestBody))

	// create authorization headers
	authHeaders, err := c.createAuthorizationHeaders(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to create session")
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to create request")
	}

	req.Header.Set("Content-Type", "application/json")
	for key, value := range labels.Merge(requestHeaders, authHeaders) {
		req.Header.Set(key, value)
	}

	response, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to send request")
	}

	if response.StatusCode != expectedStatusCode {
		return nil, nil, nuclio.GetByStatusCode(response.StatusCode)(fmt.Sprintf("Expected status code %d, got %d", expectedStatusCode, response.StatusCode))
	}

	if !returnResponseBody {
		return response, nil, nil
	}

	encodedResponseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to read response body")
	}

	defer response.Body.Close() // nolint: errcheck

	decodedResponseBody := map[string]interface{}{}
	if err := json.Unmarshal(encodedResponseBody, &decodedResponseBody); err != nil {
		return nil, nil, errors.Wrap(err, "Failed to decode response body")
	}

	return response, decodedResponseBody, nil
}

// createAuthorizationHeaders creates authorization headers for the nuclio API
func (c *NuclioAPIClient) createAuthorizationHeaders(ctx context.Context) (map[string]string, error) {
	if c.authHeaders != nil {
		return c.authHeaders, nil
	}

	// cache the auth headers
	c.authHeaders = map[string]string{
		"x-v3io-username": c.username,
		"Authorization":   "Basic " + base64.StdEncoding.EncodeToString([]byte(c.username+":"+c.accessKey)),
	}

	return c.authHeaders, nil
}
