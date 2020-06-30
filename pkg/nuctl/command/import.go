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
		Short: "Import function/project",
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

func (i *importCommandeer) importFunction(functionConfig *functionconfig.Config, project string) error {

	// populate namespace
	functionConfig.Meta.Namespace = i.rootCommandeer.namespace

	if project != "" {
		functionConfig.Meta.Labels["nuclio.io/project-name"] = project
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

func (i *importCommandeer) importFunctions(functionConfigs map[string]*functionconfig.Config, project string) error {
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
		Use:     "functions [path-to-exported-function-file]",
		Aliases: []string{"fu", "fn", "function"},
		Short:   "(or function) Import function, and by default don't deploy it",
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
				return errors.New(`Failed to resolve function body.
Make sure to provide its content via STDIN / file path.
Use --help for more information`)
			}

			unmarshalFunc, err := common.GetUnmarshalFunc(functionBody)
			if err != nil {
				return errors.Wrap(err, "Failed identifying input format")
			}

			functionConfigs, err := commandeer.resolveFunctionImportConfigs(functionBody, unmarshalFunc)
			if err != nil {
				return errors.Wrap(err, "Failed to resolve function import configs")
			}

			return commandeer.importFunctions(functionConfigs, "")
		},
	}

	commandeer.cmd = cmd

	return commandeer
}

func (i *importFunctionCommandeer) resolveFunctionImportConfigs(functionBody []byte,
	unmarshalFunc func(data []byte, v interface{}) error) (map[string]*functionconfig.Config, error) {

	// initialize
	functionConfigs := map[string]*functionconfig.Config{}

	// try single
	functionConfig := &functionconfig.Config{}
	if err := unmarshalFunc(functionBody, &functionConfig); err != nil {
		return nil, errors.Wrap(err, "Failed to parse single project config")
	}

	// no match, try multi
	if functionConfig.Meta.Name == "" {
		if err := unmarshalFunc(functionBody, &functionConfigs); err != nil {
			return nil, errors.Wrap(err, "Failed to parse multi projects data")
		}

	} else {

		// successfully parsed a single-project
		functionConfigs[functionConfig.Meta.Name] = functionConfig
	}

	return functionConfigs, nil
}

type ProjectImportConfig struct {
	Project        *platform.ProjectConfig
	Functions      map[string]*functionconfig.Config
	FunctionEvents map[string]*platform.FunctionEventConfig
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
		Use:     "projects [path-to-exported-project-file]",
		Aliases: []string{"proj", "prj", "project"},
		Short:   "(or project) Import project and all its functions and functionEvents",
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
				return errors.New(`Failed to resolve project body.
Make sure to provide its content via STDIN / file path.
Use --help for more information`)
			}

			unmarshalFunc, err := common.GetUnmarshalFunc(projectBody)
			if err != nil {
				return errors.Wrap(err, "Failed identifying input format")
			}

			projectImportConfigs, err := commandeer.resolveProjectImportConfigs(projectBody, unmarshalFunc)
			if err != nil {
				return errors.Wrap(err, "Failed to resolve project import configs")
			}

			return commandeer.importProjects(projectImportConfigs)
		},
	}

	cmd.Flags().StringSliceVar(&commandeer.skipProjectNames, "skip", []string{}, "Project names to skip (comma separated)")

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
		return errors.New("Function event's function doesn't exist")
	}

	// generate new name for events to avoid collisions
	functionEvent.Meta.Name = uuid.NewV4().String()

	// populate namespace
	functionEvent.Meta.Namespace = i.rootCommandeer.namespace

	// just deploy. the status is async through polling
	return i.rootCommandeer.platform.CreateFunctionEvent(&platform.CreateFunctionEventOptions{
		FunctionEventConfig: platform.FunctionEventConfig{
			Meta: functionEvent.Meta,
			Spec: functionEvent.Spec,
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

	functionImportErr := i.importFunctions(projectConfig.Functions, projectConfig.Project.Meta.Name)
	if functionImportErr != nil {
		i.rootCommandeer.loggerInstance.WarnWith("Unable to import all functions",
			"functionImportErr", functionImportErr)

		// return this error
		err = functionImportErr
	}

	functionEventImportErr := i.importFunctionEvents(projectConfig.FunctionEvents)
	if functionEventImportErr != nil {
		i.rootCommandeer.loggerInstance.WarnWith("Unable to import all function events",
			"functionEventImportErr", functionEventImportErr)

		// return this err only if not previously set
		if err == nil {
			err = functionEventImportErr
		}
	}

	return err
}

func (i *importProjectCommandeer) importProjects(projectImportConfigs map[string]*ProjectImportConfig) error {
	i.rootCommandeer.loggerInstance.DebugWith("Importing projects", "projects", projectImportConfigs, "skipProjectNames", i.skipProjectNames)

	// TODO: parallel this with errorGroup, mutex is required due to multi map writers
	for _, projectConfig := range projectImportConfigs {
		if i.shouldSkipProject(projectConfig) {
			i.rootCommandeer.loggerInstance.DebugWith("Skipping import for project",
				"projectName", projectConfig.Project.Meta.Name)
			continue
		}

		i.rootCommandeer.loggerInstance.DebugWith("Importing project",
			"projectName", projectConfig.Project.Meta.Name)
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

	// try single
	projectImportConfig := ProjectImportConfig{}
	if err := unmarshalFunc(projectBody, &projectImportConfig); err != nil {
		return nil, errors.Wrap(err, "Failed to parse project config (is project body malformed?)")
	}

	// no match, try multi
	if projectImportConfig.Project == nil {
		if err := unmarshalFunc(projectBody, &projectImportConfigs); err != nil {
			return nil, errors.Wrap(err, "Failed to parse project configs (is project body malformed?)")
		}

	} else {

		// successfully parsed a single-project
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
