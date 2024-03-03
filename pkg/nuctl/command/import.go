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
	"strings"
	"sync"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errgroup"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuctlcommon "github.com/nuclio/nuclio/pkg/nuctl/command/common"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/google/uuid"
	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/spf13/cobra"
	"golang.org/x/sync/semaphore"
)

type importCommandeer struct {
	cmd            *cobra.Command
	rootCommandeer *RootCommandeer
	saveReport     bool
	reportFilePath string
	skipAutofix    bool
}

func newImportCommandeer(ctx context.Context, rootCommandeer *RootCommandeer) *importCommandeer {
	commandeer := &importCommandeer{
		rootCommandeer: rootCommandeer,
	}

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import functions or projects",
		Long: `Import the configurations of one or more functions or projects
from a configuration file or from the standard input (default)`,
	}

	cmd.PersistentFlags().BoolVar(&commandeer.saveReport, "save-report", false, "Save importing report to a file")
	cmd.PersistentFlags().StringVar(&commandeer.reportFilePath, "report-file-path", "nuctl-import-report.json", "Path to import report")
	cmd.PersistentFlags().BoolVar(&commandeer.skipAutofix, "skip-autofix", false, "Skip config autofix if error occurred")

	importFunctionCommand := newImportFunctionCommandeer(ctx, commandeer).cmd
	importProjectCommand := newImportProjectCommandeer(ctx, commandeer).cmd

	cmd.AddCommand(
		importFunctionCommand,
		importProjectCommand,
	)

	commandeer.cmd = cmd

	return commandeer
}

func (i *importCommandeer) resolveInputData(args []string) ([]byte, error) {
	if len(args) >= 1 {
		filename := args[0]
		i.rootCommandeer.loggerInstance.DebugWith("Reading from a file", "filename", filename)
		file, err := nuctlcommon.OpenFile(filename)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to open a file")
		}
		i.cmd.SetIn(file)

		// close file after reading from it
		defer file.Close() // nolint: errcheck
	}

	// read from file if given, fallback to stdin
	return nuctlcommon.ReadFromInOrStdin(i.cmd.InOrStdin())
}

func (i *importCommandeer) importFunction(ctx context.Context, functionConfig *functionconfig.Config, project *platform.ProjectConfig, skipAutofix bool) error {

	// populate namespace
	functionConfig.Meta.Namespace = project.Meta.Namespace

	if project.Meta.Name != "" {
		if functionConfig.Meta.Labels == nil {
			functionConfig.Meta.Labels = make(map[string]string)
		}
		functionConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName] = project.Meta.Name
	}

	functions, err := i.rootCommandeer.platform.GetFunctions(ctx, &platform.GetFunctionsOptions{
		Name:      functionConfig.Meta.Name,
		Namespace: i.rootCommandeer.namespace,
	})
	if err != nil {
		return errors.Wrap(err, "Failed to check existing functions")
	}

	if len(functions) > 0 {
		return errors.Errorf("Function with the name: %s already exists", functionConfig.Meta.Name)
	}

	// create function
	createFunctionCtx := context.WithoutCancel(ctx)
	_, err = i.rootCommandeer.platform.CreateFunction(createFunctionCtx,
		&platform.CreateFunctionOptions{
			Logger:               i.rootCommandeer.loggerInstance,
			FunctionConfig:       *functionConfig,
			AutofixConfiguration: !skipAutofix,
		})

	return err
}

func (i *importCommandeer) isAutoFixable(err error) bool {
	for _, fixableError := range functionconfig.FixableValidationErrors {
		if strings.Contains(strings.ToLower(errors.GetErrorStackString(err, 10)),
			strings.ToLower(fixableError)) {
			return true
		}
	}
	return false
}

func (i *importCommandeer) importFunctions(ctx context.Context,
	functionConfigs map[string]*functionconfig.Config,
	project *platform.ProjectConfig,
	report *nuctlcommon.FunctionReports) error {
	i.rootCommandeer.loggerInstance.DebugWithCtx(ctx,
		"Importing functions",
		"functions", functionConfigs)
	wg := sync.WaitGroup{}
	var sem = semaphore.NewWeighted(int64(i.rootCommandeer.concurrency))
	for _, functionConfig := range functionConfigs {
		wg.Add(1)
		_ = sem.Acquire(ctx, 1)
		go func(function *functionconfig.Config) {
			i.rootCommandeer.loggerInstance.DebugWithCtx(ctx,
				"Importing function",
				"function", function.Meta.Name,
				"project", project.Meta.Name)

			if err := i.importFunction(ctx, function, project, i.skipAutofix); err != nil {
				report.AddFailure(function.Meta.Name, err, i.isAutoFixable(err))
				i.rootCommandeer.loggerInstance.ErrorWithCtx(ctx,
					"Failed to import function",
					"function", function.Meta.Name,
					"project", project.Meta.Name,
					"error", err)
			} else {
				report.AddSuccess(function.Meta.Name)
				i.rootCommandeer.loggerInstance.DebugWithCtx(ctx,
					"Function was imported successfully",
					"function", function.Meta.Name,
					"project", project.Meta.Name)
			}
			sem.Release(1)
			wg.Done()
		}(functionConfig)
	}
	wg.Wait()

	if errorReport := report.SprintfError(); errorReport != "" {
		return errors.New(fmt.Sprintf(
			"Import failed for some of the functions in project `%s`.",
			project.Meta.Name),
		)
	}
	return nil
}

type importFunctionCommandeer struct {
	*importCommandeer
	report *nuctlcommon.FunctionReports
}

func newImportFunctionCommandeer(ctx context.Context, importCommandeer *importCommandeer) *importFunctionCommandeer {
	commandeer := &importFunctionCommandeer{
		importCommandeer: importCommandeer,
		report:           nuctlcommon.NewFunctionReports(),
	}

	cmd := &cobra.Command{
		Use:     "functions [<config file>]",
		Aliases: []string{"function", "fn", "fu"},
		Short:   "(or function) Import functions",
		Long: `Import the configurations of one or more functions from
a configurations file or from standard input (default)

Note: The command doesn't deploy the imported functions.
      To deploy an imported function, use the 'deploy' command.

Arguments:
  <config file> (string) Path to a function-configurations file in JSON or YAML format (see -o|--output).
                         If not provided, the configuration is imported from standard input (stdin).`,
		RunE: func(cmd *cobra.Command, args []string) error {

			// initialize root
			if err := importCommandeer.rootCommandeer.initialize(true); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			// disable sensitive field masking in the platform when importing, for backwards compatibility
			commandeer.rootCommandeer.platform.GetConfig().DisableSensitiveFieldMasking()

			functionBody, err := commandeer.resolveInputData(args)
			if err != nil {
				return errors.Wrap(err, "Failed to read function data")
			}

			if len(functionBody) == 0 {
				return errors.New(`Failed to resolve the function-configuration body.
Make sure to provide the content via stdin or a file.
Use --help for more information`)
			}

			unmarshalFunc, err := nuctlcommon.GetUnmarshalFunc(functionBody)
			if err != nil {
				return errors.Wrap(err, "Failed to identify the input format")
			}

			functionConfigs, err := commandeer.resolveFunctionImportConfigs(functionBody, unmarshalFunc)
			if err != nil {
				return errors.Wrap(err, "Failed to resolve the imported function configuration")
			}

			// create a platform config without name, allowing them to be imported directly to the default project
			platformConfig := &platform.ProjectConfig{
				Meta: platform.ProjectMeta{
					Namespace: commandeer.rootCommandeer.namespace,
				},
			}

			err = commandeer.importFunctions(ctx, functionConfigs, platformConfig, commandeer.report)
			if commandeer.saveReport {
				commandeer.report.SaveToFile(ctx, commandeer.rootCommandeer.loggerInstance, commandeer.reportFilePath)
			}
			return err
		},
	}

	commandeer.cmd = cmd

	return commandeer
}

func (i *importFunctionCommandeer) resolveFunctionImportConfigs(functionBody []byte,
	unmarshalFunc func(data []byte, v interface{}) error) (map[string]*functionconfig.Config, error) {

	// initialize
	functionConfigs := map[string]*functionconfig.Config{}

	// try parsing a single-project configuration
	functionConfig := &functionconfig.Config{}
	if err := unmarshalFunc(functionBody, &functionConfig); err != nil {
		return nil, errors.Wrap(err, "Failed to parse a single-project configuration")
	}

	// no match; try a multi-project configuration
	if functionConfig.Meta.Name == "" {
		if err := unmarshalFunc(functionBody, &functionConfigs); err != nil {
			return nil, errors.Wrap(err, "Failed to parse a multi-project configuration")
		}

	} else {

		// successfully parsed a single-project configuration
		functionConfigs[functionConfig.Meta.Name] = functionConfig
	}

	return functionConfigs, nil
}

type importProjectCommandeer struct {
	*importCommandeer
	skipProjectNames   []string
	skipLabelSelectors string
	report             *nuctlcommon.ProjectReports
}

func newImportProjectCommandeer(ctx context.Context, importCommandeer *importCommandeer) *importProjectCommandeer {
	commandeer := &importProjectCommandeer{
		importCommandeer: importCommandeer,
		report:           nuctlcommon.NewProjectReports(),
	}

	cmd := &cobra.Command{
		Use:     "projects [<config file>]",
		Aliases: []string{"project", "prj", "proj"},
		Short:   "(or project) Import projects (including all functions, function events, and API gateways)",
		Long: `Import the configurations of one or more projects (including all project 
functions, function events, and API gateways) from a configurations file
or from standard input (default)

Note: The command doesn't deploy the functions in the  imported projects.
      To deploy an imported function, use the 'deploy' command.

Arguments:
  <config file> (string) Path to a project-configurations file in JSON or YAML format (see -o|--output).
                         If not provided, the configuration is imported from standard input (stdin).`,
		RunE: func(cmd *cobra.Command, args []string) error {

			// initialize root
			if err := importCommandeer.rootCommandeer.initialize(true); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			// disable sensitive field masking in the platform when importing, for backwards compatibility
			commandeer.rootCommandeer.platform.GetConfig().DisableSensitiveFieldMasking()

			projectBody, err := commandeer.resolveInputData(args)
			if err != nil {
				return errors.Wrap(err, "Failed to read function data")
			}

			if len(projectBody) == 0 {
				return errors.New(`Failed to resolve the project-configuration body.
Make sure to provide the content via stdin or a file.
Use --help for more information`)
			}

			unmarshalFunc, err := nuctlcommon.GetUnmarshalFunc(projectBody)
			if err != nil {
				return errors.Wrap(err, "Failed to identify the input format")
			}

			importProjectsOptions, err := commandeer.resolveImportProjectsOptions(projectBody, unmarshalFunc)
			if err != nil {
				return errors.Wrap(err, "Failed to resolve the imported project configuration")
			}

			err = commandeer.importProjects(ctx, importProjectsOptions)
			if commandeer.saveReport {
				commandeer.report.SaveToFile(ctx, commandeer.rootCommandeer.loggerInstance, commandeer.reportFilePath)
			}
			return err
		},
	}

	cmd.Flags().StringSliceVar(&commandeer.skipProjectNames, "skip", []string{}, "Names of projects to skip (don't import), as a comma-separated list")
	cmd.Flags().StringVar(&commandeer.skipLabelSelectors, "skip-label-selectors", "", "Kubernetes label-selectors filter that identifies projects to skip (don't import)")

	commandeer.cmd = cmd

	return commandeer
}

func (i *importProjectCommandeer) importFunctionEvent(ctx context.Context, functionEvent *platform.FunctionEventConfig) error {
	functions, err := i.rootCommandeer.platform.GetFunctions(ctx, &platform.GetFunctionsOptions{
		Name:      functionEvent.Meta.Labels[common.NuclioResourceLabelKeyFunctionName],
		Namespace: i.rootCommandeer.namespace,
	})
	if err != nil {
		return errors.Wrap(err, "Failed to check existing functions")
	}

	if len(functions) == 0 {
		return errors.New("The event function's parent function doesn't exist")
	}

	// generate new name for events to avoid collisions
	functionEvent.Meta.Name = uuid.New().String()

	// populate namespace
	functionEvent.Meta.Namespace = i.rootCommandeer.namespace

	// just deploy; the status is async through polling
	return i.rootCommandeer.platform.CreateFunctionEvent(ctx, &platform.CreateFunctionEventOptions{
		FunctionEventConfig: platform.FunctionEventConfig{
			Meta: functionEvent.Meta,
			Spec: functionEvent.Spec,
		},
	})
}

func (i *importProjectCommandeer) importAPIGateway(ctx context.Context, apiGateway *platform.APIGatewayConfig) error {

	// populate namespace
	apiGateway.Meta.Namespace = i.rootCommandeer.namespace

	// just create; the status is async through polling
	return i.rootCommandeer.platform.CreateAPIGateway(ctx, &platform.CreateAPIGatewayOptions{
		APIGatewayConfig: &platform.APIGatewayConfig{
			Meta: apiGateway.Meta,
			Spec: apiGateway.Spec,
		},
	})
}

func (i *importProjectCommandeer) importFunctionEvents(ctx context.Context,
	functionEvents map[string]*platform.FunctionEventConfig) error {
	errGroup, _ := errgroup.WithContext(ctx, i.rootCommandeer.loggerInstance)

	i.rootCommandeer.loggerInstance.DebugWithCtx(ctx, "Importing function events",
		"functionEvents", functionEvents)
	for _, functionEventConfig := range functionEvents {
		functionEventConfig := functionEventConfig // https://golang.org/doc/faq#closures_and_goroutines
		errGroup.Go("Import function event", func() error {
			return i.importFunctionEvent(ctx, functionEventConfig)
		})
	}

	return errGroup.Wait()
}

func (i *importProjectCommandeer) importAPIGateways(ctx context.Context,
	apiGateways map[string]*platform.APIGatewayConfig) error {

	i.rootCommandeer.loggerInstance.DebugWithCtx(ctx,
		"Importing api gateways",
		"apiGateways", apiGateways)

	if apiGateways == nil {
		return nil
	}
	errGroup, _ := errgroup.WithContext(ctx, i.rootCommandeer.loggerInstance)
	for _, apiGatewayConfig := range apiGateways {
		apiGatewayConfig := apiGatewayConfig // https://golang.org/doc/faq#closures_and_goroutines
		errGroup.Go("Import API Gateway", func() error {
			return i.importAPIGateway(ctx, apiGatewayConfig)
		})
	}
	return errGroup.Wait()
}

func (i *importProjectCommandeer) importProject(ctx context.Context,
	projectImportOptions *ProjectImportOptions,
	projectReport *nuctlcommon.ProjectReport) error {

	var err error
	project, err := i.importProjectIfMissing(ctx, projectImportOptions)
	if err != nil {
		return err
	}

	// assign imported project
	projectImportOptions.projectImportConfig.Project = project.GetConfig()

	// enrich
	i.enrichProjectImportConfig(projectImportOptions.projectImportConfig)
	// import functions
	functionImportErr := i.importFunctions(ctx,
		projectImportOptions.projectImportConfig.Functions,
		projectImportOptions.projectImportConfig.Project,
		projectReport.FunctionReports,
	)
	if functionImportErr != nil {
		i.rootCommandeer.loggerInstance.WarnWithCtx(ctx,
			"Failed to import all project functions",
			"functionImportErr", functionImportErr)

		// return this error
		err = functionImportErr
	}

	// import function events
	functionEventImportErr := i.importFunctionEvents(ctx, projectImportOptions.projectImportConfig.FunctionEvents)
	if functionEventImportErr != nil {
		i.rootCommandeer.loggerInstance.WarnWithCtx(ctx, "Failed to import all function events",
			"functionEventImportErr", functionEventImportErr)

		// return this err only if not previously set
		if err == nil {
			err = functionEventImportErr
		}
	}

	// api gateways are supported only on k8s platform
	if i.rootCommandeer.platform.GetName() == common.KubePlatformName {

		// import api gateways
		apiGatewaysImportErr := i.importAPIGateways(ctx, projectImportOptions.projectImportConfig.APIGateways)
		if apiGatewaysImportErr != nil {
			i.rootCommandeer.loggerInstance.WarnWithCtx(ctx, "Unable to import all api gateways",
				"apiGatewaysImportErr", apiGatewaysImportErr)

			// return this err only if not previously set
			if err == nil {
				err = apiGatewaysImportErr
			}
		}
	}

	return err
}

func (i *importProjectCommandeer) importProjects(ctx context.Context,
	projectsImportOptions map[string]*ProjectImportOptions) error {
	i.rootCommandeer.loggerInstance.DebugWith("Importing projects",
		"projectsImportOptions", projectsImportOptions,
		"skipLabelSelectors", i.skipLabelSelectors,
		"skipProjectNames", i.skipProjectNames)

	// TODO: parallel this with errorGroup, mutex is required due to multi map writers
	for projectName, projectImportOptions := range projectsImportOptions {
		projectImportConfig := projectImportOptions.projectImportConfig
		projectReport := nuctlcommon.NewProjectReport(projectName)
		i.report.AddReport(projectReport)
		// skip project?
		skipProject, err := i.shouldSkipProject(projectImportConfig)
		if err != nil {
			projectReport.SetFailed(&nuctlcommon.FailReport{
				FailReason: errors.Wrap(err, "Failed to check whether project needs to be skipped").Error()})
			continue
		}
		if skipProject {
			i.rootCommandeer.loggerInstance.DebugWithCtx(ctx, "Skipping import for project",
				"projectNamespace", projectImportConfig.Project.Meta.Namespace,
				"projectName", projectImportConfig.Project.Meta.Name)
			projectReport.Skipped = true
			continue
		}

		i.rootCommandeer.loggerInstance.DebugWithCtx(ctx, "Importing project",
			"projectNamespace", projectImportConfig.Project.Meta.Namespace,
			"projectName", projectName)

		// enrich namespace from arg
		if projectImportConfig.Project.Meta.Namespace == "" {
			projectImportConfig.Project.Meta.Namespace = i.rootCommandeer.namespace
		}

		// import project
		if err := i.importProject(ctx, projectImportOptions, projectReport); err != nil {
			projectReport.SetFailed(&nuctlcommon.FailReport{
				FailReason: err.Error()})
			continue
		}

		projectReport.Success = true
		i.rootCommandeer.loggerInstance.InfoWithCtx(ctx, "Successfully imported project",
			"projectNamespace", projectImportConfig.Project.Meta.Namespace,
			"projectName", projectName)
	}
	if errorReport := i.report.SprintfError(); errorReport != "" {
		return errors.New(errorReport)
	}
	return nil
}

func (i *importProjectCommandeer) resolveImportProjectsOptions(projectBody []byte,
	unmarshalFunc func(data []byte, v interface{}) error) (map[string]*ProjectImportOptions, error) {

	// initialize
	projectImportOptions := map[string]*ProjectImportOptions{}

	// for un-marshaling
	projectImportConfigs := map[string]*ProjectImportConfig{}
	projectImportConfig := ProjectImportConfig{}

	// try a single-project configuration
	if err := unmarshalFunc(projectBody, &projectImportConfig); err != nil {
		return nil, errors.Wrap(err, "Failed to parse the project configuration; the project body might be malformed")
	}

	// no match; try a multi-project configuration
	if projectImportConfig.Project == nil {
		if err := unmarshalFunc(projectBody, &projectImportConfigs); err != nil {
			return nil, errors.Wrap(err, "Failed to parse the project configuration; the project body might be malformed")
		}

	} else {

		// successfully parsed a single-project configuration
		projectImportConfigs[projectImportConfig.Project.Meta.Name] = &projectImportConfig
	}

	for projectName, importConfig := range projectImportConfigs {
		projectImportOptions[projectName] = &ProjectImportOptions{
			projectImportConfig: importConfig,
		}
	}
	return projectImportOptions, nil
}

func (i *importProjectCommandeer) shouldSkipProject(projectImportConfig *ProjectImportConfig) (bool, error) {
	for _, skipProjectName := range i.skipProjectNames {
		if skipProjectName == projectImportConfig.Project.Meta.Name {
			return true, nil
		}
	}

	// empty by default
	// if we match by empty label selectors, it will match all projects and will simply cause to skip all projects
	if i.skipLabelSelectors != "" {
		match, err := common.LabelsMapMatchByLabelSelector(i.skipLabelSelectors,
			projectImportConfig.Project.Meta.Labels)
		if err != nil {
			return false, errors.Wrap(err, "Failed to match project labels")
		}
		return match, nil

	}
	return false, nil
}

func (i *importProjectCommandeer) enrichProjectImportConfig(projectImportConfig *ProjectImportConfig) {
	i.rootCommandeer.loggerInstance.DebugWith("Enriching project resources",
		"projectNamespace", projectImportConfig.Project.Meta.Namespace,
		"projectName", projectImportConfig.Project.Meta.Name)

	for _, functionConfig := range projectImportConfig.Functions {
		functionConfig.Meta.Namespace = projectImportConfig.Project.Meta.Namespace
		if functionConfig.Meta.Labels != nil {
			functionConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName] = projectImportConfig.Project.Meta.Name
		}
	}

	for _, apiGateway := range projectImportConfig.APIGateways {
		apiGateway.Meta.Namespace = projectImportConfig.Project.Meta.Namespace
		if apiGateway.Meta.Labels != nil {
			apiGateway.Meta.Labels[common.NuclioResourceLabelKeyProjectName] = projectImportConfig.Project.Meta.Name
		}
	}

	for _, functionEvent := range projectImportConfig.FunctionEvents {
		functionEvent.Meta.Namespace = projectImportConfig.Project.Meta.Namespace
		if functionEvent.Meta.Labels != nil {
			functionEvent.Meta.Labels[common.NuclioResourceLabelKeyProjectName] = projectImportConfig.Project.Meta.Name
		}
	}
}

func (i *importProjectCommandeer) importProjectIfMissing(ctx context.Context, projectImportOptions *ProjectImportOptions) (
	platform.Project, error) {

	projectImportConfig := projectImportOptions.projectImportConfig
	projects, err := i.rootCommandeer.platform.GetProjects(ctx, &platform.GetProjectsOptions{
		Meta: projectImportConfig.Project.Meta,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get projects")
	}

	// if not exists, create it
	if len(projects) == 0 {
		newProject, err := platform.NewAbstractProject(i.rootCommandeer.loggerInstance,
			i.rootCommandeer.platform,
			platform.ProjectConfig{
				Meta: projectImportConfig.Project.Meta,
				Spec: projectImportConfig.Project.Spec,
			})
		if err != nil {
			return nil, err
		}

		if err := newProject.CreateAndWait(ctx, &platform.CreateProjectOptions{
			ProjectConfig: newProject.GetConfig(),
		}); err != nil {
			return nil, err
		}

		// get imported project
		return i.getProject(ctx, newProject.GetConfig().Meta.Name, newProject.GetConfig().Meta.Namespace)
	}
	return projects[0], nil
}

func (i *importProjectCommandeer) getProject(ctx context.Context, projectName, projectNamespace string) (platform.Project, error) {
	projects, err := i.rootCommandeer.platform.GetProjects(ctx, &platform.GetProjectsOptions{
		Meta: platform.ProjectMeta{
			Name:      projectName,
			Namespace: projectNamespace,
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get projects")
	}
	if len(projects) == 0 {
		return nil, nuclio.NewErrNotFound("Failed to find project")
	} else if len(projects) > 1 {
		return nil, nuclio.NewErrBadRequest("Found more than one project")
	}

	return projects[0], nil
}
