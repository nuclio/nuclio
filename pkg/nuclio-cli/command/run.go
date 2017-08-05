package command

import (
	"github.com/nuclio/nuclio/pkg/nuclio-cli/runner"

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
		Use:     "run function-name",
		Aliases: []string{"bu"},
		Short:   "Build, deploy and run a function",
		RunE: func(cmd *cobra.Command, args []string) error {

			// if we got positional arguments
			if len(args) != 1 {
				return errors.New("Function run requires name")
			}

			if commandeer.runOptions.Build.Path == "" {
				return errors.New("Path is required")
			}

			if commandeer.runOptions.Build.PushRegistry == "" {
				return errors.New("Push registry is required")
			}

			// second argument is resource name
			commandeer.runOptions.Build.Name = args[0]

			// set common
			commandeer.runOptions.Build.Common = &rootCommandeer.commonOptions
			commandeer.runOptions.Common = &rootCommandeer.commonOptions

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
	cmd.Flags().StringVar(&options.Events, "events", "", "Comma seperated list of event sources (in json)")
	cmd.Flags().StringVar(&options.Data, "data", "", "Comma seperated list of data bindings (in json)")
	cmd.Flags().BoolVarP(&options.Disabled, "disabled", "d", false, "Start function disabled (don't run yet)")
	cmd.Flags().Int32Var(&options.HTTPPort, "port", 0, "Public HTTP port (node port)")
	cmd.Flags().Int32Var(&options.MinReplicas, "min-replica", 0, "Minimum number of function replicas")
	cmd.Flags().Int32Var(&options.MaxReplicas, "max-replica", 0, "Maximum number of function replicas")
	cmd.Flags().BoolVar(&options.Publish, "publish", false, "Publish the function")
}
