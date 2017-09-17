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
	"github.com/nuclio/nuclio/pkg/nuctl/deleter"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/spf13/cobra"
)

type deleteCommandeer struct {
	cmd            *cobra.Command
	rootCommandeer *RootCommandeer
	deleteOptions  deleter.Options
}

func newDeleteCommandeer(rootCommandeer *RootCommandeer) *deleteCommandeer {
	commandeer := &deleteCommandeer{
		rootCommandeer: rootCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"del"},
		Short:   "Delete a resource",
	}

	cmd.AddCommand(
		newDeleteFunctionCommandeer(commandeer).cmd,
	)

	commandeer.cmd = cmd

	return commandeer
}

type deleteFunctionCommandeer struct {
	*deleteCommandeer
}

func newDeleteFunctionCommandeer(deleteCommandeer *deleteCommandeer) *deleteFunctionCommandeer {
	commandeer := &deleteFunctionCommandeer{
		deleteCommandeer: deleteCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "function [name[:version]]",
		Aliases: []string{"fu"},
		Short:   "Delete functions",
		RunE: func(cmd *cobra.Command, args []string) error {

			// if we got positional arguments
			if len(args) != 1 {
				return errors.New("Function delete requires identifier")
			}

			// set common
			commandeer.deleteOptions.Common = &deleteCommandeer.rootCommandeer.commonOptions
			commandeer.deleteOptions.Common.Identifier = args[0]

			// create logger
			logger, err := deleteCommandeer.rootCommandeer.createLogger()
			if err != nil {
				return errors.Wrap(err, "Failed to create logger")
			}

			// create function deleter and execute
			functionDeleter, err := deleter.NewFunctionDeleter(logger, &commandeer.deleteOptions)
			if err != nil {
				return errors.Wrap(err, "Failed to create function deleter")
			}

			return functionDeleter.Execute()
		},
	}

	commandeer.cmd = cmd

	return commandeer
}
