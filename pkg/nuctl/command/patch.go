/*
Copyright 2017 The Nuclio Authors.

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

package command

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/dashboard/resource"
	"github.com/nuclio/nuclio/pkg/errgroup"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuctlcommon "github.com/nuclio/nuclio/pkg/nuctl/command/common"

	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/errors"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	FunctionsEndpoint     = "functions"
	DefaultRequestTimeout = "60s"
	DefaultConcurrency    = 10
)

type patchCommandeer struct {
	cmd             *cobra.Command
	rootCommandeer  *RootCommandeer
	patchOptionsMap map[string]string
	patchOptions    *resource.PatchOptions
	httpClient      *http.Client
	apiURL          string
	username        string
	password        string
	requestTimeout  string
	skipTLSVerify   bool
	authHeaders     map[string]string
}

func newPatchCommandeer(ctx context.Context, rootCommandeer *RootCommandeer) *patchCommandeer {
	commandeer := &patchCommandeer{
		rootCommandeer: rootCommandeer,
	}

	cmd := &cobra.Command{
		Use:   "patch",
		Short: "Apply a patch to a resource",
	}

	cmd.PersistentFlags().StringToStringVarP(&commandeer.patchOptionsMap,
		"patch-options",
		"o",
		map[string]string{},
		"Patch options, as a key=value (e.g. -o key1=value1). Can be used multiple times.")
	cmd.PersistentFlags().StringVarP(&commandeer.apiURL, "api-url", "", "", "URL of the nuclio API (e.g. https://nuclio.io:8070)")
	cmd.PersistentFlags().StringVarP(&commandeer.username, "username", "u", "", "Username of a user with permissions to the nuclio API")
	cmd.PersistentFlags().StringVarP(&commandeer.password, "password", "p", "", "Password/Access Key of a user with permissions to the nuclio API")
	cmd.PersistentFlags().StringVarP(&commandeer.requestTimeout, "request-timeout", "", "60s", "Request timeout")
	cmd.PersistentFlags().BoolVarP(&commandeer.skipTLSVerify, "skip-tls-verify", "", false, "Skip TLS verification")

	cmd.MarkPersistentFlagRequired("api-url") // nolint: errcheck

	cmd.AddCommand(
		newPatchFunctionsCommandeer(ctx, commandeer).cmd,
	)

	commandeer.cmd = cmd

	return commandeer
}

// initialize will initialize the patch commandeer
func (c *patchCommandeer) initialize(ctx context.Context) error {

	// parse the request timeout
	if c.requestTimeout == "" {
		c.requestTimeout = DefaultRequestTimeout
	}
	requestTimeoutDuration, err := time.ParseDuration(c.requestTimeout)
	if err != nil {
		return errors.Wrap(err, "Failed to parse request timeout")
	}

	// initialize http client
	c.httpClient = &http.Client{
		Timeout: requestTimeoutDuration,
	}
	if c.skipTLSVerify {
		c.httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	if err := c.initializePatchOptions(); err != nil {
		return errors.Wrap(err, "Failed to initialize patch options")
	}

	c.rootCommandeer.loggerInstance.DebugWithCtx(ctx, "Initialized patch commandeer",
		"patchOptions", c.patchOptions)

	return nil
}

// initializePatchOptions transforms the given patch options map into a struct
func (c *patchCommandeer) initializePatchOptions() error {
	if len(c.patchOptionsMap) == 0 {
		return nil
	}

	c.patchOptions = &resource.PatchOptions{}

	// convert the patch options map to a patch options struct
	if err := mapstructure.Decode(c.patchOptionsMap, c.patchOptions); err != nil {
		return errors.Wrap(err, "Failed to decode patch options")
	}

	return nil
}

// sendAPIRequest sends an API request to the nuclio API
func (c *patchCommandeer) sendAPIRequest(ctx context.Context,
	method,
	url string,
	requestBody []byte,
	requestHeaders map[string]string,
	expectedStatusCode int,
	returnResponseBody bool) (*http.Response, map[string]interface{}, error) {
	c.rootCommandeer.loggerInstance.DebugWithCtx(ctx,
		"Sending API request",
		"method", method,
		"url", url,
		"headers", requestHeaders)

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
		return nil, nil, errors.Errorf("Expected status code %d, got %d", expectedStatusCode, response.StatusCode)
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
func (c *patchCommandeer) createAuthorizationHeaders(ctx context.Context) (map[string]string, error) {
	if c.authHeaders != nil {
		return c.authHeaders, nil
	}

	// resolve username and password from env vars if not provided
	if c.username == "" {
		c.username = common.GetEnvOrDefaultString("NUCLIO_USERNAME", "")
	}
	if c.password == "" {
		c.password = common.GetEnvOrDefaultString("NUCLIO_PASSWORD", "")
	}

	// if username and password are still empty, fail
	if c.username == "" || c.password == "" {
		return nil, errors.New("Username and password must be provided")
	}

	// cache the auth headers
	c.authHeaders = map[string]string{
		"x-v3io-username": c.username,
		"Authorization":   "Basic " + base64.StdEncoding.EncodeToString([]byte(c.username+":"+c.password)),
	}

	return c.authHeaders, nil
}

type patchFunctionsCommandeer struct {
	*patchCommandeer
	excludedProjects       []string
	excludedFunctions      []string
	concurrency            int
	waitForFunction        bool
	excludeFunctionWithGPU bool
	outputManifest         *nuctlcommon.PatchOutputManifest
}

func newPatchFunctionsCommandeer(ctx context.Context, patchCommandeer *patchCommandeer) *patchFunctionsCommandeer {
	commandeer := &patchFunctionsCommandeer{
		patchCommandeer: patchCommandeer,
		outputManifest:  nuctlcommon.NewOutputManifest(),
	}

	cmd := &cobra.Command{
		Use:     "functions",
		Aliases: []string{"func", "fn", "function"},
		Short:   "patch functions",
		RunE: func(cmd *cobra.Command, args []string) error {

			// initialize root
			if err := patchCommandeer.rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			// initialize patch commandeer
			if err := patchCommandeer.initialize(ctx); err != nil {
				return errors.Wrap(err, "Failed to initialize patch commandeer")
			}

			if err := commandeer.validateAndEnrichFlags(); err != nil {
				return errors.Wrap(err, "Failed to validate flags")
			}

			return commandeer.patchFunctions(ctx)
		},
	}

	cmd.PersistentFlags().StringSliceVarP(&commandeer.excludedProjects, "exclude-projects", "", []string{}, "Exclude projects to patch")
	cmd.PersistentFlags().StringSliceVarP(&commandeer.excludedFunctions, "exclude-functions", "", []string{}, "Exclude functions to patch")
	cmd.PersistentFlags().IntVarP(&commandeer.concurrency, "concurrency", "c", DefaultConcurrency, "Max number of parallel patches")
	cmd.PersistentFlags().BoolVarP(&commandeer.waitForFunction, "wait", "w", false, "Wait for function deployment to complete")
	cmd.PersistentFlags().BoolVarP(&commandeer.excludeFunctionWithGPU, "exclude-functions-with-gpu", "", false, "Skip functions with GPU")

	commandeer.cmd = cmd

	return commandeer
}

// patchFunctions patches functions
func (c *patchFunctionsCommandeer) patchFunctions(ctx context.Context) error {

	functionNames, err := c.getFunctionNames(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to get function names")
	}

	c.rootCommandeer.loggerInstance.DebugWithCtx(ctx, "Got function names", "functionNames", functionNames)

	patchErrGroup, _ := errgroup.WithContextSemaphore(ctx, c.rootCommandeer.loggerInstance, uint(c.concurrency))
	for _, function := range functionNames {
		function := function
		patchErrGroup.Go("patch function", func() error {
			if err := c.patchFunction(ctx, function); err != nil {
				c.outputManifest.AddFailure(function, err)
				return errors.Wrap(err, "Failed to patch function")
			}
			c.outputManifest.AddSuccess(function)
			return nil
		})
	}

	if err := patchErrGroup.Wait(); err != nil {

		// Functions that failed to patch are included in the output manifest,
		// so we don't need to fail the entire operation here
		c.rootCommandeer.loggerInstance.WarnWithCtx(ctx, "Failed to patch functions", "err", err)
	}

	c.logOutput(ctx)

	return nil
}

// getFunctionNames returns a list of function names
func (c *patchFunctionsCommandeer) getFunctionNames(ctx context.Context) ([]string, error) {
	c.rootCommandeer.loggerInstance.DebugWithCtx(ctx, "Getting function names")

	functionConfigs, err := c.getFunctions(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get functions")
	}

	functionNames := make([]string, 0)
	for functionName, functionConfig := range functionConfigs {

		// filter excluded functions
		if c.shouldSkipFunction(functionConfig) {
			c.outputManifest.AddSkipped(functionName)
			c.rootCommandeer.loggerInstance.DebugWithCtx(ctx, "Excluding function", "function", functionName)
			continue
		}
		functionNames = append(functionNames, functionName)
	}

	return functionNames, nil
}

// getFunctions returns a map of function name to function config
func (c *patchFunctionsCommandeer) getFunctions(ctx context.Context) (map[string]functionconfig.Config, error) {
	url := fmt.Sprintf("%s/%s", c.apiURL, FunctionsEndpoint)
	requestHeaders := map[string]string{
		headers.FunctionNamespace: c.rootCommandeer.namespace,
	}
	_, responseBody, err := c.sendAPIRequest(ctx,
		http.MethodGet, // method
		url,            // url
		nil,            // body
		requestHeaders, // headers
		http.StatusOK,  // expectedStatusCode
		true)           // returnResponseBody
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get functions")
	}

	c.rootCommandeer.loggerInstance.DebugWithCtx(ctx, "Got functions", "numOfFunctions", len(responseBody))

	functions := map[string]functionconfig.Config{}

	for functionName, functionConfigMap := range responseBody {
		functionConfig, err := nuctlcommon.ConvertToFunctionConfig(functionConfigMap.(map[string]interface{}))
		if err != nil {
			return nil, errors.Wrap(err, "Failed to convert function config")
		}

		functions[functionName] = functionConfig
	}

	return functions, nil
}

// patchFunction patches a single function
func (c *patchFunctionsCommandeer) patchFunction(ctx context.Context, function string) error {

	c.rootCommandeer.loggerInstance.DebugWithCtx(ctx, "Patching function", "function", function)

	// patch function
	payload, err := c.createPatchPayload(ctx, function)
	if err != nil {
		return errors.Wrap(err, "Failed to create patch payload")
	}
	url := fmt.Sprintf("%s/%s/%s", c.apiURL, FunctionsEndpoint, function)

	requestHeaders := map[string]string{}
	if c.waitForFunction {
		// add a header that will cause the API to wait for the function to be ready after patching
		requestHeaders[headers.WaitFunctionAction] = "true"
	}

	if _, _, err = c.sendAPIRequest(ctx,
		http.MethodPatch,
		url,
		payload,
		requestHeaders,
		http.StatusNoContent,
		false); err != nil {
		return errors.Wrap(err, "Failed to send patch API request")
	}

	return nil
}

// createPatchPayload creates and enriches a patch payload, including patch options
func (c *patchFunctionsCommandeer) createPatchPayload(ctx context.Context, function string) ([]byte, error) {
	if len(c.patchOptionsMap) == 0 {

		// enrich with default options
		c.patchOptionsMap = map[string]string{
			"desiredState": "ready",
		}
	}

	payload, err := json.Marshal(c.patchOptionsMap)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to marshal payload")
	}

	return payload, nil
}

// logOutput outputs the output manifest to the logger
func (c *patchFunctionsCommandeer) logOutput(ctx context.Context) {
	if len(c.outputManifest.GetSuccess()) > 0 {
		c.rootCommandeer.loggerInstance.InfoWithCtx(ctx, "Patched functions successfully",
			"functions", c.outputManifest.GetSuccess())
	}
	if len(c.outputManifest.GetSkipped()) > 0 {
		c.rootCommandeer.loggerInstance.InfoWithCtx(ctx, "Skipped functions",
			"functions", c.outputManifest.GetSkipped())
	}
	if len(c.outputManifest.GetFailed()) > 0 {
		for function, err := range c.outputManifest.GetFailed() {
			c.rootCommandeer.loggerInstance.ErrorWithCtx(ctx, "Failed to patch function",
				"function", function,
				"err", err)
		}
	}
}

// shouldSkipFunction returns true if the function patch should be skipped
func (c *patchFunctionsCommandeer) shouldSkipFunction(functionConfig functionconfig.Config) bool {
	functionName := functionConfig.Meta.Name
	projectName := functionConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName]

	// skip if function is excluded or if it has a positive GPU resource limit
	if common.StringSliceContainsString(c.excludedFunctions, functionName) ||
		common.StringSliceContainsString(c.excludedProjects, projectName) ||
		functionConfig.Spec.PositiveGPUResourceLimit() {
		return true
	}

	return false
}

// validateAndEnrichFlags validates and enriches flags
func (c *patchFunctionsCommandeer) validateAndEnrichFlags() error {

	// validate api url
	c.apiURL = strings.TrimSuffix(c.apiURL, "/")
	if !strings.HasSuffix(c.apiURL, "/api") {
		c.apiURL += "/api"
	}

	// validate concurrency
	if c.concurrency < 1 {
		return errors.New("Concurrency must be a positive number")
	}

	return nil
}
