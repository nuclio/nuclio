package command

import (
	"github.com/nuclio/nuclio/pkg/nuclio-cli/getter"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type getCommandeer struct {
	cmd              *cobra.Command
	rootCommandeer   *RootCommandeer
	getOptions       getter.Options
}

func newGetCommandeer(rootCommandeer *RootCommandeer) *getCommandeer {
	commandeer := &getCommandeer{
		rootCommandeer: rootCommandeer,
	}

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Display one or many resources",
	}

	cmd.PersistentFlags().BoolVar(&commandeer.getOptions.AllNamespaces, "all-namespaces", false, "Show resources from all namespaces")
	cmd.PersistentFlags().StringVarP(&commandeer.getOptions.Labels, "labels", "l", "", "Label selector (lbl1=val1,lbl2=val2..)")
	cmd.PersistentFlags().StringVarP(&commandeer.getOptions.Format, "output", "o", "text", "Output format - text|wide|yaml|json")
	cmd.PersistentFlags().BoolVarP(&commandeer.getOptions.Watch, "watch", "w", false, "Watch for changes")

	cmd.AddCommand(
		newGetFunctionCommandeer(commandeer).cmd,
	)

	commandeer.cmd = cmd

	return commandeer
}

type getFunctionCommandeer struct {
	*getCommandeer
}

func newGetFunctionCommandeer(getCommandeer *getCommandeer) *getFunctionCommandeer {
	commandeer := &getFunctionCommandeer{
		getCommandeer: getCommandeer,
	}

	cmd := &cobra.Command{
		Use:   "function [name[:version]] [-l selector] [-o text|wide|json|yaml] [--all-namespaces]",
		Short: "Display one or many functions",
		RunE: func(cmd *cobra.Command, args []string) error {

			// if we got positional arguments
			if len(args) != 0 {

				// second argument is resource name
				commandeer.getOptions.ResourceIdentifier = args[0]
			}

			if commandeer.getOptions.AllNamespaces {
				getCommandeer.rootCommandeer.commonOptions.Namespace = ""
			}

			// set common
			commandeer.getOptions.Common = &getCommandeer.rootCommandeer.commonOptions

			// create logger
			logger, err := getCommandeer.rootCommandeer.createLogger()
			if err != nil {
				return errors.Wrap(err, "Failed to create logger")
			}

			// create function getter and execute
			functionGetter, err := getter.NewFunctionGetter(logger, commandeer.cmd.OutOrStdout(), &commandeer.getOptions)
			if err != nil {
				return errors.Wrap(err, "Failed to create function getter")
			}

			return functionGetter.Execute()
		},
	}

	commandeer.cmd = cmd

	return commandeer
}
