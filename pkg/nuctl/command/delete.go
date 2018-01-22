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
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/spf13/cobra"
)

type deleteCommandeer struct {
	cmd            *cobra.Command
	rootCommandeer *RootCommandeer
}

func newDeleteCommandeer(rootCommandeer *RootCommandeer) *deleteCommandeer {
	commandeer := &deleteCommandeer{
		rootCommandeer: rootCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"del"},
		Short:   "Delete resources",
	}

	cmd.AddCommand(
		newDeleteFunctionCommandeer(commandeer).cmd,
	)

	commandeer.cmd = cmd

	return commandeer
}

type deleteFunctionCommandeer struct {
	*deleteCommandeer
	platform.DeleteOptions
}

func newDeleteFunctionCommandeer(deleteCommandeer *deleteCommandeer) *deleteFunctionCommandeer {
	commandeer := &deleteFunctionCommandeer{
		deleteCommandeer: deleteCommandeer,
		DeleteOptions:    platform.DeleteOptions{},
	}

	cmd := &cobra.Command{
		Use:     "function [name[:version]] [name[:version]] ...",
		Aliases: []string{"fu"},
		Short:   "Delete functions",
		RunE: func(cmd *cobra.Command, args []string) error {

			// Alert if no arguments were given and flag --all wasn't up
			if len(args) == 0 && !commandeer.DeleteOptions.All {
				return errors.New("Function delete requires an identifier. Delete all by raising flag --all")
			}

			// For every argument append commandeer configurations with name and namespace of new arg
			for argIndex, arg := range args {
				commandeer.DeleteOptions.FunctionConfigs = append(commandeer.DeleteOptions.FunctionConfigs, *functionconfig.NewConfig())

				// Add the function to commandeer.functionConfigs
				commandeer.DeleteOptions.FunctionConfigs[argIndex].Meta.Name = arg
				commandeer.DeleteOptions.FunctionConfigs[argIndex].Meta.Namespace = deleteCommandeer.rootCommandeer.namespace
			}

			// if flag --all raised ignore given args and send DeleteFunctions functionConfig with name ""
			if commandeer.DeleteOptions.All {
				globalConfig := *functionconfig.NewConfig()
				globalConfig.Meta.Namespace = deleteCommandeer.rootCommandeer.namespace
				commandeer.DeleteOptions.FunctionConfigs = []functionconfig.Config{globalConfig}
			}

			// initialize root
			if err := deleteCommandeer.rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			return deleteCommandeer.rootCommandeer.platform.DeleteFunctions(&platform.DeleteOptions{
				FunctionConfigs: commandeer.DeleteOptions.FunctionConfigs,
			})
		},
	}

	// set flag for delete -all option
	cmd.PersistentFlags().BoolVarP(&commandeer.DeleteOptions.All, "all", "a", false, "Delete all functions")

	commandeer.cmd = cmd

	return commandeer
}
