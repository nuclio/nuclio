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
	"io"
	"strconv"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/renderer"

	"github.com/spf13/cobra"
)

const (
	outputFormatText = "text"
	outputFormatWide = "wide"
)

type getCommandeer struct {
	cmd            *cobra.Command
	rootCommandeer *RootCommandeer
}

func newGetCommandeer(rootCommandeer *RootCommandeer) *getCommandeer {
	commandeer := &getCommandeer{
		rootCommandeer: rootCommandeer,
	}

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Display resource information",
	}

	cmd.AddCommand(
		newGetFunctionCommandeer(commandeer).cmd,
	)

	commandeer.cmd = cmd

	return commandeer
}

type getFunctionCommandeer struct {
	*getCommandeer
	getOptions platform.GetOptions
}

func newGetFunctionCommandeer(getCommandeer *getCommandeer) *getFunctionCommandeer {
	commandeer := &getFunctionCommandeer{
		getCommandeer: getCommandeer,
		getOptions:    platform.GetOptions{MatchCriterias: []platform.MatchCriteria{}},
	}

	cmd := &cobra.Command{
		Use:     "function [name[:version]] [name[:version]] ...",
		Aliases: []string{"fu"},
		Short:   "Display functions information",
		RunE: func(cmd *cobra.Command, args []string) error {
			commandeer.getOptions.Namespace = getCommandeer.rootCommandeer.namespace
			commandeer.getOptions.MatchCriterias = []platform.MatchCriteria{}

			// check if there were given args, if so append commandeer.getOptions.MatchCriterias accordingly
			if len(args) != 0 {

				// remove duplicated arguments
				args = commandeer.removeDuplicates(args)

				// update commandeer's MatchCriteria according to given args
				for argIndex, arg := range args {
					commandeer.getOptions.MatchCriterias = append(commandeer.getOptions.MatchCriterias, platform.MatchCriteria{})
					commandeer.getOptions.MatchCriterias[argIndex].Name = arg
				}
			} else {

				// if no arg was given, append with empty criteria to show all functions available
				commandeer.getOptions.MatchCriterias = append(commandeer.getOptions.MatchCriterias, platform.MatchCriteria{})
			}

			// initialize root
			if err := getCommandeer.rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			// try get functions described in commandeer.getOption
			functions, err := getCommandeer.rootCommandeer.platform.GetFunctions(&commandeer.getOptions)
			if err != nil {
				return errors.Wrap(err, "Failed to get functions")
			}

			// alert if no functions found
			if len(functions) == 0 {
				cmd.OutOrStdout().Write([]byte("No functions found"))
				return nil
			}

			// render the functions, if error occurs return it, else return nil
			return commandeer.renderFunctions(functions, commandeer.getOptions.Format, cmd.OutOrStdout())
		},
	}

	// Make flags for get function: Labels, format and watch
	cmd.PersistentFlags().StringVarP(&commandeer.getOptions.Labels, "labels", "l", "", "Function labels (lbl1=val1[,lbl2=val2,...])")
	cmd.PersistentFlags().StringVarP(&commandeer.getOptions.Format, "output", "o", outputFormatText, "Output format - \"text\", \"wide\", \"yaml\", or \"json\"")
	cmd.PersistentFlags().BoolVarP(&commandeer.getOptions.Watch, "watch", "w", false, "Watch for changes")

	commandeer.cmd = cmd

	return commandeer
}

func (g *getFunctionCommandeer) renderFunctions(functions []platform.Function, format string, writer io.Writer) error {

	// iterate over each function and make sure it's initialized
	// TODO: parallelize
	for _, function := range functions {
		if err := function.Initialize(nil); err != nil {
			return err
		}
	}

	rendererInstance := renderer.NewRenderer(writer)

	switch format {
	case outputFormatText, outputFormatWide:
		header := []string{"Namespace", "Name", "Version", "State", "Node Port", "Replicas"}
		if format == outputFormatWide {
			header = append(header, []string{
				"Labels",
				"Ingresses",
			}...)
		}

		functionRecords := [][]string{}

		// for each field
		for _, function := range functions {
			availableReplicas, specifiedReplicas := function.GetReplicas()

			// get its fields
			functionFields := []string{
				function.GetConfig().Meta.Namespace,
				function.GetConfig().Meta.Name,
				function.GetVersion(),
				function.GetState(),
				strconv.Itoa(function.GetConfig().Spec.HTTPPort),
				fmt.Sprintf("%d/%d", availableReplicas, specifiedReplicas),
			}

			// add fields for wide view
			if format == outputFormatWide {
				functionFields = append(functionFields, []string{
					common.StringMapToString(function.GetConfig().Meta.Labels),
					g.formatFunctionIngresses(function),
				}...)
			}

			// add to records
			functionRecords = append(functionRecords, functionFields)
		}

		rendererInstance.RenderTable(header, functionRecords)
	case "yaml":
		g.renderFunctionConfig(functions, rendererInstance.RenderYAML)
	case "json":
		g.renderFunctionConfig(functions, rendererInstance.RenderJSON)
	}

	return nil
}

func (g *getFunctionCommandeer) formatFunctionIngresses(function platform.Function) string {
	var formattedIngresses string

	ingresses := function.GetIngresses()

	for _, ingress := range ingresses {
		host := ingress.Host
		if host != "" {
			host += ":<port>"
		}

		for _, path := range ingress.Paths {
			formattedIngresses += fmt.Sprintf("%s%s, ", host, path)
		}
	}

	// add default ingress
	formattedIngresses += fmt.Sprintf("/%s/%s",
		function.GetConfig().Meta.Name,
		function.GetVersion())

	return formattedIngresses
}

func (g *getFunctionCommandeer) renderFunctionConfig(functions []platform.Function, renderer func(interface{}) error) error {
	for _, function := range functions {
		if err := renderer(function.GetConfig()); err != nil {
			return errors.Wrap(err, "Failed to render function config")
		}

	}

	return nil
}

// rempveDuplicates takes array of strings and returns the array unduplicated
func (g *getFunctionCommandeer) removeDuplicates(args []string) []string {
	uniqueArgs := []string{}
	for _, arg := range args {

		// assume the argument hasn't been found in uniqueArgs
		argExists := false

		for _, uniqueArg := range uniqueArgs {
			if arg == uniqueArg {

				// mark that we found it and break out
				argExists = true
				break
			}
		}

		if !argExists {

			// if we haven't found the argument in uniqueArgs, add it here
			uniqueArgs = append(uniqueArgs, arg)
		}
	}

	return uniqueArgs
}
