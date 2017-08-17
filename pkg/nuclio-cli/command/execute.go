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
	"github.com/nuclio/nuclio/pkg/nuclio-cli/executor"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type executeCommandeer struct {
	cmd            *cobra.Command
	rootCommandeer *RootCommandeer
	executeOptions executor.Options
}

func newExecuteCommandeer(rootCommandeer *RootCommandeer) *executeCommandeer {
	commandeer := &executeCommandeer{
		rootCommandeer: rootCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "execute function-name",
		Aliases: []string{"exec"},
		Short:   "Execute a function",
		RunE: func(cmd *cobra.Command, args []string) error {

			// if we got positional arguments
			if len(args) != 1 {
				return errors.New("Function exec requires name")
			}

			// set common
			commandeer.executeOptions.Common = &rootCommandeer.commonOptions
			commandeer.executeOptions.Common.Identifier = args[0]

			// create logger
			logger, err := rootCommandeer.createLogger()
			if err != nil {
				return errors.Wrap(err, "Failed to create logger")
			}

			// create function execr and execute
			functionExecutor, err := executor.NewFunctionExecutor(logger, cmd.OutOrStdout(), &commandeer.executeOptions)
			if err != nil {
				return errors.Wrap(err, "Failed to create function executor")
			}

			return functionExecutor.Execute()
		},
	}

	cmd.Flags().StringVarP(&commandeer.executeOptions.ClusterIP, "cluster-ip", "i", "", "Remote cluster IP, will use kubeconf host address by default")
	cmd.Flags().StringVarP(&commandeer.executeOptions.ContentType, "content-type", "c", "application/json", "HTTP Content Type")
	cmd.Flags().StringVarP(&commandeer.executeOptions.Url, "url", "u", "", "invocation URL")
	cmd.Flags().StringVarP(&commandeer.executeOptions.Method, "method", "m", "GET", "HTTP Method")
	cmd.Flags().StringVarP(&commandeer.executeOptions.Body, "body", "b", "", "Message body")
	cmd.Flags().StringVarP(&commandeer.executeOptions.Headers, "headers", "d", "", "HTTP headers (name=val1, ..)")

	commandeer.cmd = cmd

	return commandeer
}
