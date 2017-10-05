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
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/nuclio/pkg/nuctl2/builder"
	"github.com/spf13/cobra"
)

type buildCommandeer struct {
	cmd            *cobra.Command
	rootCommandeer *RootCommandeer
	buildOptions   platform.BuildOptions
}

func newBuildCommandeer(rootCommandeer *RootCommandeer) *buildCommandeer {
	commandeer := &buildCommandeer{
		rootCommandeer: rootCommandeer,
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

			// set common
			commandeer.buildOptions.Common = &rootCommandeer.commonOptions
			commandeer.buildOptions.Common.Identifier = args[0]

			// initialize root
			platformInstance, logger, err := rootCommandeer.initialize()
			if err != nil {
				return errors.Wrap(err, "Failed to initialize")
			}

			// create function buildr and execute
			functionBuilder, err := builder.NewFunctionBuilder(logger, platformInstance)
			if err != nil {
				return errors.Wrap(err, "Failed to create function builder")
			}

			_, err = functionBuilder.Build(&commandeer.buildOptions)
			return err
		},
	}

	addBuildFlags(cmd, &commandeer.buildOptions)

	commandeer.cmd = cmd

	return commandeer
}

func addBuildFlags(cmd *cobra.Command, options *platform.BuildOptions) {
	cmd.Flags().StringVarP(&options.Path, "path", "p", "", "Function source code path")
	cmd.Flags().StringVarP(&options.ImageName, "image", "i", "", "Docker image name, will use function name if not specified")
	cmd.Flags().StringVar(&options.ImageVersion, "version", "latest", "Docker image version")
	cmd.Flags().StringVarP(&options.OutputType, "output", "o", "docker", "Build output type - docker|binary")
	cmd.Flags().StringVarP(&options.Registry, "registry", "r", os.Getenv("NUCTL_REGISTRY"), "URL of container registry (env: NUCTL_REGISTRY)")
	cmd.Flags().StringVar(&options.NuclioSourceDir, "nuclio-src-dir", "", "Local directory with nuclio sources (avoid cloning)")
	cmd.Flags().StringVar(&options.NuclioSourceURL, "nuclio-src-url", "https://github.com/nuclio/nuclio.git", "nuclio sources url for git clone")
	cmd.Flags().StringVarP(&options.Runtime, "runtime", "", "", "Runtime - one of golang/python")
	cmd.Flags().BoolVarP(&options.NoBaseImagesPull, "no-pull", "", false, "Don't pull base images - use local versions")
}
