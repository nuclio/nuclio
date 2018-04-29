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
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/logger"
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

func (i *invoker) invoke(createFunctionInvocationOptions *platform.CreateFunctionInvocationOptions) (*platform.CreateFunctionInvocationResult, error) {

	// save options
	i.createFunctionInvocationOptions = createFunctionInvocationOptions

	// get the function by name
	functions, err := i.platform.GetFunctions(&platform.GetFunctionsOptions{
		Name:      createFunctionInvocationOptions.Name,
		Namespace: createFunctionInvocationOptions.Namespace,
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get functions")
	}

	if len(functions) == 0 {
		return nil, fmt.Errorf("Function not found: %s @ %s", createFunctionInvocationOptions.Name, createFunctionInvocationOptions.Namespace)
	}

	// use the first function found (should always be one, but if there's more just use first)
	function := functions[0]

	// make sure to initialize the function (some underlying functions are lazy load)
	if err = function.Initialize(nil); err != nil {
		return nil, errors.Wrap(err, "Failed to initialize function")
	}

	// get where the function resides
	invokeURL, err := function.GetInvokeURL(createFunctionInvocationOptions.Via)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get invoke URL")
	}

	fullpath := "http://" + invokeURL
	if createFunctionInvocationOptions.Path != "" {
		fullpath += "/" + createFunctionInvocationOptions.Path
	}

	client := &http.Client{}
	var req *http.Request
	var body io.Reader = http.NoBody

	// set body for post
	if createFunctionInvocationOptions.Method != "GET" {
		body = bytes.NewBuffer(createFunctionInvocationOptions.Body)
	}

	i.logger.InfoWith("Executing function",
		"method", createFunctionInvocationOptions.Method,
		"url", fullpath,
		"body", body,
	)

	// issue the request
	req, err = http.NewRequest(createFunctionInvocationOptions.Method, fullpath, body)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create HTTP request")
	}

	// set headers
	req.Header = createFunctionInvocationOptions.Headers

	// request logs from a given verbosity unless we're specified no logs should be returned
	if createFunctionInvocationOptions.LogLevelName != "none" {
		req.Header.Set("x-nuclio-log-level", createFunctionInvocationOptions.LogLevelName)
	}

	response, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to send HTTP request")
	}

	defer response.Body.Close() // nolint: errcheck

	i.logger.InfoWith("Got response", "status", response.Status)

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
