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
	"github.com/nuclio/nuclio/pkg/functioncr"
	"github.com/nuclio/nuclio/pkg/nuctl/runner"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type runCommandeer struct {
	cmd            *cobra.Command
	rootCommandeer *RootCommandeer
	runOptions     runner.Options
}

func newRunCommandeer(rootCommandeer *RootCommandeer) *runCommandeer {
	commandeer := &runCommandeer{
		rootCommandeer: rootCommandeer,
	}

	cmd := &cobra.Command{
		Use:   "run function-name",
		Short: "Build, deploy and run a function",
		RunE: func(cmd *cobra.Command, args []string) error {
			functionName := ""

			// if the spec path was set, load the spec
			if commandeer.runOptions.SpecPath != "" {
				err := functioncr.FromSpecFile(commandeer.runOptions.SpecPath, &commandeer.runOptions.Spec)
				if err != nil {
					return errors.Wrap(err, "Failed to read spec file")
				}
			}

			// name can either be a positional argument or passed in the spec
			if len(args) != 1 {
				if commandeer.runOptions.Spec.ObjectMeta.Name == "" {
					return errors.New("Function run requires name")
				}

				// use name from spec
				functionName = commandeer.runOptions.Spec.ObjectMeta.Name

			} else {
				functionName = args[0]
			}

			// function can either be in the path or received inline
			if commandeer.runOptions.Build.Path == "" && commandeer.runOptions.Spec.Spec.Code.Inline == "" {
				return errors.New("Function code must be provided either in path or inline in a spec file")
			}

			if commandeer.runOptions.Build.PushRegistry == "" {
				return errors.New("Push registry is required")
			}

			// set common
			commandeer.runOptions.Build.Common = &rootCommandeer.commonOptions
			commandeer.runOptions.Common = &rootCommandeer.commonOptions
			commandeer.runOptions.Common.Identifier = functionName

			// create logger
			logger, err := rootCommandeer.createLogger()
			if err != nil {
				return errors.Wrap(err, "Failed to create logger")
			}

			// create function runner and execute
			functionRunner, err := runner.NewFunctionRunner(logger, &commandeer.runOptions)
			if err != nil {
				return errors.Wrap(err, "Failed to create function runner")
			}

			return functionRunner.Execute()
		},
	}

	addRunFlags(cmd, &commandeer.runOptions)

	commandeer.cmd = cmd

	return commandeer
}

func addRunFlags(cmd *cobra.Command, options *runner.Options) {
	addBuildFlags(cmd, &options.Build)

	cmd.Flags().StringVarP(&options.SpecPath, "file", "f", "", "Function Spec File")
	cmd.Flags().StringVar(&options.Description, "desc", "", "Function description")
	cmd.Flags().StringVarP(&options.Scale, "scale", "s", "1", "Function scaling (auto|number)")
	cmd.Flags().StringVarP(&options.Labels, "labels", "l", "", "Additional function labels (lbl1=val1,lbl2=val2..)")
	cmd.Flags().StringVarP(&options.Env, "env", "e", "", "Environment variables (name1=val1,name2=val2..)")
	cmd.Flags().StringVar(&options.Events, "events", "", "Comma separated list of event sources (in json)")
	cmd.Flags().StringVar(&options.Data, "data", "", "Comma separated list of data bindings (in json)")
	cmd.Flags().BoolVarP(&options.Disabled, "disabled", "d", false, "Start function disabled (don't run yet)")
	cmd.Flags().Int32Var(&options.HTTPPort, "port", 0, "Public HTTP port (node port)")
	cmd.Flags().Int32Var(&options.MinReplicas, "min-replica", 0, "Minimum number of function replicas")
	cmd.Flags().Int32Var(&options.MaxReplicas, "max-replica", 0, "Maximum number of function replicas")
	cmd.Flags().BoolVar(&options.Publish, "publish", false, "Publish the function")
	cmd.Flags().StringVar(&options.DataBindings, "data-bindings", "", "JSON encoded data bindings for the function")
}
