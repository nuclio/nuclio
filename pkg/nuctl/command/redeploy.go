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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/nuclio/nuclio/pkg/errgroup"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/errors"
	"github.com/spf13/cobra"
)

const (
	SessionEndpoint   = "api/sessions"
	FunctionsEndpoint = "nuclio-proxy/api/functions"
)

type redeployCommandeer struct {
	cmd            *cobra.Command
	rootCommandeer *RootCommandeer
}

func newRedeployCommandeer(ctx context.Context, rootCommandeer *RootCommandeer) *redeployCommandeer {
	commandeer := &redeployCommandeer{
		rootCommandeer: rootCommandeer,
	}

	cmd := &cobra.Command{
		Use:   "redeploy",
		Short: "Redeploy functions",
	}

	cmd.AddCommand(
		newRedeployFunctionsCommandeer(ctx, commandeer).cmd,
	)

	commandeer.cmd = cmd

	return commandeer
}

type redeployFunctionsCommandeer struct {
	*redeployCommandeer
	verbose            bool
	namespace          string
	excludedProjects   []string
	excludedFunctions  []string
	concurrency        int
	username           string
	password           string
	baseURL            string
	outputManifest     map[string]interface{}
	outputManifestLock sync.Mutex
	httpClient         *http.Client
}

func newRedeployFunctionsCommandeer(ctx context.Context, redeployCommandeer *redeployCommandeer) *redeployFunctionsCommandeer {
	commandeer := &redeployFunctionsCommandeer{
		redeployCommandeer: redeployCommandeer,
		outputManifest: map[string]interface{}{
			"Failed":  map[string]error{},
			"Success": []string{},
		},
		outputManifestLock: sync.Mutex{},
	}

	cmd := &cobra.Command{
		Use:     "functions",
		Aliases: []string{"func", "fn", "function"},
		Short:   "Redeploy functions",
		RunE: func(cmd *cobra.Command, args []string) error {

			// initialize root
			if err := redeployCommandeer.rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			// initialize http client
			commandeer.httpClient = &http.Client{}

			return commandeer.redeployFunctions(ctx)
		},
	}

	cmd.PersistentFlags().BoolVarP(&commandeer.verbose, "verbose", "v", false, "Enable verbose logging")
	cmd.PersistentFlags().StringVarP(&commandeer.namespace, "tenant", "t", "default-tenant", "Tenant name")
	cmd.PersistentFlags().StringSliceVarP(&commandeer.excludedProjects, "exclude-projects", "ep", []string{}, "Exclude projects to redeploy")
	cmd.PersistentFlags().StringSliceVarP(&commandeer.excludedFunctions, "exclude-functions", "ef", []string{}, "Exclude functions to redeploy")
	cmd.PersistentFlags().IntVarP(&commandeer.concurrency, "concurrency", "c", 10, "Number of parallel redeployments")
	cmd.PersistentFlags().StringVarP(&commandeer.username, "username", "u", "", "Username of a user with permissions to redeploy functions")
	cmd.PersistentFlags().StringVarP(&commandeer.password, "password", "p", "", "Password of a user with permissions to redeploy functions")
	cmd.PersistentFlags().StringVarP(&commandeer.baseURL, "base-url", "url", "", "URL of the iguazio dashboard")

	// mark required flags
	cmd.MarkPersistentFlagRequired("username") // nolint: errcheck
	cmd.MarkPersistentFlagRequired("password") // nolint: errcheck
	cmd.MarkPersistentFlagRequired("base-url") // nolint: errcheck

	return commandeer
}

func (c *redeployFunctionsCommandeer) redeployFunctions(ctx context.Context) error {

	functionNames, err := c.getFunctionNames(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to get function names")
	}

	redeployErrGroup, _ := errgroup.WithContextSemaphore(ctx, c.rootCommandeer.loggerInstance, uint(c.concurrency))
	for _, function := range functionNames {
		function := function
		redeployErrGroup.Go("redeploy function", func() error {
			if err := c.redeployFunction(ctx, function); err != nil {
				c.outputManifestLock.Lock()
				c.outputManifest["Failed"].(map[string]error)[function] = err
				c.outputManifestLock.Unlock()
				return errors.Wrap(err, "Failed to redeploy function")
			}
			c.outputManifestLock.Lock()
			c.outputManifest["Success"] = append(c.outputManifest["Success"].([]string), function)
			c.outputManifestLock.Unlock()
			return nil
		})
	}

	if err := redeployErrGroup.Wait(); err != nil {
		c.rootCommandeer.loggerInstance.WarnWith("Failed to redeploy functions", "err", err)
	}

	c.logOutput()

	return nil
}

func (c *redeployFunctionsCommandeer) getFunctionNames(ctx context.Context) ([]string, error) {

	// get all functions in the namespace
	functions, err := c.rootCommandeer.platform.GetFunctions(ctx, &platform.GetFunctionsOptions{
		Namespace: c.namespace,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get functions")
	}

	functionNames := make([]string, 0)
	for _, function := range functions {
		functionNames = append(functionNames, function.GetConfig().Meta.Name)
	}

	return functionNames, nil
}

func (c *redeployFunctionsCommandeer) redeployFunction(ctx context.Context, function string) error {

	// create session
	cookieHeader, err := c.createSession(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to create session")
	}

	// redeploy function
	url := fmt.Sprintf("%s/%s/%s", c.baseURL, FunctionsEndpoint, function)
	payload, err := json.Marshal(map[string]interface{}{
		"desiredState": "ready",
	})
	if err != nil {
		return errors.Wrap(err, "Failed to marshal payload")
	}

	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewBuffer(payload))
	if err != nil {
		return errors.Wrapf(err, "Failed to create redeploy request for function %s", function)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", cookieHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return errors.Wrapf(err, "Failed to send redeploy request for function %s", function)
	}

	defer resp.Body.Close() // nolint: errcheck

	if resp.StatusCode != http.StatusNoContent {
		return errors.Errorf("Failed to redeploy function %s. Status code: %d", function, resp.StatusCode)
	}

	return nil
}

func (c *redeployFunctionsCommandeer) createSession(ctx context.Context) (string, error) {
	payload, err := json.Marshal(map[string]interface{}{
		"data": map[string]interface{}{
			"type": "session",
			"attributes": map[string]interface{}{
				"username": c.username,
				"password": c.password,
			},
		},
	})
	if err != nil {
		return "", errors.Wrap(err, "Failed to marshal payload")
	}

	url := fmt.Sprintf("%s/%s", c.baseURL, SessionEndpoint)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))
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

func (c *redeployFunctionsCommandeer) cookieToHeader(cookie map[string]string) (string, error) {
	marshalledCookie, err := json.Marshal(cookie)
	if err != nil {
		return "", errors.Wrap(err, "Failed to marshal cookie")
	}

	return fmt.Sprintf("session=j:%s", string(marshalledCookie)), nil

}

func (c *redeployFunctionsCommandeer) logOutput() {
	successful := c.outputManifest["Success"].([]string)
	failed := c.outputManifest["Failed"].(map[string]error)
	c.rootCommandeer.loggerInstance.InfoWith("Redeployed functions successfully", "functions", successful)
	if len(failed) > 0 {
		c.rootCommandeer.loggerInstance.Error("Failed to redeploy functions:")
		for function, err := range failed {
			c.rootCommandeer.loggerInstance.ErrorWith("Failed to redeploy function", "function", function, "err", err)
		}
	}
}
