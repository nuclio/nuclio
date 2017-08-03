package command

import (
	"fmt"

	"github.com/nuclio/nuclio/pkg/nuclio-cli/getter"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type getCommandeer struct {
	cmd              *cobra.Command
	parentCommandeer *NuclioCLICommandeer
	getOptions       getter.Options
}

func newGetCommandeer(parentCommandeer *NuclioCLICommandeer) *getCommandeer {
	commandeer := getCommandeer{
		parentCommandeer: parentCommandeer,
	}

	cmd := &cobra.Command{
		Use:   "get resource-type [name[:version]] [-l selector] [-o text|wide|json|yaml] [--all-namespaces]",
		Short: "Display one or many resources",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceType := ""

			// if we got positional arguments
			if len(args) > 0 {
				if len(args) > 1 {

					// second argument is resource name
					commandeer.getOptions.ResourceIdentifier = args[1]
				}

				// first argument is resource type
				resourceType = args[0]

			} else {
				return fmt.Errorf("Missing resource type. One of: function")
			}

			if commandeer.getOptions.AllNamespaces {
				commandeer.parentCommandeer.commonOptions.Namespace = ""
			}

			// set common
			commandeer.getOptions.Common = &parentCommandeer.commonOptions

			// create logger
			logger, err := parentCommandeer.createLogger()
			if err != nil {
				return errors.Wrap(err, "Failed to create logger")
			}

			// execute the proper resource get
			switch resourceType {
			case "fu", "function":
				return getter.NewFunctionGetter(logger, commandeer.cmd.OutOrStdout()).Execute(&commandeer.getOptions)
			default:
				return fmt.Errorf(`Unknown resource type %s - try "function"`, args[0])
			}

			return nil
		},
	}

	cmd.PersistentFlags().BoolVar(&commandeer.getOptions.AllNamespaces, "all-namespaces", false, "Show resources from all namespaces")
	cmd.PersistentFlags().StringVarP(&commandeer.getOptions.Labels, "labels", "l", "", "Label selector (lbl1=val1,lbl2=val2..)")
	cmd.PersistentFlags().StringVarP(&commandeer.getOptions.Format, "output", "o", "text", "Output format - text|wide|yaml|json")
	cmd.PersistentFlags().BoolVarP(&commandeer.getOptions.Watch, "watch", "w", false, "Watch for changes")

	commandeer.cmd = cmd

	return &commandeer
}
