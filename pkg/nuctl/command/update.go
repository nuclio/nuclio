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
	"encoding/json"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/spf13/cobra"
)

type updateCommandeer struct {
	cmd            *cobra.Command
	rootCommandeer *RootCommandeer
	updateOptions  *platform.UpdateOptions
	commands       stringSliceFlag
}

func newUpdateCommandeer(rootCommandeer *RootCommandeer) *updateCommandeer {
	commandeer := &updateCommandeer{
		rootCommandeer: rootCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "update",
		Aliases: []string{"upd"},
		Short:   "Update a resource",
	}

	cmd.AddCommand(
		newUpdateFunctionCommandeer(commandeer).cmd,
	)

	commandeer.cmd = cmd

	return commandeer
}

type updateFunctionCommandeer struct {
	*updateCommandeer
	encodedDataBindings string
	encodedTriggers     string
	encodedIngresses    string
}

func newUpdateFunctionCommandeer(updateCommandeer *updateCommandeer) *updateFunctionCommandeer {
	commandeer := &updateFunctionCommandeer{
		updateCommandeer: updateCommandeer,
	}

	commandeer.updateOptions = platform.NewUpdateOptions(updateCommandeer.rootCommandeer.commonOptions)

	cmd := &cobra.Command{
		Use:     "function [name[:version]]",
		Aliases: []string{"fu"},
		Short:   "Update functions",
		RunE: func(cmd *cobra.Command, args []string) error {

			// if we got positional arguments
			if len(args) != 1 {
				return errors.New("Function update requires identifier")
			}

			// decode the JSON data bindings
			if err := json.Unmarshal([]byte(commandeer.encodedDataBindings),
				&commandeer.updateOptions.Deploy.DataBindings); err != nil {
				return errors.Wrap(err, "Failed to decode data bindings")
			}

			// decode the JSON triggers
			if err := json.Unmarshal([]byte(commandeer.encodedTriggers),
				&commandeer.updateOptions.Deploy.Triggers); err != nil {
				return errors.Wrap(err, "Failed to decode triggers")
			}

			// decode the JSON ingresses
			if err := json.Unmarshal([]byte(commandeer.encodedIngresses),
				&commandeer.updateOptions.Deploy.Ingresses); err != nil {
				return errors.Wrap(err, "Failed to decode ingresses")
			}

			// update build stuff
			commandeer.updateOptions.Deploy.Build.Commands = commandeer.commands

			// initialize root
			if err := updateCommandeer.rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			return updateCommandeer.rootCommandeer.platform.UpdateFunction(commandeer.updateOptions)
		},
	}

	// add run flags
	addDeployFlags(cmd,
		&commandeer.updateOptions.Deploy,
		&commandeer.commands,
		&commandeer.encodedDataBindings,
		&commandeer.encodedTriggers,
		&commandeer.encodedIngresses)

	commandeer.cmd = cmd

	return commandeer
}
