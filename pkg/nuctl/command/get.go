/*
Copyright 2023 The Nuclio Authors.

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
	"context"

	nucliocommon "github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/nuctl/command/common"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/spf13/cobra"
)

type getCommandeer struct {
	cmd            *cobra.Command
	rootCommandeer *RootCommandeer
}

func newGetCommandeer(ctx context.Context, rootCommandeer *RootCommandeer) *getCommandeer {
	commandeer := &getCommandeer{
		rootCommandeer: rootCommandeer,
	}

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Display resource information",
	}

	getFunctionCommand := newGetFunctionCommandeer(ctx, commandeer).cmd
	getProjectCommand := newGetProjectCommandeer(ctx, commandeer).cmd
	getFunctionEventCommand := newGetFunctionEventCommandeer(ctx, commandeer).cmd
	getAPIGatewayCommand := newGetAPIGatewayCommandeer(ctx, commandeer).cmd

	cmd.AddCommand(
		getFunctionCommand,
		getProjectCommand,
		getFunctionEventCommand,
		getAPIGatewayCommand,
	)

	commandeer.cmd = cmd

	return commandeer
}

type getFunctionCommandeer struct {
	*getCommandeer
	getFunctionsOptions platform.GetFunctionsOptions
	output              string
}

func newGetFunctionCommandeer(ctx context.Context, getCommandeer *getCommandeer) *getFunctionCommandeer {
	commandeer := &getFunctionCommandeer{
		getCommandeer: getCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "functions [name[:version]]",
		Aliases: []string{"fu", "fn", "function"},
		Short:   "(or function) Display function information",
		RunE: func(cmd *cobra.Command, args []string) error {

			// if we got positional arguments
			if len(args) != 0 {

				// second argument is a resource name
				commandeer.getFunctionsOptions.Name = args[0]
			}

			// initialize root
			if err := getCommandeer.rootCommandeer.initialize(true); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			commandeer.getFunctionsOptions.Namespace = getCommandeer.rootCommandeer.namespace

			functions, err := getCommandeer.rootCommandeer.platform.GetFunctions(ctx, &commandeer.getFunctionsOptions)
			if err != nil {
				return errors.Wrap(err, "Failed to get functions")
			}

			if len(functions) == 0 {
				if commandeer.getFunctionsOptions.Name != "" {
					return nuclio.NewErrNotFound("No functions found")
				}
				cmd.OutOrStdout().Write([]byte("No functions found\n")) // nolint: errcheck
				return nil
			}

			// render the functions
			return common.RenderFunctions(ctx,
				commandeer.rootCommandeer.loggerInstance,
				functions,
				commandeer.output,
				cmd.OutOrStdout(),
				commandeer.renderFunctionConfigWithStatus,
				&nucliocommon.ExportFunctionOptions{})
		},
	}
	cmd.PersistentFlags().StringVarP(&commandeer.getFunctionsOptions.Labels, "labels", "l", "", "Function labels (lbl1=val1[,lbl2=val2,...])")
	cmd.PersistentFlags().StringVarP(&commandeer.output, "output", "o", common.OutputFormatText, "Output format - \"text\", \"wide\", \"yaml\", or \"json\"")
	commandeer.cmd = cmd

	return commandeer
}

func (g *getFunctionCommandeer) renderFunctionConfigWithStatus(functions []platform.Function,
	renderer func(interface{}) error, exportOptions *nucliocommon.ExportFunctionOptions) error {
	configsWithStatus := make([]functionconfig.ConfigWithStatus, 0, len(functions))
	for _, function := range functions {
		functionConfigWithStatus := functionconfig.ConfigWithStatus{
			Config: *function.GetConfig(),
			Status: *function.GetStatus(),
		}
		configsWithStatus = append(configsWithStatus, functionConfigWithStatus)
	}
	if len(configsWithStatus) == 1 {
		if err := renderer(configsWithStatus[0]); err != nil {
			return errors.Wrap(err, "Failed to render function config with status")
		}
		return nil
	}
	if err := renderer(configsWithStatus); err != nil {
		return errors.Wrap(err, "Failed to render function config with status")
	}

	return nil
}

type getProjectCommandeer struct {
	*getCommandeer
	getProjectsOptions platform.GetProjectsOptions
	output             string
}

func newGetProjectCommandeer(ctx context.Context, getCommandeer *getCommandeer) *getProjectCommandeer {
	commandeer := &getProjectCommandeer{
		getCommandeer: getCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "projects name",
		Aliases: []string{"proj", "prj", "project"},
		Short:   "(or project) Display project information",
		RunE: func(cmd *cobra.Command, args []string) error {

			// if we got positional arguments
			if len(args) != 0 {

				// second argument is a resource name
				commandeer.getProjectsOptions.Meta.Name = args[0]
			}

			// initialize root
			if err := getCommandeer.rootCommandeer.initialize(true); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			// get namespace
			commandeer.getProjectsOptions.Meta.Namespace = getCommandeer.rootCommandeer.namespace

			projects, err := getCommandeer.rootCommandeer.platform.GetProjects(ctx, &commandeer.getProjectsOptions)
			if err != nil {
				return errors.Wrap(err, "Failed to get projects")
			}

			if len(projects) == 0 {
				if commandeer.getProjectsOptions.Meta.Name != "" {
					return nuclio.NewErrNotFound("No projects found")
				}
				cmd.OutOrStdout().Write([]byte("No projects found\n")) // nolint: errcheck
				return nil
			}

			// render the projects
			return common.RenderProjects(ctx,
				projects,
				commandeer.output,
				cmd.OutOrStdout(),
				commandeer.renderProjectConfig,
				false)
		},
	}

	cmd.PersistentFlags().StringVarP(&commandeer.output, "output", "o", common.OutputFormatText, "Output format - \"text\", \"wide\", \"yaml\", or \"json\"")

	commandeer.cmd = cmd

	return commandeer
}

func (g *getProjectCommandeer) renderProjectConfig(ctx context.Context, projects []platform.Project, renderer func(interface{}) error) error {
	if len(projects) == 1 {
		if err := renderer(projects[0].GetConfig()); err != nil {
			return errors.Wrap(err, "Failed to render project config")
		}
		return nil
	}

	projectConfigs := make([]platform.ProjectConfig, 0, len(projects))
	for _, project := range projects {
		projectConfigs = append(projectConfigs, *project.GetConfig())
	}
	if err := renderer(projectConfigs); err != nil {
		return errors.Wrap(err, "Failed to render project config")
	}
	return nil
}

type getAPIGatewayCommandeer struct {
	*getCommandeer
	getAPIGatewaysOptions platform.GetAPIGatewaysOptions
	output                string
}

func newGetAPIGatewayCommandeer(ctx context.Context, getCommandeer *getCommandeer) *getAPIGatewayCommandeer {
	commandeer := &getAPIGatewayCommandeer{
		getCommandeer: getCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "apigateways name",
		Aliases: []string{"agw", "apigateway"},
		Short:   "(or apigateway) Display api gateways information",
		RunE: func(cmd *cobra.Command, args []string) error {

			// if we got positional arguments
			if len(args) != 0 {

				// second argument is a resource name
				commandeer.getAPIGatewaysOptions.Name = args[0]
			}

			// initialize root
			if err := getCommandeer.rootCommandeer.initialize(true); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			commandeer.getAPIGatewaysOptions.Namespace = getCommandeer.rootCommandeer.namespace

			apiGateways, err := getCommandeer.rootCommandeer.platform.GetAPIGateways(ctx, &commandeer.getAPIGatewaysOptions)
			if err != nil {
				return errors.Wrap(err, "Failed to get api gateways")
			}

			if len(apiGateways) == 0 {
				if commandeer.getAPIGatewaysOptions.Name != "" {
					return nuclio.NewErrNotFound("No api gateways found")
				}
				cmd.OutOrStdout().Write([]byte("No api gateways found\n")) // nolint: errcheck
				return nil
			}

			// render the function events
			return common.RenderAPIGateways(apiGateways, commandeer.output, cmd.OutOrStdout(), commandeer.renderAPIGatewayConfig)
		},
	}

	cmd.PersistentFlags().StringVarP(&commandeer.output, "output", "o", common.OutputFormatText, "Output format - \"text\", \"wide\", \"yaml\", or \"json\"")

	commandeer.cmd = cmd

	return commandeer
}

func (g *getAPIGatewayCommandeer) renderAPIGatewayConfig(apiGateways []platform.APIGateway, renderer func(interface{}) error) error {
	if len(apiGateways) == 1 {
		if err := renderer(apiGateways[0].GetConfig()); err != nil {
			return errors.Wrap(err, "Failed to render api gateway config")
		}
		return nil
	}

	apiGatewayConfigs := make([]platform.APIGatewayConfig, 0, len(apiGateways))
	for _, apiGateway := range apiGateways {
		apiGatewayConfigs = append(apiGatewayConfigs, *apiGateway.GetConfig())
	}
	if err := renderer(apiGatewayConfigs); err != nil {
		return errors.Wrap(err, "Failed to render api gateway config")
	}

	return nil
}

type getFunctionEventCommandeer struct {
	*getCommandeer
	getFunctionEventsOptions platform.GetFunctionEventsOptions
	output                   string
	functionName             string
}

func newGetFunctionEventCommandeer(ctx context.Context, getCommandeer *getCommandeer) *getFunctionEventCommandeer {
	commandeer := &getFunctionEventCommandeer{
		getCommandeer: getCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "functionevents name",
		Aliases: []string{"fe", "functionevent"},
		Short:   "(or functionevent) Display function event information",
		RunE: func(cmd *cobra.Command, args []string) error {

			// if we got positional arguments
			if len(args) != 0 {

				// second argument is a resource name
				commandeer.getFunctionEventsOptions.Meta.Name = args[0]
			}

			// initialize root
			if err := getCommandeer.rootCommandeer.initialize(true); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			commandeer.getFunctionEventsOptions.Meta.Namespace = getCommandeer.rootCommandeer.namespace

			if commandeer.functionName != "" {
				commandeer.getFunctionEventsOptions.Meta.Labels = map[string]string{
					nucliocommon.NuclioResourceLabelKeyFunctionName: commandeer.functionName,
				}
			}

			functionEvents, err := getCommandeer.rootCommandeer.platform.GetFunctionEvents(ctx,
				&commandeer.getFunctionEventsOptions)
			if err != nil {
				return errors.Wrap(err, "Failed to get function events")
			}

			if len(functionEvents) == 0 {
				if commandeer.getFunctionEventsOptions.Meta.Name != "" {
					return nuclio.NewErrNotFound("No function events found")
				}
				cmd.OutOrStdout().Write([]byte("No function events found\n")) // nolint: errcheck
				return nil
			}

			// render the function events
			return common.RenderFunctionEvents(functionEvents, commandeer.output, cmd.OutOrStdout(), commandeer.renderFunctionEventConfig)
		},
	}

	cmd.PersistentFlags().StringVarP(&commandeer.functionName, "function", "f", "", "Filter by owning function (optional)")
	cmd.PersistentFlags().StringVarP(&commandeer.output, "output", "o", common.OutputFormatText, "Output format - \"text\", \"wide\", \"yaml\", or \"json\"")

	commandeer.cmd = cmd

	return commandeer
}

func (g *getFunctionEventCommandeer) renderFunctionEventConfig(functionEvents []platform.FunctionEvent, renderer func(interface{}) error) error {
	if len(functionEvents) == 1 {
		if err := renderer(functionEvents[0].GetConfig()); err != nil {
			return errors.Wrap(err, "Failed to render function event config")
		}
		return nil
	}

	functionEventConfigs := make([]platform.FunctionEventConfig, 0, len(functionEvents))
	for _, functionEvent := range functionEvents {
		functionEventConfigs = append(functionEventConfigs, *functionEvent.GetConfig())
	}
	if err := renderer(functionEventConfigs); err != nil {
		return errors.Wrap(err, "Failed to render function event config")
	}

	return nil
}
