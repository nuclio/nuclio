package command

import (
	"github.com/nuclio/nuclio/pkg/nuclio-cli/deleter"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type deleteCommandeer struct {
	cmd            *cobra.Command
	rootCommandeer *RootCommandeer
	deleteOptions  deleter.Options
}

func newDeleteCommandeer(rootCommandeer *RootCommandeer) *deleteCommandeer {
	commandeer := &deleteCommandeer{
		rootCommandeer: rootCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"del"},
		Short:   "Delete a resource",
	}

	cmd.AddCommand(
		newDeleteFunctionCommandeer(commandeer).cmd,
	)

	commandeer.cmd = cmd

	return commandeer
}

type deleteFunctionCommandeer struct {
	*deleteCommandeer
}

func newDeleteFunctionCommandeer(deleteCommandeer *deleteCommandeer) *deleteFunctionCommandeer {
	commandeer := &deleteFunctionCommandeer{
		deleteCommandeer: deleteCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "function [name[:version]]",
		Aliases: []string{"fu"},
		Short:   "Delete functions",
		RunE: func(cmd *cobra.Command, args []string) error {

			// if we got positional arguments
			if len(args) != 1 {
				return errors.New("Function delete requires identifier")
			}

			// second argument is resource name
			commandeer.deleteOptions.ResourceIdentifier = args[0]

			// set common
			commandeer.deleteOptions.Common = &deleteCommandeer.rootCommandeer.commonOptions

			// create logger
			logger, err := deleteCommandeer.rootCommandeer.createLogger()
			if err != nil {
				return errors.Wrap(err, "Failed to create logger")
			}

			// create function deleter and execute
			functionDeleter, err := deleter.NewFunctionDeleter(logger, &commandeer.deleteOptions)
			if err != nil {
				return errors.Wrap(err, "Failed to create function deleter")
			}

			return functionDeleter.Execute()
		},
	}

	commandeer.cmd = cmd

	return commandeer
}
