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

	"github.com/nuclio/nuclio/pkg/nuctl"
	"github.com/nuclio/nuclio/pkg/util/common"

	"github.com/mgutz/ansi"
	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/zap"
	"github.com/pkg/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FunctionExecutor struct {
	nucliocli.KubeConsumer
	logger  nuclio.Logger
	writer  io.Writer
	options *Options
}

func NewFunctionExecutor(parentLogger nuclio.Logger,
	writer io.Writer,
	options *Options) (*FunctionExecutor, error) {
	var err error
	kubeHost := ""

	newFunctionExecutor := &FunctionExecutor{
		logger:  parentLogger.GetChild("executor").(nuclio.Logger),
		writer:  writer,
		options: options,
	}

	// get kube stuff
	kubeHost, err = newFunctionExecutor.GetClients(newFunctionExecutor.logger, options.Common.KubeconfigPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get clients")
	}

	// update kubehost if not set
	if options.Common.KubeHost == "" {
		options.Common.KubeHost = kubeHost
	}

	return newFunctionExecutor, nil
}

func (fe *FunctionExecutor) Execute() error {
	functioncrInstance, err := fe.FunctioncrClient.Get(fe.options.Common.Namespace, fe.options.Common.Identifier)
	if err != nil {
		return errors.Wrap(err, "Failed to get function custom resource")
	}

	functionService, err := fe.Clientset.CoreV1().Services(functioncrInstance.Namespace).Get(functioncrInstance.Name, meta_v1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "Failed to get function service")
	}

	if fe.options.ClusterIP == "" {
		url, err := url.Parse(fe.options.Common.KubeHost)
		if err == nil && url.Host != "" {
			fe.options.ClusterIP = strings.Split(url.Host, ":")[0]
		}
	}

	port := strconv.Itoa(int(functionService.Spec.Ports[0].NodePort))

	fullpath := "http://" + fe.options.ClusterIP + ":" + port + "/" + fe.options.Url

	client := &http.Client{}
	var req *http.Request
	var body io.Reader = http.NoBody

	// set body for post
	if fe.options.Method == "POST" {
		body = bytes.NewBuffer([]byte(fe.options.Body))
	}

	fe.logger.InfoWith("Executing function",
		"method", fe.options.Method,
		"url", fullpath,
		"body", body,
	)

	// issue the request
	req, err = http.NewRequest(fe.options.Method, fullpath, body)
	if err != nil {
		return errors.Wrap(err, "Failed to create HTTP request")
	}

	req.Header.Set("Content-Type", fe.options.ContentType)
	req.Header.Set("X-nuclio-log-level", fe.options.LogLevelName)
	headers := common.StringToStringMap(fe.options.Headers)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	response, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "Failed to send HTTP request")
	}

	defer response.Body.Close()

	fe.logger.InfoWith("Got response", "status", response.Status)

	// try to output the logs (ignore errors
	fe.outputFunctionLogs(response)

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
	encodedFunctionLogs := "[" + response.Header.Get("X-nuclio-logs") + "]"

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
		return logger.InfoWith
	case "warn":
		return logger.WarnWith
	case "error":
		return logger.ErrorWith
	default:
		return logger.DebugWith
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

	htmlData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}

	// Print raw body
	fmt.Printf("\n%s\n", ansi.Color("> Response body:", "blue+h"))
	fmt.Println(string(htmlData))

	return nil
}
