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
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/nuctl2/getter"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/spf13/cobra"
)

type getCommandeer struct {
	cmd            *cobra.Command
	rootCommandeer *RootCommandeer
	getOptions     platform.GetOptions
}

func newGetCommandeer(rootCommandeer *RootCommandeer) *getCommandeer {
	commandeer := &getCommandeer{
		rootCommandeer: rootCommandeer,
	}

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Display one or many resources",
	}

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
		Use:     "function [name[:version]]",
		Aliases: []string{"fu"},
		Short:   "Display one or many functions",
		RunE: func(cmd *cobra.Command, args []string) error {

			// set common
			commandeer.getOptions.Common = &getCommandeer.rootCommandeer.commonOptions

			// if we got positional arguments
			if len(args) != 0 {

				// second argument is resource name
				commandeer.getOptions.Common.Identifier = args[0]
			}

			// create logger
			logger, err := getCommandeer.rootCommandeer.createLogger()
			if err != nil {
				return errors.Wrap(err, "Failed to create logger")
			}

			// initialize root
			platformInstance, logger, err := getCommandeer.rootCommandeer.initialize()
			if err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			// create function getter and execute
			functionGetter, err := getter.NewFunctionGetter(logger, platformInstance)
			if err != nil {
				return errors.Wrap(err, "Failed to create function getter")
			}

			return functionGetter.Get(&commandeer.getOptions, commandeer.cmd.OutOrStdout())
		},
	}

	commandeer.cmd = cmd

	return commandeer
}
