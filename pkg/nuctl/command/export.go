package command

import (
	"fmt"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/spf13/cobra"
)

type exportCommandeer struct {
	cmd            *cobra.Command
	rootCommandeer *RootCommandeer
}

func newExportCommandeer(rootCommandeer *RootCommandeer) *exportCommandeer {
	commandeer := &exportCommandeer{
		rootCommandeer: rootCommandeer,
	}

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export resource to json format",
	}

	exportFunctionCommand := newExportFunctionCommandeer(commandeer).cmd
	exportProjectCommand := newExportProjectCommandeer(commandeer).cmd

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
	noScrub             bool
}

func newExportFunctionCommandeer(exportCommandeer *exportCommandeer) *exportFunctionCommandeer {
	commandeer := &exportFunctionCommandeer{
		exportCommandeer: exportCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "function [name[:version]]",
		Aliases: []string{"fu"},
		Short:   "Export function to yaml format",
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

			functions, err := exportCommandeer.rootCommandeer.platform.GetFunctions(&commandeer.getFunctionsOptions)
			if err != nil {
				return errors.Wrap(err, "Failed to get functions")
			}

			if len(functions) == 0 {
				if commandeer.getFunctionsOptions.Name != "" {
					return nuclio.NewErrNotFound("No functions found")
				}
				cmd.OutOrStdout().Write([]byte("No functions found")) // nolint: errcheck
				return nil
			}

			// render the functions
			return renderFunctions(functions, commandeer.output, cmd.OutOrStdout(), commandeer.renderFunctionConfig)
		},
	}

	cmd.PersistentFlags().StringVarP(&commandeer.getFunctionsOptions.Labels, "labels", "l", "", "Function labels (lbl1=val1[,lbl2=val2,...])")
	cmd.PersistentFlags().StringVarP(&commandeer.output, "output", "o", OutputFormatYAML, "Output format - \"yaml\", or \"json\"")
	cmd.PersistentFlags().BoolVar(&commandeer.noScrub, "no-scrub", false, "Allow function sensitive data to be exported")

	commandeer.cmd = cmd

	return commandeer
}

func (e *exportFunctionCommandeer) renderFunctionConfig(functions []platform.Function, renderer func(interface{}) error) error {
	if len(functions) == 1 {
		functionConfig := functions[0].GetConfig()
		functionConfig.PrepareFunctionForExport(e.noScrub)
		if err := renderer(functionConfig); err != nil {
			return errors.Wrap(err, "Failed to render function config")
		}
		return nil
	}

	functionConfigs := map[string]*functionconfig.Config{}
	for _, function := range functions {
		functionConfig := function.GetConfig()
		functionConfig.PrepareFunctionForExport(e.noScrub)
		functionConfigs[functionConfig.Meta.Name] = functionConfig
	}
	if err := renderer(functionConfigs); err != nil {
		return errors.Wrap(err, "Failed to render function config")
	}

	return nil
}

type exportProjectCommandeer struct {
	*exportCommandeer
	getProjectsOptions platform.GetProjectsOptions
	output             string
}

func newExportProjectCommandeer(exportCommandeer *exportCommandeer) *exportProjectCommandeer {
	commandeer := &exportProjectCommandeer{
		exportCommandeer: exportCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "project name",
		Aliases: []string{"proj"},
		Short:   "Export project with all it's functions and function events",
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

			projects, err := exportCommandeer.rootCommandeer.platform.GetProjects(&commandeer.getProjectsOptions)
			if err != nil {
				return errors.Wrap(err, "Failed to get projects")
			}

			if len(projects) == 0 {
				if commandeer.getProjectsOptions.Meta.Name != "" {
					return nuclio.NewErrNotFound("No functions found")
				}
				cmd.OutOrStdout().Write([]byte("No projects found")) // nolint: errcheck
				return nil
			}

			// render the projects
			return renderProjects(projects, commandeer.output, cmd.OutOrStdout(), commandeer.renderProjectConfig)
		},
	}

	cmd.PersistentFlags().StringVarP(&commandeer.output, "output", "o", OutputFormatYAML, "Output format - \"yaml\", or \"json\"")

	commandeer.cmd = cmd

	return commandeer
}

func (e *exportProjectCommandeer) getFunctionEvents(functionConfig *functionconfig.Config) ([]platform.FunctionEvent, error) {
	getFunctionEventOptions := platform.GetFunctionEventsOptions{
		Meta: platform.FunctionEventMeta{
			Name:      "",
			Namespace: functionConfig.Meta.Namespace,
			Labels: map[string]string{
				"nuclio.io/function-name": functionConfig.Meta.Name,
			},
		},
	}

	functionEvents, err := e.rootCommandeer.platform.GetFunctionEvents(&getFunctionEventOptions)
	if err != nil {
		return nil, err
	}

	return functionEvents, nil
}

func (e *exportProjectCommandeer) exportProjectFunctionsAndFunctionEvents(projectConfig *platform.ProjectConfig) (
	map[string]*functionconfig.Config, map[string]*platform.FunctionEventConfig, error) {
	getFunctionOptions := &platform.GetFunctionsOptions{
		Namespace: projectConfig.Meta.Namespace,
		Labels:    fmt.Sprintf("nuclio.io/project-name=%s", projectConfig.Meta.Name),
	}
	functions, err := e.rootCommandeer.platform.GetFunctions(getFunctionOptions)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to get functions")
	}
	functionMap := map[string]*functionconfig.Config{}
	functionEventMap := map[string]*platform.FunctionEventConfig{}

	for _, function := range functions {
		if err := function.Initialize(nil); err != nil {
			return nil, nil, err
		}
		functionConfig := function.GetConfig()

		functionEvents, err := e.getFunctionEvents(functionConfig)
		if err != nil {
			return nil, nil, errors.Wrap(err, "Failed to get function events")
		}
		for _, functionEvent := range functionEvents {
			functionEventConfig := functionEvent.GetConfig()
			functionEventConfig.Meta.Namespace = ""
			functionEventMap[functionEventConfig.Meta.Name] = functionEventConfig
		}

		functionConfig.PrepareFunctionForExport(false)
		functionMap[functionConfig.Meta.Name] = functionConfig
	}

	return functionMap, functionEventMap, nil
}

func (e *exportProjectCommandeer) exportProject(projectConfig *platform.ProjectConfig) (map[string]interface{}, error) {
	functions, functionEvents, err := e.exportProjectFunctionsAndFunctionEvents(projectConfig)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"project":        projectConfig,
		"functions":      functions,
		"functionEvents": functionEvents,
	}, nil
}

func (e *exportProjectCommandeer) renderProjectConfig(projects []platform.Project, renderer func(interface{}) error) error {
	if len(projects) == 1 {
		projectExport, err := e.exportProject(projects[0].GetConfig())
		if err != nil {
			return errors.Wrap(err, "Failed to export project")
		}
		if err := renderer(projectExport); err != nil {
			return errors.Wrap(err, "Failed to render function config")
		}
		return nil
	}

	projectConfigs := map[string]interface{}{}
	for _, project := range projects {
		projectConfig := project.GetConfig()
		projectExport, err := e.exportProject(projectConfig)
		if err != nil {
			return errors.Wrap(err, "Failed to gather functions and function events")
		}
		projectConfigs[projectConfig.Meta.Name] = projectExport
	}
	if err := renderer(projectConfigs); err != nil {
		return errors.Wrap(err, "Failed to render function config")
	}

	return nil
}
