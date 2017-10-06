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

package kube

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/mgutz/ansi"
	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/platform"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type invoker struct {
	logger            nuclio.Logger
	invokeOptions     *platform.InvokeOptions
	kubeCommonOptions *CommonOptions
	consumer          *consumer
	platform          platform.Platform
}

func newInvoker(parentLogger nuclio.Logger, platform platform.Platform) (*invoker, error) {
	newinvoker := &invoker{
		logger:   parentLogger.GetChild("invoker").(nuclio.Logger),
		platform: platform,
	}

	return newinvoker, nil
}

func (i *invoker) invoke(consumer *consumer, invokeOptions *platform.InvokeOptions, writer io.Writer) error {

	// save options
	i.invokeOptions = invokeOptions
	i.kubeCommonOptions = invokeOptions.Common.Platform.(*CommonOptions)
	i.consumer = consumer

	functioncrInstance, err := consumer.functioncrClient.Get(i.invokeOptions.Common.Namespace,
		invokeOptions.Common.Identifier)

	if err != nil {
		return errors.Wrap(err, "Failed to get function custom resource")
	}

	functionService, err := consumer.clientset.CoreV1().
		Services(functioncrInstance.Namespace).
		Get(functioncrInstance.Name, meta_v1.GetOptions{})

	if err != nil {
		return errors.Wrap(err, "Failed to get function service")
	}

	if invokeOptions.ClusterIP == "" {
		var kubeURL *url.URL

		kubeURL, err = url.Parse(consumer.kubeHost)
		if err == nil && kubeURL.Host != "" {
			invokeOptions.ClusterIP = strings.Split(kubeURL.Host, ":")[0]
		}
	}

	port := strconv.Itoa(int(functionService.Spec.Ports[0].NodePort))

	fullpath := "http://" + invokeOptions.ClusterIP + ":" + port + "/" + invokeOptions.URL

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
		i.outputFunctionLogs(response)
	}

	// output the headers
	i.outputResponseHeaders(response)

	// output the boy
	i.outputResponseBody(response)

	return nil
}

func (i *invoker) outputFunctionLogs(response *http.Response) error {

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
	functionLogger, err := nucliozap.NewNuclioZapCmd(i.invokeOptions.Common.Identifier, nucliozap.DebugLevel)
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

func (i *invoker) outputResponseHeaders(response *http.Response) error {
	fmt.Printf("\n%s\n", ansi.Color("> Response headers:", "blue+h"))

	for headerName, headerValue := range response.Header {

		// skip the log headers
		if strings.ToLower(headerName) == strings.ToLower("X-Nuclio-Logs") {
			continue
		}

		fmt.Printf("%s = %s\n", headerName, headerValue[0])
	}

	return nil
}

func (i *invoker) outputResponseBody(response *http.Response) error {
	var responseBodyString string

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}

	// Print raw body
	fmt.Printf("\n%s\n", ansi.Color("> Response body:", "blue+h"))

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

	fmt.Println(responseBodyString)

	return nil
}
