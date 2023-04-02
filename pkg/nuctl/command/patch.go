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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dashboard/resource"
	"github.com/nuclio/nuclio/pkg/errgroup"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/errors"
	"github.com/spf13/cobra"
)

const (
	SessionEndpoint   = "api/sessions"
	FunctionsEndpoint = "nuclio-proxy/api/functions"
)

type patchCommandeer struct {
	cmd             *cobra.Command
	rootCommandeer  *RootCommandeer
	patchOptionsMap map[string]string
	patchOptions    *resource.PatchOptions
	httpClient      *http.Client
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

	cmd.AddCommand(
		newPatchFunctionsCommandeer(ctx, commandeer).cmd,
	)

	commandeer.cmd = cmd

	return commandeer
}

func (c *patchCommandeer) initialize() error {

	// initialize http client
	c.httpClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	if err := c.initializePatchOptions(); err != nil {
		return errors.Wrap(err, "Failed to initialize patch options")
	}

	c.rootCommandeer.loggerInstance.DebugWith("Initialized patch commandeer",
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

	excludedProjects   []string
	excludedFunctions  []string
	concurrency        int
	username           string
	password           string
	apiURL             string
	waitForFunction    bool
	outputManifest     map[string]interface{}
	outputManifestLock sync.Mutex
}

func newPatchFunctionsCommandeer(ctx context.Context, patchCommandeer *patchCommandeer) *patchFunctionsCommandeer {
	commandeer := &patchFunctionsCommandeer{
		patchCommandeer: patchCommandeer,
		outputManifest: map[string]interface{}{
			"Failed":  map[string]error{},
			"Success": []string{},
		},
		outputManifestLock: sync.Mutex{},
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
			if err := patchCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize patch commandeer")
			}

			return commandeer.patchFunctions(ctx)
		},
	}

	cmd.PersistentFlags().StringSliceVarP(&commandeer.excludedProjects, "exclude-projects", "", []string{}, "Exclude projects to patch")
	cmd.PersistentFlags().StringSliceVarP(&commandeer.excludedFunctions, "exclude-functions", "", []string{}, "Exclude functions to patch")
	cmd.PersistentFlags().IntVarP(&commandeer.concurrency, "concurrency", "c", 10, "Max number of parallel patches")
	cmd.PersistentFlags().StringVarP(&commandeer.username, "username", "u", "", "Username of a user with permissions to patch functions")
	cmd.PersistentFlags().StringVarP(&commandeer.password, "password", "p", "", "Password of a user with permissions to patch functions")
	cmd.PersistentFlags().StringVarP(&commandeer.apiURL, "api-url", "", "", "URL of the nuclio API (e.g. https://nuclio.io:8070)")
	cmd.PersistentFlags().BoolVarP(&commandeer.waitForFunction, "wait", "w", false, "Wait for function to be ready after patching")

	// mark required flags
	cmd.MarkPersistentFlagRequired("username") // nolint: errcheck
	cmd.MarkPersistentFlagRequired("password") // nolint: errcheck
	cmd.MarkPersistentFlagRequired("base-url") // nolint: errcheck

	commandeer.cmd = cmd

	return commandeer
}

func (c *patchFunctionsCommandeer) patchFunctions(ctx context.Context) error {

	functionNames, err := c.getFunctionNames(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to get function names")
	}

	c.rootCommandeer.loggerInstance.DebugWith("Got function names", "functionNames", functionNames)

	// create session
	sessionCookieHeader, err := c.createSession(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to create session")
	}

	patchErrGroup, _ := errgroup.WithContextSemaphore(ctx, c.rootCommandeer.loggerInstance, uint(c.concurrency))
	for _, function := range functionNames {
		function := function
		patchErrGroup.Go("patch function", func() error {
			err := c.patchFunction(ctx, function, sessionCookieHeader)
			c.addResultToOutputManifest(function, err)
			if err != nil {
				return errors.Wrap(err, "Failed to patch function")
			}
			return nil
		})
	}

	if err := patchErrGroup.Wait(); err != nil {
		c.rootCommandeer.loggerInstance.WarnWith("Failed to patch functions", "err", err)
	}

	c.logOutput()

	return nil
}

func (c *patchFunctionsCommandeer) getFunctionNames(ctx context.Context) ([]string, error) {
	c.rootCommandeer.loggerInstance.DebugWith("Getting function names")

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
		projectName := function.GetConfig().Meta.Labels["nuclio.io/project-name"]

		// filter excluded functions
		if common.StringSliceContainsString(c.excludedFunctions, functionName) ||
			common.StringSliceContainsString(c.excludedProjects, projectName) {

			c.rootCommandeer.loggerInstance.DebugWith("Excluding function", "function", functionName)
			continue
		}
		functionNames = append(functionNames, functionName)
	}

	return functionNames, nil
}

func (c *patchFunctionsCommandeer) patchFunction(ctx context.Context, function, sessionCookieHeader string) error {

	c.rootCommandeer.loggerInstance.DebugWith("Patching function", "function", function)

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
	req.Header.Set("Cookie", sessionCookieHeader)
	if c.waitForFunction {

		// add a header that will cause the API to wait for the function to be ready after patching
		req.Header.Set("x-nuclio-wait-function-action", "true")
	}

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

func (c *patchFunctionsCommandeer) createSession(ctx context.Context) (string, error) {
	c.rootCommandeer.loggerInstance.DebugWith("Creating session")

	url := fmt.Sprintf("%s/%s", c.apiURL, SessionEndpoint)

	req, err := http.NewRequest(http.MethodPost, url, nil)
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "application/json")
	if err != nil {
		return "", errors.Wrap(err, "Failed to create request")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "Failed to send request")
	}

	defer resp.Body.Close() // nolint: errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "Failed to read response body")
	}

	if resp.StatusCode != http.StatusCreated {
		return "", errors.Errorf("Failed to create session. Status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var sessionResponse map[string]interface{}
	if err := json.Unmarshal(body, &sessionResponse); err != nil {
		return "", errors.Wrap(err, "Failed to unmarshal response body")
	}

	sessionId := sessionResponse["data"].(map[string]interface{})["id"].(string)
	cookie := map[string]string{
		"sid": sessionId,
	}

	return c.cookieToHeader(cookie)
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

func (c *patchFunctionsCommandeer) cookieToHeader(cookie map[string]string) (string, error) {
	marshalledCookie, err := json.Marshal(cookie)
	if err != nil {
		return "", errors.Wrap(err, "Failed to marshal cookie")
	}

	return fmt.Sprintf("session=j:%s", string(marshalledCookie)), nil
}

func (c *patchFunctionsCommandeer) addResultToOutputManifest(function string, err error) {
	c.outputManifestLock.Lock()
	defer c.outputManifestLock.Unlock()

	if err != nil {
		c.outputManifest["Failed"].(map[string]error)[function] = err
	} else {
		c.outputManifest["Success"] = append(c.outputManifest["Success"].([]string), function)
	}
}

func (c *patchFunctionsCommandeer) logOutput() {
	successful := c.outputManifest["Success"].([]string)
	failed := c.outputManifest["Failed"].(map[string]error)
	if len(successful) > 0 {
		c.rootCommandeer.loggerInstance.InfoWith("Patched functions successfully", "functions", successful)
	}
	if len(failed) > 0 {
		c.rootCommandeer.loggerInstance.Error("Failed to patch functions:")
		for function, err := range failed {
			c.rootCommandeer.loggerInstance.ErrorWith("Failed to patch function", "function", function, "err", err)
		}
	}
}
