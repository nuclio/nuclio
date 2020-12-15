package command

import (
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/nuctl/command/common"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/errors"
	uuid "github.com/satori/go.uuid"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

type importCommandeer struct {
	cmd            *cobra.Command
	rootCommandeer *RootCommandeer
}

func newImportCommandeer(rootCommandeer *RootCommandeer) *importCommandeer {
	commandeer := &importCommandeer{
		rootCommandeer: rootCommandeer,
	}

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import functions or projects",
		Long: `Import the configurations of one or more functions or projects
from a configuration file or from the standard input (default)`,
	}

	importFunctionCommand := newImportFunctionCommandeer(commandeer).cmd
	importProjectCommand := newImportProjectCommandeer(commandeer).cmd

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
		file, err := common.OpenFile(filename)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to open file")
		}
		i.cmd.SetIn(file)

		// close file after reading from it
		defer file.Close() // nolint: errcheck
	}

	// read from file if given, fallback to stdin
	return common.ReadFromInOrStdin(i.cmd.InOrStdin())
}

func (i *importCommandeer) importFunction(functionConfig *functionconfig.Config, project *platform.ProjectConfig) error {

	// populate namespace
	functionConfig.Meta.Namespace = project.Meta.Namespace

	if project.Meta.Name != "" {
		functionConfig.Meta.Labels["nuclio.io/project-name"] = project.Meta.Name
	}

	functions, err := i.rootCommandeer.platform.GetFunctions(&platform.GetFunctionsOptions{
		Name:      functionConfig.Meta.Name,
		Namespace: i.rootCommandeer.namespace,
	})
	if err != nil {
		return errors.Wrap(err, "Failed to check existing functions")
	}

	if len(functions) > 0 {
		return errors.Errorf("Function with the name: %s already exists", functionConfig.Meta.Name)
	}

	_, err = i.rootCommandeer.platform.CreateFunction(&platform.CreateFunctionOptions{
		Logger:         i.rootCommandeer.loggerInstance,
		FunctionConfig: *functionConfig,
	})

	return err
}

func (i *importCommandeer) importFunctions(functionConfigs map[string]*functionconfig.Config, project *platform.ProjectConfig) error {
	var errGroup errgroup.Group

	i.rootCommandeer.loggerInstance.DebugWith("Importing functions", "functions", functionConfigs)
	for _, functionConfig := range functionConfigs {
		functionConfig := functionConfig // https://golang.org/doc/faq#closures_and_goroutines
		errGroup.Go(func() error {
			return i.importFunction(functionConfig, project)
		})
	}

	return errGroup.Wait()
}

type importFunctionCommandeer struct {
	*importCommandeer
}

func newImportFunctionCommandeer(importCommandeer *importCommandeer) *importFunctionCommandeer {
	commandeer := &importFunctionCommandeer{
		importCommandeer: importCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "functions [<config file>]",
		Aliases: []string{"function", "fn", "fu"},
		Short:   "(or function) Import functions",
		Long: `(or function) Import the configurations of one or more functions
from a configurations file or from standard input (default)

Note: The command doesn't deploy the imported functions.
      To deploy an imported function, use the 'deploy' command.

Arguments:
  <config file> (string) Path to a function-configurations file in JSON or YAML format (see -o|--output).
                         If not provided, the configuration is imported from standard input (stdin).`,
		RunE: func(cmd *cobra.Command, args []string) error {

			// initialize root
			if err := importCommandeer.rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			functionBody, err := commandeer.resolveInputData(args)
			if err != nil {
				return errors.Wrap(err, "Failed to read function data")
			}

			if len(functionBody) == 0 {
				return errors.New(`Failed to resolve the function-configuration body.
Make sure to provide the content via stdin or a file.
Use --help for more information`)
			}

			unmarshalFunc, err := common.GetUnmarshalFunc(functionBody)
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

			return commandeer.importFunctions(functionConfigs, platformConfig)
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

type ProjectImportConfig struct {
	Project        *platform.ProjectConfig
	Functions      map[string]*functionconfig.Config
	FunctionEvents map[string]*platform.FunctionEventConfig
	APIGateways    map[string]*platform.APIGatewayConfig
}

type importProjectCommandeer struct {
	*importCommandeer
	skipProjectNames []string
}

func newImportProjectCommandeer(importCommandeer *importCommandeer) *importProjectCommandeer {
	commandeer := &importProjectCommandeer{
		importCommandeer: importCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "projects [<config file>]",
		Aliases: []string{"project", "prj", "proj"},
		Short:   "(or project) Import projects (including all functions, function events, and API gateways)",
		Long: `(or project) Import the configurations of one or more projects (including
all project functions, function events, and API gateways) from a configurations file
or from standard input (default)

Note: The command doesn't deploy the functions in the  imported projects.
      To deploy an imported function, use the 'deploy' command.

Arguments:
  <config file> (string) Path to a project-configurations file in JSON or YAML format (see -o|--output).
                         If not provided, the configuration is imported from standard input (stdin).`,
		RunE: func(cmd *cobra.Command, args []string) error {

			// initialize root
			if err := importCommandeer.rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			projectBody, err := commandeer.resolveInputData(args)
			if err != nil {
				return errors.Wrap(err, "Failed to read function data")
			}

			if len(projectBody) == 0 {
				return errors.New(`Failed to resolve the project-configuration body.
Make sure to provide the content via stdin or a file.
Use --help for more information`)
			}

			unmarshalFunc, err := common.GetUnmarshalFunc(projectBody)
			if err != nil {
				return errors.Wrap(err, "Failed to identify the input format")
			}

			projectImportConfigs, err := commandeer.resolveProjectImportConfigs(projectBody, unmarshalFunc)
			if err != nil {
				return errors.Wrap(err, "Failed to resolve the imported project configuration")
			}

			return commandeer.importProjects(projectImportConfigs)
		},
	}

	cmd.Flags().StringSliceVar(&commandeer.skipProjectNames, "skip", []string{}, "Names of projects to skip (don't import), as a comma-separated list")

	commandeer.cmd = cmd

	return commandeer
}

func (i *importProjectCommandeer) importFunctionEvent(functionEvent *platform.FunctionEventConfig) error {
	functions, err := i.rootCommandeer.platform.GetFunctions(&platform.GetFunctionsOptions{
		Name:      functionEvent.Meta.Labels["nuclio.io/function-name"],
		Namespace: i.rootCommandeer.namespace,
	})
	if err != nil {
		return errors.Wrap(err, "Failed to check existing functions")
	}
	if len(functions) == 0 {
		return errors.New("The event function's parent function doesn't exist")
	}

	// generate new name for events to avoid collisions
	functionEvent.Meta.Name = uuid.NewV4().String()

	// populate namespace
	functionEvent.Meta.Namespace = i.rootCommandeer.namespace

	// just deploy; the status is async through polling
	return i.rootCommandeer.platform.CreateFunctionEvent(&platform.CreateFunctionEventOptions{
		FunctionEventConfig: platform.FunctionEventConfig{
			Meta: functionEvent.Meta,
			Spec: functionEvent.Spec,
		},
	})
}

func (i *importProjectCommandeer) importAPIGateway(apiGateway *platform.APIGatewayConfig) error {

	// populate namespace
	apiGateway.Meta.Namespace = i.rootCommandeer.namespace

	// just create; the status is async through polling
	return i.rootCommandeer.platform.CreateAPIGateway(&platform.CreateAPIGatewayOptions{
		APIGatewayConfig: &platform.APIGatewayConfig{
			Meta: apiGateway.Meta,
			Spec: apiGateway.Spec,
		},
	})
}

func (i *importProjectCommandeer) importFunctionEvents(functionEvents map[string]*platform.FunctionEventConfig) error {
	var errGroup errgroup.Group

	i.rootCommandeer.loggerInstance.DebugWith("Importing function events",
		"functionEvents", functionEvents)
	for _, functionEventConfig := range functionEvents {
		functionEventConfig := functionEventConfig // https://golang.org/doc/faq#closures_and_goroutines
		errGroup.Go(func() error {
			return i.importFunctionEvent(functionEventConfig)
		})
	}

	return errGroup.Wait()
}

func (i *importProjectCommandeer) importAPIGateways(apiGateways map[string]*platform.APIGatewayConfig) error {
	var errGroup errgroup.Group

	i.rootCommandeer.loggerInstance.DebugWith("Importing api gateways", "apiGateways", apiGateways)

	if apiGateways == nil {
		return nil
	}

	for _, apiGatewayConfig := range apiGateways {
		apiGatewayConfig := apiGatewayConfig // https://golang.org/doc/faq#closures_and_goroutines
		errGroup.Go(func() error {
			return i.importAPIGateway(apiGatewayConfig)
		})
	}

	return errGroup.Wait()
}

func (i *importProjectCommandeer) importProject(projectConfig *ProjectImportConfig) error {
	var err error
	projects, err := i.rootCommandeer.platform.GetProjects(&platform.GetProjectsOptions{
		Meta: projectConfig.Project.Meta,
	})
	if err != nil {
		return errors.Wrap(err, "Failed to check existing projects")
	}
	if len(projects) == 0 {
		newProject, err := platform.NewAbstractProject(i.rootCommandeer.loggerInstance,
			i.rootCommandeer.platform,
			platform.ProjectConfig{
				Meta: projectConfig.Project.Meta,
				Spec: projectConfig.Project.Spec,
			})
		if err != nil {
			return err
		}
		if err = newProject.CreateAndWait(); err != nil {
			return err
		}
	}

	functionImportErr := i.importFunctions(projectConfig.Functions, projectConfig.Project)
	if functionImportErr != nil {
		i.rootCommandeer.loggerInstance.WarnWith("Failed to import all project functions",
			"functionImportErr", functionImportErr)

		// return this error
		err = functionImportErr
	}

	functionEventImportErr := i.importFunctionEvents(projectConfig.FunctionEvents)
	if functionEventImportErr != nil {
		i.rootCommandeer.loggerInstance.WarnWith("Failed to import all function events",
			"functionEventImportErr", functionEventImportErr)

		// return this err only if not previously set
		if err == nil {
			err = functionEventImportErr
		}
	}

	// api gateways are supported only on k8s platform
	if i.rootCommandeer.platform.GetName() == "kube" {
		apiGatewaysImportErr := i.importAPIGateways(projectConfig.APIGateways)
		if apiGatewaysImportErr != nil {
			i.rootCommandeer.loggerInstance.WarnWith("Unable to import all api gateways",
				"apiGatewaysImportErr", apiGatewaysImportErr)

			// return this err only if not previously set
			if err == nil {
				err = apiGatewaysImportErr
			}
		}
	}

	return err
}

func (i *importProjectCommandeer) importProjects(projectImportConfigs map[string]*ProjectImportConfig) error {
	i.rootCommandeer.loggerInstance.DebugWith("Importing projects",
		"projects", projectImportConfigs,
		"skipProjectNames", i.skipProjectNames)

	// TODO: parallel this with errorGroup, mutex is required due to multi map writers
	for _, projectConfig := range projectImportConfigs {
		if i.shouldSkipProject(projectConfig) {
			i.rootCommandeer.loggerInstance.DebugWith("Skipping import for project",
				"projectName", projectConfig.Project.Meta.Name)
			continue
		}

		i.rootCommandeer.loggerInstance.DebugWith("Importing project",
			"projectMeta", projectConfig.Project.Meta)

		if projectConfig.Project.Meta.Namespace == "" {
			projectConfig.Project.Meta.Namespace = i.rootCommandeer.namespace
		}
		if err := i.importProject(projectConfig); err != nil {
			return errors.Wrap(err, "Failed to import project")
		}
	}
	return nil
}

func (i *importProjectCommandeer) resolveProjectImportConfigs(projectBody []byte,
	unmarshalFunc func(data []byte, v interface{}) error) (map[string]*ProjectImportConfig, error) {

	// initialize
	projectImportConfigs := map[string]*ProjectImportConfig{}

	// try a single-project configuration
	projectImportConfig := ProjectImportConfig{}
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

	return projectImportConfigs, nil
}

func (i *importProjectCommandeer) shouldSkipProject(projectConfig *ProjectImportConfig) bool {
	for _, skipProjectName := range i.skipProjectNames {
		if skipProjectName == projectConfig.Project.Meta.Name {
			return true
		}
	}
	return false
}
