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
	"context"
	"encoding/json"
	"os"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/errors"
	"github.com/spf13/cobra"
)

type buildCommandeer struct {
	cmd                        *cobra.Command
	rootCommandeer             *RootCommandeer
	commands                   stringSliceFlag
	functionConfig             functionconfig.Config
	functionConfigPath         string
	runtime                    string
	handler                    string
	encodedRuntimeAttributes   string
	encodedCodeEntryAttributes string
	outputImageFile            string
}

func newBuildCommandeer(rootCommandeer *RootCommandeer) *buildCommandeer {
	commandeer := &buildCommandeer{
		rootCommandeer: rootCommandeer,
		functionConfig: *functionconfig.NewConfig(),
	}

	cmd := &cobra.Command{
		Use:     "build function-name [options]",
		Aliases: []string{"bu"},
		Short:   "Build a function",
		RunE: func(cmd *cobra.Command, args []string) error {

			// update build stuff
			if len(args) == 1 {
				commandeer.functionConfig.Meta.Name = args[0]
			}

			// initialize root
			if err := rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			commandeer.functionConfig.Meta.Namespace = rootCommandeer.namespace
			commandeer.functionConfig.Spec.Build.Commands = commandeer.commands
			commandeer.functionConfig.Spec.Build.FunctionConfigPath = commandeer.functionConfigPath
			commandeer.functionConfig.Spec.Runtime = commandeer.runtime
			commandeer.functionConfig.Spec.Handler = commandeer.handler

			// decode the JSON build runtime attributes
			if err := json.Unmarshal([]byte(commandeer.encodedRuntimeAttributes),
				&commandeer.functionConfig.Spec.Build.RuntimeAttributes); err != nil {
				return errors.Wrap(err, "Failed to decode build runtime attributes")
			}

			if commandeer.functionConfig.Spec.Build.Offline {
				rootCommandeer.loggerInstance.Debug("Offline flag is passed, setting no-pull as well")
				commandeer.functionConfig.Spec.Build.NoBaseImagesPull = true
			}

			// decode the JSON build code entry attributes
			if err := json.Unmarshal([]byte(commandeer.encodedCodeEntryAttributes),
				&commandeer.functionConfig.Spec.Build.CodeEntryAttributes); err != nil {
				return errors.Wrap(err, "Failed to decode code entry attributes")
			}

			_, err := rootCommandeer.platform.CreateFunctionBuild(
				context.Background(),
				&platform.CreateFunctionBuildOptions{
					Logger:          rootCommandeer.loggerInstance,
					FunctionConfig:  commandeer.functionConfig,
					PlatformName:    rootCommandeer.platform.GetName(),
					OutputImageFile: commandeer.outputImageFile,
				})
			return err
		},
	}

	addBuildFlags(cmd, &commandeer.functionConfig.Spec.Build, &commandeer.functionConfigPath, &commandeer.runtime, &commandeer.handler, &commandeer.commands, &commandeer.encodedRuntimeAttributes, &commandeer.encodedCodeEntryAttributes)
	cmd.Flags().StringVarP(&commandeer.outputImageFile, "output-image-file", "", "", "Path to output container image of the build")

	commandeer.cmd = cmd

	return commandeer
}

func addBuildFlags(cmd *cobra.Command, functionBuild *functionconfig.Build, functionConfigPath *string, runtime *string, handler *string, commands *stringSliceFlag, encodedRuntimeAttributes *string, encodedCodeEntryAttributes *string) { // nolint
	cmd.Flags().StringVarP(&functionBuild.Path, "path", "p", "", "Path to the function's source code")
	cmd.Flags().StringVarP(&functionBuild.FunctionSourceCode, "source", "", "", "The function's source code (overrides \"path\")")
	cmd.Flags().StringVarP(functionConfigPath, "file", "f", "", "Path to a function-configuration file")
	cmd.Flags().StringVarP(&functionBuild.Image, "image", "i", "", "Name of a container image (default - the function name)")
	cmd.Flags().StringVarP(&functionBuild.Registry, "registry", "r", os.Getenv("NUCTL_REGISTRY"), "URL of a container registry (env: NUCTL_REGISTRY)")
	cmd.Flags().StringVarP(runtime, "runtime", "", "", "Runtime (for example, \"golang\", \"python:3.9\")")
	cmd.Flags().StringVarP(handler, "handler", "", "", "Name of a function handler")
	cmd.Flags().BoolVarP(&functionBuild.NoBaseImagesPull, "no-pull", "", false, "Don't pull base images - use local versions")
	cmd.Flags().BoolVarP(&functionBuild.NoCleanup, "no-cleanup", "", false, "Don't clean up temporary directories")
	cmd.Flags().StringVarP(&functionBuild.BaseImage, "base-image", "", "", "Name of the base image (default - per-runtime default)")
	cmd.Flags().Var(commands, "build-command", "Commands to run when building the processor image")
	cmd.Flags().StringVarP(&functionBuild.OnbuildImage, "onbuild-image", "", "", "The runtime onbuild image used to build the processor image")
	cmd.Flags().BoolVarP(&functionBuild.Offline, "offline", "", false, "Don't assume internet connectivity exists")
	cmd.Flags().StringVar(encodedRuntimeAttributes, "build-runtime-attrs", "{}", "JSON-encoded build runtime attributes for the function")
	cmd.Flags().StringVar(encodedCodeEntryAttributes, "build-code-entry-attrs", "{}", "JSON-encoded build code entry attributes for the function")
	cmd.Flags().StringVar(&functionBuild.CodeEntryType, "code-entry-type", "", "Type of code entry (for example, \"url\", \"github\", \"image\")")
}
