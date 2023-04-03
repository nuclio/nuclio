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
	"net/http"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dashboard/resource"
	"github.com/nuclio/nuclio/pkg/errgroup"
	nuctlcommon "github.com/nuclio/nuclio/pkg/nuctl/command/common"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/errors"
	"github.com/spf13/cobra"
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

type patchFunctionsCommandeer struct {
	*patchCommandeer

	excludedProjects    []string
	excludedFunctions   []string
	concurrency         int
	waitForFunction     bool
	skipFunctionWithGPU bool
	outputManifest      *nuctlcommon.OutputManifest
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
	cmd.PersistentFlags().StringVarP(&commandeer.apiURL, "api-url", "", "", "URL of the nuclio API (e.g. https://nuclio.io:8070/api)")
	cmd.PersistentFlags().BoolVarP(&commandeer.waitForFunction, "wait", "w", false, "Wait for function to be ready after patching")
	cmd.PersistentFlags().BoolVarP(&commandeer.skipFunctionWithGPU, "skip-gpu", "", false, "Skip functions with GPU")

	// mark required flags
	cmd.MarkPersistentFlagRequired("api-url") // nolint: errcheck

	commandeer.cmd = cmd

	return commandeer
}

func (c *patchFunctionsCommandeer) patchFunctions(ctx context.Context) error {

	functionNames, err := c.getFunctionNames(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to get function names")
	}

	c.rootCommandeer.loggerInstance.DebugWithCtx(ctx, "Got function names", "functionNames", functionNames)

	// create authorization headers
	authHeaders, err := c.createAuthorizationHeaders(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to create session")
	}

	patchErrGroup, _ := errgroup.WithContextSemaphore(ctx, c.rootCommandeer.loggerInstance, uint(c.concurrency))
	for _, function := range functionNames {
		function := function
		patchErrGroup.Go("patch function", func() error {
			if err := c.patchFunction(ctx, function, authHeaders); err != nil {
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

func (c *patchFunctionsCommandeer) getFunctionNames(ctx context.Context) ([]string, error) {
	c.rootCommandeer.loggerInstance.DebugWithCtx(ctx, "Getting function names")

	// get all functions in the namespace
	functions, err := c.rootCommandeer.platform.GetFunctions(ctx, &platform.GetFunctionsOptions{
		Namespace: c.rootCommandeer.namespace,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get functions")
	}

	functionNames := make([]string, 0)
	for _, function := range functions {
		functionName := function.GetConfig().Meta.Name

		// filter excluded functions
		if c.shouldSkipFunction(function) {
			c.outputManifest.AddSkipped(functionName)
			c.rootCommandeer.loggerInstance.DebugWithCtx(ctx, "Excluding function", "function", functionName)
			continue
		}
		functionNames = append(functionNames, functionName)
	}

	return functionNames, nil
}

func (c *patchFunctionsCommandeer) patchFunction(ctx context.Context, function string, sessionCookieHeader map[string]string) error {

	c.rootCommandeer.loggerInstance.DebugWithCtx(ctx, "Patching function", "function", function)

	// patch function
	payload, err := c.createPatchPayload(ctx, function)
	if err != nil {
		return errors.Wrap(err, "Failed to create patch payload")
	}
	url := fmt.Sprintf("%s/%s/%s", c.apiURL, FunctionsEndpoint, function)

	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewBuffer(payload))
	if err != nil {
		return errors.Wrapf(err, "Failed to create patch request for function %s", function)
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range sessionCookieHeader {
		req.Header.Set(key, value)
	}

	if c.waitForFunction {

		// add a header that will cause the API to wait for the function to be ready after patching
		req.Header.Set("x-nuclio-wait-function-action", "true")
	}

	c.rootCommandeer.loggerInstance.DebugWithCtx(ctx, "Sending patch request",
		"url", url,
		"payload", string(payload),
		"headers", req.Header)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return errors.Wrapf(err, "Failed to send patch request for function %s", function)
	}

	defer resp.Body.Close() // nolint: errcheck

	if resp.StatusCode != http.StatusNoContent {
		return errors.Errorf("Failed to patch function %s. Status code: %d", function, resp.StatusCode)
	}

	return nil
}

func (c *patchFunctionsCommandeer) createAuthorizationHeaders(ctx context.Context) (map[string]string, error) {

	// resolve username and password from env vars if not provided
	if c.username == "" {
		c.username = common.GetEnvOrDefaultString("NUCLIO_USERNAME", "")
	}
	if c.password == "" {
		c.password = common.GetEnvOrDefaultString("NUCLIO_PASSWORD", "")
	}

	// create authorization headers
	return map[string]string{
		"x-v3io-username": c.username,
		"Authorization":   "Basic " + base64.StdEncoding.EncodeToString([]byte(c.username+":"+c.password)),
	}, nil
}

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

func (c *patchFunctionsCommandeer) shouldSkipFunction(function platform.Function) bool {
	functionName := function.GetConfig().Meta.Name
	projectName := function.GetConfig().Meta.Labels[common.NuclioResourceLabelKeyProjectName]

	// skip if function is excluded or if it has a positive GPU resource limit
	if common.StringSliceContainsString(c.excludedFunctions, functionName) ||
		common.StringSliceContainsString(c.excludedProjects, projectName) ||
		function.GetConfig().Spec.PositiveGPUResourceLimit() {
		return true
	}

	return false
}

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
