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
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/nuctl2/invoker"

	"github.com/spf13/cobra"
)

type invokeCommandeer struct {
	cmd            *cobra.Command
	rootCommandeer *RootCommandeer
	invokeOptions  platform.InvokeOptions
}

func newInvokeCommandeer(rootCommandeer *RootCommandeer) *invokeCommandeer {
	commandeer := &invokeCommandeer{
		rootCommandeer: rootCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "invoke function-name",
		Short:   "Invoke a function",
		RunE: func(cmd *cobra.Command, args []string) error {

			// if we got positional arguments
			if len(args) != 1 {
				return errors.New("Function invoke requires name")
			}

			// verify correctness of logger level
			switch commandeer.invokeOptions.LogLevelName {
			case "none", "debug", "info", "warn", "error":
				break
			default:
				return errors.New("Invalid logger level name. Must be one of none / debug / info / warn / error")
			}

			// set common
			commandeer.invokeOptions.Common = &rootCommandeer.commonOptions
			commandeer.invokeOptions.Common.Identifier = args[0]

			// initialize root
			platformInstance, logger, err := rootCommandeer.initialize()
			if err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			// create function execr and invoke
			functionExecutor, err := invoker.NewFunctionInvoker(logger, platformInstance)
			if err != nil {
				return errors.Wrap(err, "Failed to create function executor")
			}

			return functionExecutor.Invoke(&commandeer.invokeOptions, cmd.OutOrStdout())
		},
	}

	cmd.Flags().StringVarP(&commandeer.invokeOptions.ClusterIP, "cluster-ip", "i", "", "Remote cluster IP, will use kubeconf host address by default")
	cmd.Flags().StringVarP(&commandeer.invokeOptions.ContentType, "content-type", "c", "application/json", "HTTP Content Type")
	cmd.Flags().StringVarP(&commandeer.invokeOptions.URL, "url", "u", "", "invocation URL")
	cmd.Flags().StringVarP(&commandeer.invokeOptions.Method, "method", "m", "GET", "HTTP Method")
	cmd.Flags().StringVarP(&commandeer.invokeOptions.Body, "body", "b", "", "Message body")
	cmd.Flags().StringVarP(&commandeer.invokeOptions.Headers, "headers", "d", "", "HTTP headers (name=val1, ..)")
	cmd.Flags().StringVarP(&commandeer.invokeOptions.LogLevelName, "log-level", "l", "info", "One of none / debug / info / warn / error")

	commandeer.cmd = cmd

	return commandeer
}
