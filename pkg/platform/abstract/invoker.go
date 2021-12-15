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

package abstract

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

type invoker struct {
	logger                          logger.Logger
	platform                        platform.Platform
	createFunctionInvocationOptions *platform.CreateFunctionInvocationOptions
}

func newInvoker(parentLogger logger.Logger, platform platform.Platform) (*invoker, error) {
	newinvoker := &invoker{
		logger:   parentLogger.GetChild("invoker"),
		platform: platform,
	}

	return newinvoker, nil
}

func (i *invoker) invoke(ctx context.Context,
	function platform.Function,
	createFunctionInvocationOptions *platform.CreateFunctionInvocationOptions) (
	*platform.CreateFunctionInvocationResult, error) {

	// save options
	i.createFunctionInvocationOptions = createFunctionInvocationOptions

	invokeURL, err := i.resolveInvokeURL(ctx, function, createFunctionInvocationOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to resolve invocation url")
	}

	fullpath := fmt.Sprintf("http://%s", invokeURL)
	if createFunctionInvocationOptions.Path != "" {
		fullpath += "/" + createFunctionInvocationOptions.Path
	}

	client := &http.Client{
		Timeout: createFunctionInvocationOptions.Timeout,
	}
	var req *http.Request
	var body io.Reader = http.NoBody

	// set body for post
	if createFunctionInvocationOptions.Method != "GET" {
		body = bytes.NewBuffer(createFunctionInvocationOptions.Body)
	}

	// issue the request
	req, err = http.NewRequest(createFunctionInvocationOptions.Method, fullpath, body)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create HTTP request")
	}

	// set headers
	req.Header = createFunctionInvocationOptions.Headers
	req.Header.Set("x-nuclio-target", function.GetConfig().Meta.Name)

	// request logs from a given verbosity unless we're specified no logs should be returned
	if createFunctionInvocationOptions.LogLevelName != "none" && req.Header.Get("x-nuclio-log-level") == "" {
		req.Header.Set("x-nuclio-log-level", createFunctionInvocationOptions.LogLevelName)
	}

	i.logger.InfoWithCtx(ctx, "Executing function",
		"method", createFunctionInvocationOptions.Method,
		"url", fullpath,
		"headers", req.Header)

	response, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to send HTTP request")
	}

	defer response.Body.Close() // nolint: errcheck

	i.logger.InfoWithCtx(ctx, "Got response", "status", response.Status)

	// read the body
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read response body")
	}

	return &platform.CreateFunctionInvocationResult{
		Headers:    response.Header,
		Body:       responseBody,
		StatusCode: response.StatusCode,
	}, nil
}

func (i *invoker) resolveInvokeURL(ctx context.Context,
	function platform.Function,
	createFunctionInvocationOptions *platform.CreateFunctionInvocationOptions) (string, error) {
	if createFunctionInvocationOptions.URL != "" {

		// validate given url, must matching one of the function status invocation urls
		if !common.StringSliceContainsString(function.GetStatus().InvocationURLs(),
			createFunctionInvocationOptions.URL) {
			i.logger.WarnWithCtx(ctx, "Invocation URL does not match any of function status invocation urls",
				"url", createFunctionInvocationOptions.URL,
				"invocationURLs", function.GetStatus().InvocationURLs())
			return "", nuclio.NewErrBadRequest(fmt.Sprintf("Invalid function url %s",
				createFunctionInvocationOptions.URL))
		}
		return createFunctionInvocationOptions.URL, nil
	}

	// get where the function resides
	return function.GetInvokeURL(ctx, createFunctionInvocationOptions.Via)
}
