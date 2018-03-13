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

	cmd.AddCommand(
		newCreateProjectCommandeer(commandeer).cmd,
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
