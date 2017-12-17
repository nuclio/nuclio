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
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/mgutz/ansi"
	"github.com/nuclio/nuclio-sdk"
)

type invoker struct {
	logger        nuclio.Logger
	platform      platform.Platform
	invokeOptions *platform.InvokeOptions
}

func newInvoker(parentLogger nuclio.Logger, platform platform.Platform) (*invoker, error) {
	newinvoker := &invoker{
		logger:   parentLogger.GetChild("invoker"),
		platform: platform,
	}

	return newinvoker, nil
}

func (i *invoker) invoke(invokeOptions *platform.InvokeOptions, writer io.Writer) error {

	// save options
	i.invokeOptions = invokeOptions

	// get the function by name
	functions, err := i.platform.GetFunctions(&platform.GetOptions{
		Name:      invokeOptions.Name,
		Namespace: invokeOptions.Namespace,
	})

	if len(functions) == 0 {
		return errors.Wrap(err, "Function not found")
	}

	// use the first function found (should always be one, but if there's more just use first)
	function := functions[0]

	// make sure to initialize the function (some underlying functions are lazy load)
	if err = function.Initialize(nil); err != nil {
		return errors.Wrap(err, "Failed to initialize function")
	}

	// get where the function resides
	invokeURL, err := function.GetInvokeURL(invokeOptions.Via)
	if err != nil {
		return errors.Wrap(err, "Failed to get invoke URL")
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
		body = bytes.NewBuffer([]byte(invokeOptions.Body))
	}

	i.logger.InfoWith("Executing function",
		"method", invokeOptions.Method,
		"url", fullpath,
		"body", body,
	)

	// issue the request
	req, err = http.NewRequest(invokeOptions.Method, fullpath, body)
	if err != nil {
		return errors.Wrap(err, "Failed to create HTTP request")
	}

	req.Header.Set("Content-Type", invokeOptions.ContentType)

	// request logs from a given verbosity unless we're specified no logs should be returned
	if invokeOptions.LogLevelName != "none" {
		req.Header.Set("X-nuclio-log-level", invokeOptions.LogLevelName)
	}

	headers := common.StringToStringMap(invokeOptions.Headers)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	response, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "Failed to send HTTP request")
	}

	defer response.Body.Close()

	i.logger.InfoWith("Got response", "status", response.Status)

	// try to output the logs (ignore errors)
	if invokeOptions.LogLevelName != "none" {
		i.outputFunctionLogs(response, writer)
	}

	// output the headers
	i.outputResponseHeaders(response, writer)

	// output the boy
	i.outputResponseBody(response, writer)

	return nil
}

func (i *invoker) outputFunctionLogs(response *http.Response, writer io.Writer) error {

	// the function logs should return as JSON
	functionLogs := []map[string]interface{}{}

	// wrap the contents in [] so that it appears as a JSON array
	encodedFunctionLogs := response.Header.Get("X-nuclio-logs")

	// parse the JSON into function logs
	err := json.Unmarshal([]byte(encodedFunctionLogs), &functionLogs)
	if err != nil {
		return errors.Wrap(err, "Failed to parse logs")
	}

	// if there are no logs, return now
	if len(functionLogs) == 0 {
		return nil
	}

	// create a logger whose name is that of the function and whose severity was chosen by command line
	// arguments during invocation
	functionLogger, err := nucliozap.NewNuclioZap(i.invokeOptions.Name,
		"console",
		writer,
		writer,
		nucliozap.DebugLevel)

	if err != nil {
		return errors.Wrap(err, "Failed to create function logger")
	}

	i.logger.Info(">>> Start of function logs")

	// iterate through all the logs
	for _, functionLog := range functionLogs {
		message := functionLog["message"].(string)
		levelName := functionLog["level"].(string)
		delete(functionLog, "message")
		delete(functionLog, "level")
		delete(functionLog, "name")

		// convert args map to a slice of interfaces
		args := i.stringInterfaceMapToInterfaceSlice(functionLog)

		// output to log by level
		i.getOutputByLevelName(functionLogger, levelName)(message, args...)
	}

	if len(functionLogs) != 0 {
		i.logger.Info("<<< End of function logs")
	}

	return nil
}

func (i *invoker) stringInterfaceMapToInterfaceSlice(input map[string]interface{}) []interface{} {
	output := []interface{}{}

	// convert the map to a flat slice of interfaces
	for argName, argValue := range input {
		output = append(output, argName)
		output = append(output, argValue)
	}

	return output
}

func (i *invoker) getOutputByLevelName(logger nuclio.Logger, levelName string) func(interface{}, ...interface{}) {
	switch levelName {
	case "info":
		return i.logger.InfoWith
	case "warn":
		return i.logger.WarnWith
	case "error":
		return i.logger.ErrorWith
	default:
		return i.logger.DebugWith
	}
}

func (i *invoker) outputResponseHeaders(response *http.Response, writer io.Writer) error {
	fmt.Fprintf(writer, "\n%s\n", ansi.Color("> Response headers:", "blue+h"))

	for headerName, headerValue := range response.Header {

		// skip the log headers
		if strings.ToLower(headerName) == strings.ToLower("X-Nuclio-Logs") {
			continue
		}

		fmt.Fprintf(writer, "%s = %s\n", headerName, headerValue[0])
	}

	return nil
}

func (i *invoker) outputResponseBody(response *http.Response, writer io.Writer) error {
	var responseBodyString string

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}

	// Print raw body
	fmt.Fprintf(writer, "\n%s\n", ansi.Color("> Response body:", "blue+h"))

	// check if response is json
	if response.Header.Get("Content-Type") == "application/json" {
		var indentedBody bytes.Buffer

		err = json.Indent(&indentedBody, responseBody, "", "    ")
		if err != nil {
			responseBodyString = string(responseBody)
		} else {
			responseBodyString = indentedBody.String()
		}

	} else {
		responseBodyString = string(responseBody)
	}

	fmt.Fprintln(writer, responseBodyString)

	return nil
}
