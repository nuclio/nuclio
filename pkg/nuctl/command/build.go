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
	"fmt"
	"os"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/spf13/cobra"
)

type buildCommandeer struct {
	cmd            *cobra.Command
	rootCommandeer *RootCommandeer
	commands       stringSliceFlag
	functionConfig functionconfig.Config
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

			// if we got positional arguments
			switch len(args) {
			case 0:
				return fmt.Errorf("Missing function path")
			case 1: /* noop */
			default:
				return fmt.Errorf("Too many arguments")
			}

			// update build stuff
			commandeer.functionConfig.Meta.Name = args[0]
			commandeer.functionConfig.Meta.Namespace = rootCommandeer.namespace
			commandeer.functionConfig.Spec.Build.Commands = commandeer.commands

			// initialize root
			if err := rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			_, err := rootCommandeer.platform.BuildFunction(&platform.BuildOptions{
				Logger:         rootCommandeer.logger,
				FunctionConfig: commandeer.functionConfig,
			})
			return err
		},
	}

	addBuildFlags(cmd, &commandeer.functionConfig, &commandeer.commands)

	commandeer.cmd = cmd

	return commandeer
}

func addBuildFlags(cmd *cobra.Command, config *functionconfig.Config, commands *stringSliceFlag) {
	cmd.Flags().StringVarP(&config.Spec.Build.Path, "path", "p", "", "Function source code path")
	cmd.Flags().StringVarP(&config.Spec.Build.FunctionConfigPath, "file", "f", "", "Function configuration file")
	cmd.Flags().StringVarP(&config.Spec.Build.ImageName, "image", "i", "", "Docker image name, will use function name if not specified")
	cmd.Flags().StringVar(&config.Spec.Build.ImageVersion, "version", "latest", "Docker image version")
	cmd.Flags().StringVarP(&config.Spec.Build.OutputType, "output", "o", "docker", "Build output type - docker|binary")
	cmd.Flags().StringVarP(&config.Spec.Build.Registry, "registry", "r", os.Getenv("NUCTL_REGISTRY"), "URL of container registry (env: NUCTL_REGISTRY)")
	cmd.Flags().StringVar(&config.Spec.Build.NuclioSourceDir, "nuclio-src-dir", "", "Local directory with nuclio sources (avoid cloning)")
	cmd.Flags().StringVar(&config.Spec.Build.NuclioSourceURL, "nuclio-src-url", "https://github.com/nuclio/nuclio.git", "nuclio sources url for git clone")
	cmd.Flags().StringVarP(&config.Spec.Runtime, "runtime", "", "", "Runtime (e.g. golang, golang:1.8, python:2.7)")
	cmd.Flags().StringVarP(&config.Spec.Handler, "handler", "", "", "Name of handler")
	cmd.Flags().BoolVarP(&config.Spec.Build.NoBaseImagesPull, "no-pull", "", false, "Don't pull base images - use local versions")
	cmd.Flags().StringVarP(&config.Spec.Build.BaseImageName, "base-image", "", "", "Name of base image. If empty, per-runtime default is used")
	cmd.Flags().Var(commands, "build-command", "Commands to run on build of processor image")
}
