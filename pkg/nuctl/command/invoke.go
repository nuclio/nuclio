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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/mgutz/ansi"
	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/spf13/cobra"
)

type invokeCommandeer struct {
	cmd                             *cobra.Command
	rootCommandeer                  *RootCommandeer
	createFunctionInvocationOptions platform.CreateFunctionInvocationOptions
	invokeVia                       string
	contentType                     string
	headers                         string
	body                            string
}

func newInvokeCommandeer(rootCommandeer *RootCommandeer) *invokeCommandeer {
	commandeer := &invokeCommandeer{
		rootCommandeer: rootCommandeer,
	}

	cmd := &cobra.Command{
		Use:   "invoke function-name",
		Short: "Invoke a function",
		RunE: func(cmd *cobra.Command, args []string) error {

			// if we got positional arguments
			if len(args) != 1 {
				return errors.New("Function invoke requires name")
			}

			commandeer.createFunctionInvocationOptions.Name = args[0]
			commandeer.createFunctionInvocationOptions.Namespace = rootCommandeer.namespace
			commandeer.createFunctionInvocationOptions.Body = []byte(commandeer.body)
			commandeer.createFunctionInvocationOptions.Headers = http.Header{}

			// set headers
			for headerName, headerValue := range common.StringToStringMap(commandeer.headers, "=") {
				commandeer.createFunctionInvocationOptions.Headers.Set(headerName, headerValue)
			}

			commandeer.createFunctionInvocationOptions.Headers.Set("Content-Type", commandeer.contentType)

			// verify correctness of logger level
			switch commandeer.createFunctionInvocationOptions.LogLevelName {
			case "none", "debug", "info", "warn", "error":
				break
			default:
				return errors.New("Invalid logger level name. Must be one of none / debug / info / warn / error")
			}

			// convert via
			switch commandeer.invokeVia {
			case "any":
				commandeer.createFunctionInvocationOptions.Via = platform.InvokeViaAny
			case "external-ip":
				commandeer.createFunctionInvocationOptions.Via = platform.InvokeViaExternalIP
			case "loadbalancer":
				commandeer.createFunctionInvocationOptions.Via = platform.InvokeViaLoadBalancer
			default:
				return errors.New("Invalid via type - must be ingress / nodePort")
			}

			// initialize root
			if err := rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			invokeResult, err := rootCommandeer.platform.CreateFunctionInvocation(&commandeer.createFunctionInvocationOptions)
			if err != nil {
				return errors.Wrap(err, "Failed to invoke function")
			}

			// write the result to output
			return commandeer.outputInvokeResult(&commandeer.createFunctionInvocationOptions, invokeResult, cmd.OutOrStdout())
		},
	}

	cmd.Flags().StringVarP(&commandeer.contentType, "content-type", "c", "application/json", "HTTP Content-Type")
	cmd.Flags().StringVarP(&commandeer.createFunctionInvocationOptions.Path, "path", "p", "", "Path to the function to invoke")
	cmd.Flags().StringVarP(&commandeer.createFunctionInvocationOptions.Method, "method", "m", "GET", "HTTP method for invoking the function")
	cmd.Flags().StringVarP(&commandeer.body, "body", "b", "", "HTTP message body")
	cmd.Flags().StringVarP(&commandeer.headers, "headers", "d", "", "HTTP headers (name=val1[,name=val2,...])")
	cmd.Flags().StringVarP(&commandeer.invokeVia, "via", "", "any", "Invoke the function via - \"any\": a load balancer or an external IP; \"loadbalancer\": a load balancer; \"external-ip\": an external IP")
	cmd.Flags().StringVarP(&commandeer.createFunctionInvocationOptions.LogLevelName, "log-level", "l", "info", "Log level - \"none\", \"debug\", \"info\", \"warn\", or \"error\"")

	commandeer.cmd = cmd

	return commandeer
}

func (i *invokeCommandeer) outputInvokeResult(createFunctionInvocationOptions *platform.CreateFunctionInvocationOptions,
	invokeResult *platform.CreateFunctionInvocationResult,
	writer io.Writer) error {

	// try to output the logs (ignore errors)
	if createFunctionInvocationOptions.LogLevelName != "none" {
		if err := i.outputFunctionLogs(invokeResult, writer); err != nil {
			return errors.Wrap(err, "Failed to output logs")
		}
	}

	// output the headers
	if err := i.outputResponseHeaders(invokeResult, writer); err != nil {
		return errors.Wrap(err, "Failed to output headers")
	}

	// output the body
	if err := i.outputResponseBody(invokeResult, writer); err != nil {
		return errors.Wrap(err, "Failed to output body")
	}

	return nil
}

func (i *invokeCommandeer) outputFunctionLogs(invokeResult *platform.CreateFunctionInvocationResult, writer io.Writer) error {

	// the function logs should return as JSON
	functionLogs := []map[string]interface{}{}

	// wrap the contents in [] so that it appears as a JSON array
	encodedFunctionLogs := invokeResult.Headers.Get("x-nuclio-logs")

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
	functionLogger, err := nucliozap.NewNuclioZap(i.createFunctionInvocationOptions.Name,
		"console",
		writer,
		writer,
		nucliozap.DebugLevel)

	if err != nil {
		return errors.Wrap(err, "Failed to create function logger")
	}

	i.rootCommandeer.loggerInstance.Info(">>> Start of function logs")

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
		i.rootCommandeer.loggerInstance.Info("<<< End of function logs")
	}

	return nil
}

func (i *invokeCommandeer) stringInterfaceMapToInterfaceSlice(input map[string]interface{}) []interface{} {
	output := []interface{}{}

	// convert the map to a flat slice of interfaces
	for argName, argValue := range input {
		output = append(output, argName)
		output = append(output, argValue)
	}

	return output
}

func (i *invokeCommandeer) getOutputByLevelName(logger logger.Logger, levelName string) func(interface{}, ...interface{}) {
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

func (i *invokeCommandeer) outputResponseHeaders(invokeResult *platform.CreateFunctionInvocationResult, writer io.Writer) error {
	fmt.Fprintf(writer, "\n%s\n", ansi.Color("> Response headers:", "blue+h"))

	for headerName, headerValue := range invokeResult.Headers {

		// skip the log headers
		if strings.ToLower(headerName) == strings.ToLower("X-Nuclio-Logs") {
			continue
		}

		fmt.Fprintf(writer, "%s = %s\n", headerName, headerValue[0])
	}

	return nil
}

func (i *invokeCommandeer) outputResponseBody(invokeResult *platform.CreateFunctionInvocationResult, writer io.Writer) error {
	var responseBodyString string

	// Print raw body
	fmt.Fprintf(writer, "\n%s\n", ansi.Color("> Response body:", "blue+h"))

	// check if response is json
	if invokeResult.Headers.Get("Content-Type") == "application/json" {
		var indentedBody bytes.Buffer

		err := json.Indent(&indentedBody, invokeResult.Body, "", "    ")
		if err != nil {
			responseBodyString = string(invokeResult.Body)
		} else {
			responseBodyString = indentedBody.String()
		}

	} else {
		responseBodyString = string(invokeResult.Body)
	}

	fmt.Fprintln(writer, responseBodyString)

	return nil
}
