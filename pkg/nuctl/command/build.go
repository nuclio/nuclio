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
	"os"

	"github.com/nuclio/nuclio/pkg/nuctl/builder"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type buildCommandeer struct {
	cmd            *cobra.Command
	rootCommandeer *RootCommandeer
	buildOptions   builder.Options
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
			if len(args) != 1 {
				return errors.New("Function build requires name")
			}

			// set common
			commandeer.buildOptions.Common = &rootCommandeer.commonOptions
			commandeer.buildOptions.Path = args[0]

			// create logger
			logger, err := rootCommandeer.createLogger()
			if err != nil {
				return errors.Wrap(err, "Failed to create logger")
			}

			// create function buildr and execute
			functionBuilder, err := builder.NewFunctionBuilder(logger, &commandeer.buildOptions)
			if err != nil {
				return errors.Wrap(err, "Failed to create function builder")
			}

			return functionBuilder.Execute()
		},
	}

	addBuildFlags(cmd, &commandeer.buildOptions)

	commandeer.cmd = cmd

	return commandeer
}

func addBuildFlags(cmd *cobra.Command, options *builder.Options) {
	cmd.Flags().StringVarP(&options.Path, "path", "p", "", "Function source code path")
	cmd.Flags().StringVarP(&options.ImageName, "image", "i", "", "Container image to use, will use function name if not specified")
	cmd.Flags().StringVarP(&options.OutputType, "output", "o", "docker", "Build output type - docker|binary")
	cmd.Flags().StringVarP(&options.PushRegistry, "registry", "r", os.Getenv("PUSH_REGISTRY"), "URL of container registry (for push)")
	cmd.Flags().StringVar(&options.NuclioSourceDir, "nuclio-src-dir", "", "Local directory with nuclio sources (avoid cloning)")
	cmd.Flags().StringVar(&options.NuclioSourceURL, "nuclio-src-url", "https://github.com/nuclio/nuclio.git", "nuclio sources url for git clone")
}
