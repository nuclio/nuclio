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

package executor

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

	"github.com/mgutz/ansi"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/nuctl"
	"github.com/nuclio/nuclio/pkg/util/common"
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/nuclio/nuclio-sdk"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FunctionExecutor struct {
	logger       nuclio.Logger
	options      *Options
	kubeConsumer *nuctl.KubeConsumer
}

func NewFunctionExecutor(parentLogger nuclio.Logger) (*FunctionExecutor, error) {
	newFunctionExecutor := &FunctionExecutor{
		logger: parentLogger.GetChild("executor").(nuclio.Logger),
	}

	return newFunctionExecutor, nil
}

func (fe *FunctionExecutor) Execute(kubeConsumer *nuctl.KubeConsumer, options *Options, writer io.Writer) error {

	// save options, consumer
	fe.options = options
	fe.kubeConsumer = kubeConsumer

	functioncrInstance, err := fe.kubeConsumer.FunctioncrClient.Get(options.Common.Namespace, options.Common.Identifier)
	if err != nil {
		return errors.Wrap(err, "Failed to get function custom resource")
	}

	functionService, err := fe.kubeConsumer.Clientset.CoreV1().
		Services(functioncrInstance.Namespace).
		Get(functioncrInstance.Name, meta_v1.GetOptions{})

	if err != nil {
		return errors.Wrap(err, "Failed to get function service")
	}

	if options.ClusterIP == "" {
		var kubeURL *url.URL

		kubeURL, err = url.Parse(fe.kubeConsumer.KubeHost)
		if err == nil && kubeURL.Host != "" {
			options.ClusterIP = strings.Split(kubeURL.Host, ":")[0]
		}
	}

	port := strconv.Itoa(int(functionService.Spec.Ports[0].NodePort))

	fullpath := "http://" + options.ClusterIP + ":" + port + "/" + options.URL

	client := &http.Client{}
	var req *http.Request
	var body io.Reader = http.NoBody

	// set body for post
	if options.Method == "POST" {
		body = bytes.NewBuffer([]byte(options.Body))
	}

	fe.logger.InfoWith("Executing function",
		"method", options.Method,
		"url", fullpath,
		"body", body,
	)

	// issue the request
	req, err = http.NewRequest(options.Method, fullpath, body)
	if err != nil {
		return errors.Wrap(err, "Failed to create HTTP request")
	}

	req.Header.Set("Content-Type", options.ContentType)

	// request logs from a given verbosity unless we're specified no logs should be returned
	if options.LogLevelName != "none" {
		req.Header.Set("X-nuclio-log-level", options.LogLevelName)
	}

	headers := common.StringToStringMap(options.Headers)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	response, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "Failed to send HTTP request")
	}

	defer response.Body.Close()

	fe.logger.InfoWith("Got response", "status", response.Status)

	// try to output the logs (ignore errors)
	if options.LogLevelName != "none" {
		fe.outputFunctionLogs(response)
	}

	// output the headers
	fe.outputResponseHeaders(response)

	// output the boy
	fe.outputResponseBody(response)

	return nil
}

func (fe *FunctionExecutor) outputFunctionLogs(response *http.Response) error {

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
	functionLogger, err := nucliozap.NewNuclioZapCmd(fe.options.Common.Identifier, nucliozap.DebugLevel)
	if err != nil {
		return errors.Wrap(err, "Failed to create function logger")
	}

	fe.logger.Info(">>> Start of function logs")

	// iterate through all the logs
	for _, functionLog := range functionLogs {
		message := functionLog["message"].(string)
		levelName := functionLog["level"].(string)
		delete(functionLog, "message")
		delete(functionLog, "level")
		delete(functionLog, "name")

		// convert args map to a slice of interfaces
		args := fe.stringInterfaceMapToInterfaceSlice(functionLog)

		// output to log by level
		fe.getOutputByLevelName(functionLogger, levelName)(message, args...)
	}

	if len(functionLogs) != 0 {
		fe.logger.Info("<<< End of function logs")
	}

	return nil
}

func (fe *FunctionExecutor) stringInterfaceMapToInterfaceSlice(input map[string]interface{}) []interface{} {
	output := []interface{}{}

	// convert the map to a flat slice of interfaces
	for argName, argValue := range input {
		output = append(output, argName)
		output = append(output, argValue)
	}

	return output
}

func (fe *FunctionExecutor) getOutputByLevelName(logger nuclio.Logger, levelName string) func(interface{}, ...interface{}) {
	switch levelName {
	case "info":
		return fe.logger.InfoWith
	case "warn":
		return fe.logger.WarnWith
	case "error":
		return fe.logger.ErrorWith
	default:
		return fe.logger.DebugWith
	}
}

func (fe *FunctionExecutor) outputResponseHeaders(response *http.Response) error {
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

func (fe *FunctionExecutor) outputResponseBody(response *http.Response) error {
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
