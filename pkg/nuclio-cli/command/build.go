package command

import (
	"os"

	"github.com/nuclio/nuclio/pkg/nuclio-cli/builder"

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
			commandeer.buildOptions.Common.Identifier = args[0]

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
	cmd.Flags().StringVar(&options.NuclioSourceURL, "nuclio-src-url", "git@github.com:nuclio/nuclio.git", "nuclio sources url for git clone")
}
