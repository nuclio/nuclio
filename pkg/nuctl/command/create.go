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

type createCommandeer struct {
	cmd            *cobra.Command
	rootCommandeer *RootCommandeer
}

func newCreateCommandeer(rootCommandeer *RootCommandeer) *createCommandeer {
	commandeer := &createCommandeer{
		rootCommandeer: rootCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"cre"},
		Short:   "Create resources",
	}

	createProjectCommand := newCreateProjectCommandeer(commandeer).cmd
	createFunctionEventCommand := newCreateFunctionEventCommandeer(commandeer).cmd

	cmd.AddCommand(
		createProjectCommand,
		createFunctionEventCommand,
	)

	commandeer.cmd = cmd

	return commandeer
}

type createProjectCommandeer struct {
	*createCommandeer
	projectConfig platform.ProjectConfig
}

func newCreateProjectCommandeer(createCommandeer *createCommandeer) *createProjectCommandeer {
	commandeer := &createProjectCommandeer{
		createCommandeer: createCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "project name",
		Aliases: []string{"proj"},
		Short:   "Create projects",
		RunE: func(cmd *cobra.Command, args []string) error {

			// if we got positional arguments
			if len(args) != 1 {
				return errors.New("Project create requires an identifier")
			}

			commandeer.projectConfig.Meta.Name = args[0]
			commandeer.projectConfig.Meta.Namespace = createCommandeer.rootCommandeer.namespace

			// initialize root
			if err := createCommandeer.rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			return createCommandeer.rootCommandeer.platform.CreateProject(&platform.CreateProjectOptions{
				ProjectConfig: commandeer.projectConfig,
			})
		},
	}

	cmd.Flags().StringVar(&commandeer.projectConfig.Spec.DisplayName, "display-name", "", "Project display name, if different than name")
	cmd.Flags().StringVar(&commandeer.projectConfig.Spec.Description, "description", "", "Project description")

	commandeer.cmd = cmd

	return commandeer
}

type createFunctionEventCommandeer struct {
	*createCommandeer
	functionEventConfig platform.FunctionEventConfig
	encodedAttributes   string
	functionName        string
}

func newCreateFunctionEventCommandeer(createCommandeer *createCommandeer) *createFunctionEventCommandeer {
	commandeer := &createFunctionEventCommandeer{
		createCommandeer: createCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "functionevent name",
		Aliases: []string{"fe"},
		Short:   "Create function events",
		RunE: func(cmd *cobra.Command, args []string) error {

			// if we got positional arguments
			if len(args) != 1 {
				return errors.New("Function event create requires an identifier")
			}

			if commandeer.functionName == "" {
				return errors.New("Function event must belong to a function")
			}

			commandeer.functionEventConfig.Meta.Name = args[0]
			commandeer.functionEventConfig.Meta.Namespace = createCommandeer.rootCommandeer.namespace
			commandeer.functionEventConfig.Meta.Labels = map[string]string{
				"nuclio.io/function-name": commandeer.functionName,
			}

			// initialize root
			if err := createCommandeer.rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			// decode the JSON attributes
			if err := json.Unmarshal([]byte(commandeer.encodedAttributes),
				&commandeer.functionEventConfig.Spec.Attributes); err != nil {
				return errors.Wrap(err, "Failed to decode function event attributes")
			}

			return createCommandeer.rootCommandeer.platform.CreateFunctionEvent(&platform.CreateFunctionEventOptions{
				FunctionEventConfig: commandeer.functionEventConfig,
			})
		},
	}

	cmd.Flags().StringVar(&commandeer.functionName, "function", "", "function this event belongs to")
	cmd.Flags().StringVar(&commandeer.functionEventConfig.Spec.DisplayName, "display-name", "", "display name, if different than name (optional)")
	cmd.Flags().StringVar(&commandeer.functionEventConfig.Spec.TriggerName, "trigger-name", "", "trigger name to invoke (optional)")
	cmd.Flags().StringVar(&commandeer.functionEventConfig.Spec.TriggerKind, "trigger-kind", "", "trigger kind to invoke (optional)")
	cmd.Flags().StringVar(&commandeer.functionEventConfig.Spec.Body, "body", "", "body content to invoke the function with")
	cmd.Flags().StringVar(&commandeer.encodedAttributes, "attrs", "{}", "JSON-encoded attributes for the function event")

	commandeer.cmd = cmd

	return commandeer
}
