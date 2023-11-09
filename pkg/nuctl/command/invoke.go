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

package command

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/headers"
	nuctlcommon "github.com/nuclio/nuclio/pkg/nuctl/command/common"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/fatih/color"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/spf13/cobra"
)

type invokeCommandeer struct {
	cmd                             *cobra.Command
	rootCommandeer                  *RootCommandeer
	createFunctionInvocationOptions platform.CreateFunctionInvocationOptions
	invokeVia                       string
	externalIPAddresses             string
	timeout                         time.Duration
	contentType                     string
	headers                         string
	body                            string
	raiseOnStatus                   bool
}

func newInvokeCommandeer(ctx context.Context, rootCommandeer *RootCommandeer) *invokeCommandeer {
	commandeer := &invokeCommandeer{
		rootCommandeer: rootCommandeer,
	}

	cmd := &cobra.Command{
		Use:   "invoke function-name",
		Short: "Invoke a function",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error

			// if we got positional arguments
			if len(args) != 1 {
				return errors.New("Function invoke requires name")
			}

			// initialize root
			if err := rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			commandeer.createFunctionInvocationOptions.Name = args[0]
			commandeer.createFunctionInvocationOptions.Namespace = rootCommandeer.namespace

			// try parse body input from flag
			commandeer.createFunctionInvocationOptions.Body, err = commandeer.resolveBody()
			if err != nil {
				return errors.Wrap(err, "Failed to resolve body")
			}
			commandeer.createFunctionInvocationOptions.Headers = http.Header{}

			// resolve invocation method
			commandeer.createFunctionInvocationOptions.Method = commandeer.resolveMethod()

			// set external IP, if given
			if commandeer.externalIPAddresses != "" {
				if err := rootCommandeer.platform.SetExternalIPAddresses(strings.Split(commandeer.externalIPAddresses, ",")); err != nil {
					return errors.Wrap(err, "Failed to set external IP address")
				}
			}

			// set headers
			for headerName, headerValue := range common.StringToStringMap(commandeer.headers, "=") {
				commandeer.createFunctionInvocationOptions.Headers.Set(headerName, headerValue)
			}

			// populate content type
			if err := commandeer.populateContentType(); err != nil {
				return errors.Wrap(err, "Failed to populate content-type")
			}

			// verify correctness of logger level
			switch commandeer.createFunctionInvocationOptions.LogLevelName {
			case "none", "debug", "info", "warn", "error": // nolint: goconst
				break
			default:
				return errors.New("Invalid logger level name. Must be one of none / debug / info / warn / error")
			}

			// enrich request with function
			if err := commandeer.createFunctionInvocationOptions.EnrichFunction(ctx, rootCommandeer.platform); err != nil {
				return errors.Wrap(err, "Failed to enrich function invocation options")
			}

			// Implementation detail: first url is the intra-cluster url, following urls are external urls
			invocationURLs := commandeer.createFunctionInvocationOptions.FunctionInstance.GetStatus().InvocationURLs()
			if len(invocationURLs) == 0 {
				return errors.New("Function has no invocation URLs")
			}

			// convert via
			switch commandeer.invokeVia {
			case "any":

				// if running with platform, invoke internally
				if common.RunningInContainer() || common.IsInKubernetesCluster() {
					commandeer.createFunctionInvocationOptions.URL = invocationURLs[0]
					break
				}

				// default to external ip
				if err := commandeer.enrichOptionsForExternalIP(invocationURLs); err != nil {
					return errors.Wrap(err, "Failed to invoke via external IP")
				}

			case "external-ip", "loadbalancer":

				// unified behavior for BC.
				if err := commandeer.enrichOptionsForExternalIP(invocationURLs); err != nil {
					return errors.Wrap(err, "Failed to invoke via external IP")
				}
			default:
				return errors.Errorf(`Unknown invocation method %s. Must be one of "any", "external-ip", "loadbalancer"`,
					commandeer.invokeVia)
			}

			commandeer.createFunctionInvocationOptions.Timeout = commandeer.timeout
			invokeResult, err := rootCommandeer.platform.CreateFunctionInvocation(ctx,
				&commandeer.createFunctionInvocationOptions)
			if err != nil {
				return errors.Wrap(err, "Failed to invoke function")
			}

			// write the result to output
			return commandeer.outputInvokeResult(&commandeer.createFunctionInvocationOptions, invokeResult, cmd.OutOrStdout())
		},
	}

	cmd.Flags().StringVarP(&commandeer.contentType, "content-type", "c", "", "HTTP Content-Type")
	cmd.Flags().StringVarP(&commandeer.createFunctionInvocationOptions.Path, "path", "p", "", "Path to the function to invoke")
	cmd.Flags().StringVarP(&commandeer.createFunctionInvocationOptions.Method, "method", "m", "", "HTTP method for invoking the function")
	cmd.Flags().StringVarP(&commandeer.body, "body", "b", "", "HTTP message body")
	cmd.Flags().StringVarP(&commandeer.headers, "headers", "d", "", "HTTP headers (name=val1[,name=val2,...])")
	cmd.Flags().StringVarP(&commandeer.invokeVia, "via", "", "any", "Invoke the function via - \"any\": a load balancer or an external IP; \"loadbalancer\": a load balancer; \"external-ip\": an external IP")
	cmd.Flags().StringVarP(&commandeer.createFunctionInvocationOptions.LogLevelName, "log-level", "l", "info", "Log level - \"none\", \"debug\", \"info\", \"warn\", or \"error\"")
	cmd.Flags().StringVarP(&commandeer.externalIPAddresses, "external-ips", "", os.Getenv("NUCTL_EXTERNAL_IP_ADDRESSES"), "External IP addresses (comma-delimited) with which to invoke the function")
	cmd.Flags().DurationVarP(&commandeer.timeout, "timeout", "t", platformconfig.DefaultFunctionInvocationTimeoutSeconds*time.Second, "Invocation request timeout")
	cmd.Flags().BoolVarP(&commandeer.createFunctionInvocationOptions.SkipTLSVerification, "skip-tls", "", false, "Skip TLS verification")
	cmd.Flags().BoolVarP(&commandeer.raiseOnStatus, "raise-on-status", "", false, "Fail nuctl in case function invocation returns non-200 status code")

	commandeer.cmd = cmd

	return commandeer
}

func (i *invokeCommandeer) enrichOptionsForExternalIP(invocationURLs []string) error {
	i.createFunctionInvocationOptions.SkipURLValidation = true

	// provided external ip address,
	if i.externalIPAddresses != "" {

		// function has node port
		if functionNodePort := i.createFunctionInvocationOptions.
			FunctionInstance.
			GetStatus().
			HTTPPort; functionNodePort != 0 {
			externalIPAddresses, err := i.rootCommandeer.platform.GetExternalIPAddresses()
			if err != nil {
				return errors.Wrap(err, "Failed to get external IP addresses")
			}
			i.createFunctionInvocationOptions.URL = fmt.Sprintf("%s:%d", externalIPAddresses[0],
				functionNodePort)
		} else {
			return errors.New("Function has no node port and thus cannot be invoked externally " +
				"while providing external ip addresses")
		}
		return nil
	}

	if len(invocationURLs) < 2 {
		return errors.New("Function has no external invocation url")
	}

	// use last invocation url
	// implementation detail: first url is the intra-cluster url, following urls are external urls
	// the last url is the one that is most likely to be an ingress, if not, node port
	i.createFunctionInvocationOptions.URL = invocationURLs[len(invocationURLs)-1]

	// replace the host with the external ip address in case running from a container / cluster
	// in which case that host's ip address is not accessible within the docker network / k8s cluster
	if common.RunningInContainer() || common.IsInKubernetesCluster() {

		// parsing url requires us to add a scheme, adding one (it doesn't change the results)
		urlToParse := i.createFunctionInvocationOptions.URL
		if !strings.HasPrefix(urlToParse, "http") {
			urlToParse = "https://" + urlToParse
		}
		parsedURL, err := url.Parse(urlToParse)
		if err != nil {
			return errors.Wrap(err, "Failed to parse invocation URL")
		}

		if common.StringSliceContainsString(
			[]string{"localhost", "0.0.0.0", "127.0.0.1"}, parsedURL.Hostname()) {
			externalIPAddress, err := i.rootCommandeer.platform.GetExternalIPAddresses()
			if err != nil {
				return errors.Wrap(err, "Failed to get external IP addresses")
			}
			i.rootCommandeer.loggerInstance.DebugWith("Overriding external IP address",
				"currentExternalIPAddress", parsedURL.Hostname(),
				"overridingExternalIPAddress", externalIPAddress)
			i.createFunctionInvocationOptions.URL = fmt.Sprintf("%s:%s",
				externalIPAddress[0],
				parsedURL.Port(),
			)

		}
	}
	return nil
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

	// if the flag is set - fail in case function invocation returns non-200 status code
	if !(invokeResult.StatusCode >= http.StatusOK && invokeResult.StatusCode < 300) && i.raiseOnStatus {
		return errors.New("Function invocation failed")
	}

	return nil
}

// populateContentType populate from flag if given, header if given or default to resolve from body
func (i *invokeCommandeer) populateContentType() error {
	var contentTypes []string
	contentTypeHeaderName := "Content-Type"
	headers := i.createFunctionInvocationOptions.Headers

	// given as flag
	if i.contentType != "" {
		contentTypes = append(contentTypes, i.contentType)
	}

	// given as header
	if headers.Get(contentTypeHeaderName) != "" {

		// we want all values given
		contentTypes = append(contentTypes, headers.Values(contentTypeHeaderName)...)
	}

	// not given at all
	if len(contentTypes) == 0 {

		// try guess from body
		contentTypes = append(contentTypes, http.DetectContentType([]byte(i.body)))
	}

	// reset
	headers.Del(contentTypeHeaderName)

	// iterate over all content types and add
	for _, contentType := range contentTypes {
		parsedContentType, _, err := mime.ParseMediaType(contentType)
		if err != nil {
			return errors.Wrapf(err, "Failed to parse media type %s", contentType)
		}
		headers.Add(contentTypeHeaderName, parsedContentType)
	}
	return nil
}

func (i *invokeCommandeer) resolveBody() ([]byte, error) {

	// try resolve body from flag
	if i.body != "" {
		i.cmd.SetIn(bytes.NewBufferString(i.body))
	}

	// fallback to stdin
	return nuctlcommon.ReadFromInOrStdin(i.cmd.InOrStdin())
}

func (i *invokeCommandeer) resolveMethod() string {

	// if user did not specified method
	if i.createFunctionInvocationOptions.Method == "" {

		// user provided request body, default to POST
		if len(i.createFunctionInvocationOptions.Body) > 0 {
			return http.MethodPost
		}

		// In case of no body, default to GET
		return http.MethodGet

	}
	return i.createFunctionInvocationOptions.Method
}

func (i *invokeCommandeer) outputFunctionLogs(invokeResult *platform.CreateFunctionInvocationResult, writer io.Writer) error {

	// the function logs should return as JSON
	functionLogs := []map[string]interface{}{}

	// wrap the contents in [] so that it appears as a JSON array
	encodedFunctionLogs := invokeResult.Headers.Get(headers.Logs)

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
		nil,
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
	color.New(color.FgHiBlue).Fprintf(writer, "\n%s\n", "> Response headers:") // nolint: errcheck

	for headerName, headerValue := range invokeResult.Headers {

		// skip the log headers
		if strings.EqualFold(headerName, headers.Logs) {
			continue
		}

		fmt.Fprintf(writer, "%s = %s\n", headerName, headerValue[0]) // nolint: errcheck
	}

	return nil
}

func (i *invokeCommandeer) outputResponseBody(invokeResult *platform.CreateFunctionInvocationResult, writer io.Writer) error {
	var responseBodyString string

	// Print raw body
	color.New(color.FgHiBlue).Fprintf(writer, "\n%s\n", "> Response body:") // nolint: errcheck

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

	fmt.Fprintln(writer, responseBodyString) // nolint: errcheck

	return nil
}
