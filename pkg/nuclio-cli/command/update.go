package command

import (
	"github.com/nuclio/nuclio/pkg/nuclio-cli/updater"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type updateCommandeer struct {
	cmd            *cobra.Command
	rootCommandeer *RootCommandeer
	updateOptions  updater.Options
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
}

func newUpdateFunctionCommandeer(updateCommandeer *updateCommandeer) *updateFunctionCommandeer {
	commandeer := &updateFunctionCommandeer{
		updateCommandeer: updateCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "function [name[:version]]",
		Aliases: []string{"fu"},
		Short:   "Update functions",
		RunE: func(cmd *cobra.Command, args []string) error {

			// if we got positional arguments
			if len(args) != 1 {
				return errors.New("Function update requires identifier")
			}

			// set common
			commandeer.updateOptions.Common = &updateCommandeer.rootCommandeer.commonOptions
			commandeer.updateOptions.Run.Common = &updateCommandeer.rootCommandeer.commonOptions
			commandeer.updateOptions.Common.Identifier = args[0]

			// create logger
			logger, err := updateCommandeer.rootCommandeer.createLogger()
			if err != nil {
				return errors.Wrap(err, "Failed to create logger")
			}

			// create function updater and execute
			functionUpdater, err := updater.NewFunctionUpdater(logger, &commandeer.updateOptions)
			if err != nil {
				return errors.Wrap(err, "Failed to create function updater")
			}

			return functionUpdater.Execute()
		},
	}

	// add run flags
	addRunFlags(cmd, &commandeer.updateOptions.Run)

	commandeer.cmd = cmd

	return commandeer
}
