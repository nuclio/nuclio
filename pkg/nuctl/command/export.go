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
	"fmt"
	"github.com/nuclio/nuclio/pkg/dashboard/resource"
	"sync"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errgroup"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuctlcommon "github.com/nuclio/nuclio/pkg/nuctl/command/common"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/spf13/cobra"
)

type exportCommandeer struct {
	cmd            *cobra.Command
	rootCommandeer *RootCommandeer
	scrubber       *functionconfig.Scrubber
	noScrub        bool
}

func newExportCommandeer(ctx context.Context, rootCommandeer *RootCommandeer) *exportCommandeer {
	commandeer := &exportCommandeer{
		rootCommandeer: rootCommandeer,
	}

	// initialize scrubber, used for restoring only
	commandeer.scrubber = functionconfig.NewScrubber(nil, nil)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export functions or projects",
		Long: `Export the configuration of a specific function or project or of all functions/projects (default)
to the standard output, in JSON or YAML format`,
	}

	cmd.PersistentFlags().BoolVar(&commandeer.noScrub, "no-scrub", false, "Export all function data, including sensitive and unnecessary data")

	exportFunctionCommand := newExportFunctionCommandeer(ctx, commandeer).cmd
	exportProjectCommand := newExportProjectCommandeer(ctx, commandeer).cmd

	cmd.AddCommand(
		exportFunctionCommand,
		exportProjectCommand,
	)

	commandeer.cmd = cmd

	return commandeer
}

type exportFunctionCommandeer struct {
	*exportCommandeer
	getFunctionsOptions platform.GetFunctionsOptions
	output              string
	skipSpecCleanup     bool
	withPrevState       bool
}

func newExportFunctionCommandeer(ctx context.Context, exportCommandeer *exportCommandeer) *exportFunctionCommandeer {
	commandeer := &exportFunctionCommandeer{
		exportCommandeer: exportCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "functions [<function>]",
		Aliases: []string{"function", "fn", "fu"},
		Short:   "(or function) Export functions",
		Long: `(or function) Export the configuration of a specific function or of all deployed Nuclio functions (default)
to the standard output, in JSON or YAML format (see -o|--output)

Arguments:
  <function> (string) The name of a function to export`,
		RunE: func(cmd *cobra.Command, args []string) error {

			// if we got positional arguments
			if len(args) != 0 {

				// second argument is a resource name
				commandeer.getFunctionsOptions.Name = args[0]
			}

			// initialize root
			if err := exportCommandeer.rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			commandeer.getFunctionsOptions.Namespace = exportCommandeer.rootCommandeer.namespace

			functions, err := exportCommandeer.rootCommandeer.platform.GetFunctions(ctx,
				&commandeer.getFunctionsOptions)
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
			exportOptions := &resource.ExportOptions{SkipSpecCleanUp: commandeer.skipSpecCleanup, NoScrub: commandeer.noScrub, AddPrevState: commandeer.withPrevState}

			// render the functions
			return nuctlcommon.RenderFunctions(ctx,
				commandeer.rootCommandeer.loggerInstance,
				functions,
				commandeer.output,
				cmd.OutOrStdout(),
				commandeer.renderFunctionConfig,
				exportOptions,
			)
		},
	}

	cmd.Flags().BoolVarP(&commandeer.skipSpecCleanup, "skip-spec-cleanup", "", false, "Do not clear the image info from the function spec")
	cmd.Flags().BoolVarP(&commandeer.withPrevState, "with-previous-state", "", false, "Save function state before export so it can be redeployed right to this state")
	cmd.PersistentFlags().StringVarP(&commandeer.output, "output", "o", nuctlcommon.OutputFormatYAML, "Output format - \"json\" or \"yaml\"")

	commandeer.cmd = cmd

	return commandeer
}

func (e *exportFunctionCommandeer) renderFunctionConfig(functions []platform.Function, renderer func(interface{}) error, options *resource.ExportOptions) error {
	functionConfigs := map[string]*functionconfig.Config{}
	lock := sync.Mutex{}
	errGroup, errGroupCtx := errgroup.WithContextSemaphore(context.Background(),
		e.rootCommandeer.loggerInstance,
		errgroup.DefaultErrgroupConcurrency)
	for _, function := range functions {
		function := function
		errGroup.Go("renderFunctionConfig", func() error {

			functionConfig := function.GetConfig()

			if scrubbed, err := e.scrubber.HasScrubbedConfig(functionConfig,
				e.rootCommandeer.platform.GetConfig().SensitiveFields.CompileSensitiveFieldsRegex()); err == nil && scrubbed {
				var restoreErr error
				functionConfig, restoreErr = e.scrubber.RestoreFunctionConfig(errGroupCtx,
					functionConfig,
					e.rootCommandeer.platform.GetName(),
					e.rootCommandeer.platform.GetFunctionSecretMap)
				if restoreErr != nil {
					return errors.Wrap(err, "Failed to restore function config")
				}
			} else if err != nil {
				return errors.Wrap(err, "Failed to check if function config is scrubbed")
			}
			if options.AddPrevState {
				options.PrevState = string(function.GetStatus().State)
			}
			functionConfig.PrepareFunctionForExport(options)
			lock.Lock()
			functionConfigs[functionConfig.Meta.Name] = functionConfig
			lock.Unlock()

			return nil
		})
	}

	if err := errGroup.Wait(); err != nil {
		return errors.Wrap(err, "Failed to render function configs")
	}

	var err error
	if len(functions) == 1 {
		err = renderer(functionConfigs[functions[0].GetConfig().Meta.Name])
	} else {
		err = renderer(functionConfigs)
	}
	if err != nil {
		return errors.Wrap(err, "Failed to render function configuration")
	}

	return nil
}

type exportProjectCommandeer struct {
	*exportCommandeer
	getProjectsOptions platform.GetProjectsOptions
	output             string
	skipSpecCleanup    bool
}

func newExportProjectCommandeer(ctx context.Context, exportCommandeer *exportCommandeer) *exportProjectCommandeer {
	commandeer := &exportProjectCommandeer{
		exportCommandeer: exportCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "projects [<project>]",
		Aliases: []string{"project", "prj", "proj"},
		Short:   "(or project) Export projects (including all functions, function events, and API gateways)",
		Long: `(or project) Export the configuration of a specific project (including
all its functions, function events, and API gateways) or of all projects (default)
to the standard output, in JSON or YAML format (see -o|--output)

Arguments:
  <project> (string) The name of a project to export`,
		RunE: func(cmd *cobra.Command, args []string) error {

			// if we got positional arguments
			if len(args) != 0 {

				// second argument is a resource name
				commandeer.getProjectsOptions.Meta.Name = args[0]
			}

			// initialize root
			if err := exportCommandeer.rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			// get namespace
			commandeer.getProjectsOptions.Meta.Namespace = exportCommandeer.rootCommandeer.namespace

			projects, err := exportCommandeer.rootCommandeer.platform.GetProjects(ctx,
				&commandeer.getProjectsOptions)
			if err != nil {
				return errors.Wrap(err, "Failed to get projects")
			}

			if len(projects) == 0 {
				if commandeer.getProjectsOptions.Meta.Name != "" {
					return nuclio.NewErrNotFound("Project not found")
				}
				cmd.OutOrStdout().Write([]byte("No projects found")) // nolint: errcheck
				return nil
			}

			// render the projects
			return nuctlcommon.RenderProjects(ctx, projects, commandeer.output, cmd.OutOrStdout(), commandeer.renderProjectConfig, commandeer.skipSpecCleanup)
		},
	}

	cmd.Flags().BoolVarP(&commandeer.skipSpecCleanup, "skip-spec-cleanup", "", false, "Do not clear the image info from the function spec")
	cmd.PersistentFlags().StringVarP(&commandeer.output, "output", "o", nuctlcommon.OutputFormatYAML, "Output format - \"json\" or \"yaml\"")
	commandeer.cmd = cmd

	return commandeer
}

func (e *exportProjectCommandeer) getFunctionEvents(ctx context.Context, functionConfig *functionconfig.Config) ([]platform.FunctionEvent, error) {
	getFunctionEventOptions := platform.GetFunctionEventsOptions{
		Meta: platform.FunctionEventMeta{
			Name:      "",
			Namespace: functionConfig.Meta.Namespace,
			Labels: map[string]string{
				"nuclio.io/function-name": functionConfig.Meta.Name,
			},
		},
	}

	functionEvents, err := e.rootCommandeer.platform.GetFunctionEvents(ctx, &getFunctionEventOptions)
	if err != nil {
		return nil, err
	}

	return functionEvents, nil
}

func (e *exportProjectCommandeer) exportAPIGateways(ctx context.Context, projectConfig *platform.ProjectConfig) (map[string]*platform.APIGatewayConfig, error) {
	getAPIGatewaysOptions := &platform.GetAPIGatewaysOptions{
		Namespace: projectConfig.Meta.Namespace,
		Labels: fmt.Sprintf("%s=%s", common.NuclioResourceLabelKeyProjectName,
			projectConfig.Meta.Name),
	}

	// get all api gateways in the project
	apiGateways, err := e.rootCommandeer.platform.GetAPIGateways(ctx, getAPIGatewaysOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get API gateways")
	}

	apiGatewaysMap := map[string]*platform.APIGatewayConfig{}

	// create a mapping of an api gateway name to its config [ string -> *platform.APIGatewayConfig ]
	for _, apiGateway := range apiGateways {
		apiGatewayConfig := apiGateway.GetConfig()
		apiGatewayConfig.PrepareAPIGatewayForExport(false)
		apiGatewaysMap[apiGatewayConfig.Meta.Name] = apiGatewayConfig
	}

	return apiGatewaysMap, nil
}

func (e *exportProjectCommandeer) exportProjectFunctionsAndFunctionEvents(ctx context.Context, projectConfig *platform.ProjectConfig, exportFuncOptions *resource.ExportOptions) (
	map[string]*functionconfig.Config, map[string]*platform.FunctionEventConfig, error) {
	getFunctionOptions := &platform.GetFunctionsOptions{
		Namespace: projectConfig.Meta.Namespace,
		Labels:    fmt.Sprintf("%s=%s", common.NuclioResourceLabelKeyProjectName, projectConfig.Meta.Name),
	}
	functions, err := e.rootCommandeer.platform.GetFunctions(ctx, getFunctionOptions)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to get functions")
	}
	functionMap := map[string]*functionconfig.Config{}
	functionEventMap := map[string]*platform.FunctionEventConfig{}

	for _, function := range functions {
		if err := function.Initialize(ctx, nil); err != nil {
			e.rootCommandeer.loggerInstance.DebugWith("Failed to initialize a function", "err", err.Error())
		}

		// restore the function config, if needed
		functionConfig, err := e.scrubber.RestoreFunctionConfig(context.Background(),
			function.GetConfig(),
			e.rootCommandeer.platform.GetName(),
			e.rootCommandeer.platform.GetFunctionSecretMap)
		if err != nil {
			return nil, nil, errors.Wrap(err, "Failed to restore function config")
		}

		functionEvents, err := e.getFunctionEvents(ctx, functionConfig)
		if err != nil {
			return nil, nil, errors.Wrap(err, "Failed to get function events")
		}
		for _, functionEvent := range functionEvents {
			functionEventConfig := functionEvent.GetConfig()
			functionEventConfig.Meta.Namespace = ""
			functionEventMap[functionEventConfig.Meta.Name] = functionEventConfig
		}
		if exportFuncOptions.AddPrevState {
			exportFuncOptions.PrevState = string(function.GetStatus().State)
		}
		functionConfig.PrepareFunctionForExport(exportFuncOptions)
		functionMap[functionConfig.Meta.Name] = functionConfig
	}

	return functionMap, functionEventMap, nil
}

func (e *exportProjectCommandeer) exportProject(ctx context.Context, projectConfig *platform.ProjectConfig) (map[string]interface{}, error) {
	exportFuncOptions := &resource.ExportOptions{SkipSpecCleanUp: false, NoScrub: e.noScrub}
	functions, functionEvents, err := e.exportProjectFunctionsAndFunctionEvents(ctx, projectConfig, exportFuncOptions)
	if err != nil {
		return nil, err
	}

	projectConfig.Scrub()

	exportedProject := map[string]interface{}{
		"project":        projectConfig,
		"functions":      functions,
		"functionEvents": functionEvents,
	}

	// api gateways are supported only on k8s platform
	if e.rootCommandeer.platform.GetName() == common.KubePlatformName {
		apiGateways, err := e.exportAPIGateways(ctx, projectConfig)
		if err != nil {

			// in case an error occurred while exporting api gateways - skip this part
			// (because it may fail when exporting after an upgrade from an older version)
			e.rootCommandeer.loggerInstance.DebugWith("Failed to export api gateways; continuing with project export",
				"err", err)
		}

		exportedProject["apiGateways"] = apiGateways
	}

	return exportedProject, nil
}

func (e *exportProjectCommandeer) renderProjectConfig(ctx context.Context, projects []platform.Project, renderer func(interface{}) error) error {
	projectConfigs := map[string]interface{}{}
	for _, project := range projects {
		projectConfig := project.GetConfig()
		projectExport, err := e.exportProject(ctx, projectConfig)
		if err != nil {
			return errors.Wrap(err, "Failed to gather functions and function events")
		}
		projectConfigs[projectConfig.Meta.Name] = projectExport
	}

	var err error
	if len(projects) == 1 {
		err = renderer(projectConfigs[projects[0].GetConfig().Meta.Name])
	} else {
		err = renderer(projectConfigs)
	}

	if err != nil {
		return errors.Wrap(err, "Failed to render a function configuration")
	}

	return nil
}
