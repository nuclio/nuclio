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

package abstract

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

type invoker struct {
	logger   logger.Logger
	platform platform.Platform
}

func newInvoker(parentLogger logger.Logger, platform platform.Platform) (*invoker, error) {
	return &invoker{
		logger:   parentLogger.GetChild("invoker"),
		platform: platform,
	}, nil
}

func (i *invoker) invoke(ctx context.Context,
	createFunctionInvocationOptions *platform.CreateFunctionInvocationOptions) (
	*platform.CreateFunctionInvocationResult, error) {

	// enrich function instance
	if createFunctionInvocationOptions.FunctionInstance == nil {
		if err := createFunctionInvocationOptions.EnrichFunction(ctx, i.platform); err != nil {
			return nil, errors.Wrap(err, "Failed to resolve function")
		}
	}

	// for API backwards compatibility - enrich url in case it's not given
	if createFunctionInvocationOptions.URL == "" &&
		len(createFunctionInvocationOptions.FunctionInstance.GetStatus().InvocationURLs()) > 0 {
		invocationURL := createFunctionInvocationOptions.FunctionInstance.GetStatus().InvocationURLs()[0]
		i.logger.DebugWithCtx(ctx,
			"Using default invocation URL",
			"url", invocationURL)
		createFunctionInvocationOptions.URL = invocationURL
	}

	invokeURL, err := i.resolveInvokeURL(ctx, createFunctionInvocationOptions)
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

	// if tls verification is disabled, skip verification
	if createFunctionInvocationOptions.SkipTLSVerification {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: createFunctionInvocationOptions.SkipTLSVerification},
		}
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
	req.Header.Set(headers.TargetName, createFunctionInvocationOptions.FunctionInstance.GetConfig().Meta.Name)

	// request logs from a given verbosity unless we're specified no logs should be returned
	if createFunctionInvocationOptions.LogLevelName != "none" && req.Header.Get(headers.LogLevel) == "" {
		req.Header.Set(headers.LogLevel, createFunctionInvocationOptions.LogLevelName)
	}

	i.logger.InfoWithCtx(ctx,
		"Executing function",
		"method", createFunctionInvocationOptions.Method,
		"url", fullpath,
		"bodyLength", len(createFunctionInvocationOptions.Body),
		"headers", req.Header)

	response, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to send HTTP request")
	}

	defer response.Body.Close() // nolint: errcheck

	i.logger.InfoWithCtx(ctx, "Got response", "status", response.Status)

	// read the body
	responseBody, err := io.ReadAll(response.Body)
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
	createFunctionInvocationOptions *platform.CreateFunctionInvocationOptions) (string, error) {

	if createFunctionInvocationOptions.URL == "" {
		return "", errors.New("Invocation URL is required")
	}
	function := createFunctionInvocationOptions.FunctionInstance

	// validate given url, must match one of the function status invocation urls
	if !createFunctionInvocationOptions.SkipURLValidation {
		if !common.StringSliceContainsString(function.GetStatus().InvocationURLs(),
			createFunctionInvocationOptions.URL) {
			i.logger.WarnWithCtx(ctx,
				"Invocation URL does not match any of function status invocation urls",
				"url", createFunctionInvocationOptions.URL,
				"invocationURLs", function.GetStatus().InvocationURLs())
			return "", nuclio.NewErrBadRequest(fmt.Sprintf("Invalid function url %s",
				createFunctionInvocationOptions.URL))
		}
	}

	return strings.TrimRight(createFunctionInvocationOptions.URL, "/"), nil
}
