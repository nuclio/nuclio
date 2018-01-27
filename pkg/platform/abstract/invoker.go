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
	logger        logger.Logger
	platform      platform.Platform
	invokeOptions *platform.InvokeOptions
}

func newInvoker(parentLogger logger.Logger, platform platform.Platform) (*invoker, error) {
	newinvoker := &invoker{
		logger:   parentLogger.GetChild("invoker"),
		platform: platform,
	}

	return newinvoker, nil
}

func (i *invoker) invoke(invokeOptions *platform.InvokeOptions) (*platform.InvokeResult, error) {

	// save options
	i.invokeOptions = invokeOptions

	// get the function by name
	functions, err := i.platform.GetFunctions(&platform.GetOptions{
		Name:      invokeOptions.Name,
		Namespace: invokeOptions.Namespace,
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get functions")
	}

	if len(functions) == 0 {
		return nil, fmt.Errorf("Function not found: %s @ %s", invokeOptions.Name, invokeOptions.Namespace)
	}

	// use the first function found (should always be one, but if there's more just use first)
	function := functions[0]

	// make sure to initialize the function (some underlying functions are lazy load)
	if err = function.Initialize(nil); err != nil {
		return nil, errors.Wrap(err, "Failed to initialize function")
	}

	// get where the function resides
	invokeURL, err := function.GetInvokeURL(invokeOptions.Via)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get invoke URL")
	}

	fullpath := "http://" + invokeURL
	if invokeOptions.Path != "" {
		fullpath += "/" + invokeOptions.Path
	}

	client := &http.Client{}
	var req *http.Request
	var body io.Reader = http.NoBody

	// set body for post
	if invokeOptions.Method == "POST" {
		body = bytes.NewBuffer(invokeOptions.Body)
	}

	i.logger.InfoWith("Executing function",
		"method", invokeOptions.Method,
		"url", fullpath,
		"body", body,
	)

	// issue the request
	req, err = http.NewRequest(invokeOptions.Method, fullpath, body)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create HTTP request")
	}

	// request logs from a given verbosity unless we're specified no logs should be returned
	if invokeOptions.LogLevelName != "none" {
		req.Header.Set("X-nuclio-log-level", invokeOptions.LogLevelName)
	}

	// set headers
	req.Header = invokeOptions.Headers

	response, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to send HTTP request")
	}

	defer response.Body.Close()

	i.logger.InfoWith("Got response", "status", response.Status)

	// read the body
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read response body")
	}

	return &platform.InvokeResult{
		Headers:    response.Header,
		Body:       responseBody,
		StatusCode: response.StatusCode,
	}, nil
}
