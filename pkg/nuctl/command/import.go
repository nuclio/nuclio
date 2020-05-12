package command

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/ghodss/yaml"
	"github.com/nuclio/errors"
	uuid "github.com/satori/go.uuid"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

type importCommandeer struct {
	cmd            *cobra.Command
	rootCommandeer *RootCommandeer
	deploy         bool
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

func (i *importCommandeer) addImportCommandFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&i.deploy, "deploy", false, "Deploy the function or functions after import, false by default")
}

func (i *importCommandeer) readFromStdinOrFile(args []string) ([]byte, error) {
	if len(args) >= 1 {
		return ioutil.ReadFile(args[0])
	}
	return ioutil.ReadAll(os.Stdin)
}

func (i *importCommandeer) getUnmarshalFunc(bytes []byte) (func(data []byte, v interface{}) error, error) {
	var err error
	var obj map[string]interface{}

	if err = json.Unmarshal(bytes, &obj); err == nil {
		return json.Unmarshal, nil
	}

	if err = yaml.Unmarshal(bytes, &obj); err == nil {
		return yaml.Unmarshal, nil
	}

	return nil, errors.New("Input is neither json nor yaml")
}

func (i *importCommandeer) importFunction(functionConfig *functionconfig.Config, deploy bool, project string) error {

	// populate namespace
	functionConfig.Meta.Namespace = i.rootCommandeer.namespace

	if project != "" {
		functionConfig.Meta.Labels["nuclio.io/project-name"] = project
	}

	if deploy {

		// Remove skip annotations
		functionConfig.Meta.RemoveSkipBuildAnnotation()
		functionConfig.Meta.RemoveSkipDeployAnnotation()
	} else {
		functionConfig.AddSkipAnnotations()
	}

	functions, err := i.rootCommandeer.platform.GetFunctions(&platform.GetFunctionsOptions{
		Name:      functionConfig.Meta.Name,
		Namespace: i.rootCommandeer.namespace,
	})
	if err != nil {
		return errors.Wrap(err, "Failed to check existing functions")
	}

	if len(functions) > 0 {
		return errors.New(fmt.Sprintf("Function with the name: %s already exists", functionConfig.Meta.Name))
	}

	_, err = i.rootCommandeer.platform.CreateFunction(&platform.CreateFunctionOptions{
		Logger:         i.rootCommandeer.loggerInstance,
		FunctionConfig: *functionConfig,
	})

	return err
}

func (i *importCommandeer) importFunctions(functionConfigs map[string]*functionconfig.Config,
	deploy bool,
	project string) error {
	var g errgroup.Group

	for _, functionConfig := range functionConfigs {
		functionConfig := functionConfig // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			return i.importFunction(functionConfig, deploy, project)
		})
	}

	return g.Wait()
}

type importFunctionCommandeer struct {
	*importCommandeer
	functionConfigs map[string]*functionconfig.Config
}

func newImportFunctionCommandeer(importCommandeer *importCommandeer) *importFunctionCommandeer {
	commandeer := &importFunctionCommandeer{
		importCommandeer: importCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "function [path-to-exported-function-file]",
		Aliases: []string{"fu"},
		Short:   "Import function, and by default don't deploy it",
		RunE: func(cmd *cobra.Command, args []string) error {
			funcBytes, err := commandeer.readFromStdinOrFile(args)
			if err != nil {
				return errors.Wrap(err, "Failed to read function data")
			}

			// initialize root
			if err := importCommandeer.rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			unmarshalFunc, err := commandeer.getUnmarshalFunc(funcBytes)
			if err != nil {
				return errors.Wrap(err, "Failed identifying input format")
			}

			// First try parsing multiple functions
			err = commandeer.parseMultipleFunctionsImport(funcBytes, commandeer.functionConfigs, unmarshalFunc)
			if err != nil {

				// If that fails, try parsing a single function
				commandeer.functionConfigs = map[string]*functionconfig.Config{}
				err = commandeer.parseFunctionImport(funcBytes, commandeer.functionConfigs, unmarshalFunc)
				if err != nil {
					return errors.Wrap(err, "Failed to parse function data")
				}
			}

			return commandeer.importFunctions(commandeer.functionConfigs, commandeer.deploy, "")
		},
	}

	commandeer.addImportCommandFlags(cmd)

	commandeer.cmd = cmd

	return commandeer
}

func (i *importFunctionCommandeer) parseFunctionImport(funcBytes []byte,
	functionConfigs map[string]*functionconfig.Config,
	unmarshalFunc func(data []byte, v interface{}) error) error {

	functionConfig := &functionconfig.Config{}
	if err := unmarshalFunc(funcBytes, functionConfig); err != nil {
		return errors.Wrap(err, "Failed encoding function import config")
	}

	functionConfigs[functionConfig.Meta.Name] = functionConfig

	return nil
}

func (i *importFunctionCommandeer) parseMultipleFunctionsImport(funcBytes []byte,
	functionConfigs map[string]*functionconfig.Config,
	unmarshalFunc func(data []byte, v interface{}) error) error {
	if err := unmarshalFunc(funcBytes, functionConfigs); err != nil {
		return errors.Wrap(err, "Failed encoding function import config")
	}

	return nil
}

type ProjectImportConfig struct {
	Project        platform.ProjectConfig
	Functions      map[string]*functionconfig.Config
	FunctionEvents map[string]*platform.FunctionEventConfig
}

type importProjectCommandeer struct {
	*importCommandeer
	projectImportConfigs map[string]*ProjectImportConfig
}

func newImportProjectCommandeer(importCommandeer *importCommandeer) *importProjectCommandeer {
	commandeer := &importProjectCommandeer{
		importCommandeer: importCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "project [path-to-exported-project-file]",
		Aliases: []string{"proj"},
		Short:   "Import project and all its functions and functionEvents",
		RunE: func(cmd *cobra.Command, args []string) error {
			projBytes, err := commandeer.readFromStdinOrFile(args)
			if err != nil {
				return errors.Wrap(err, "Failed to read function data")
			}

			// initialize root
			if err := importCommandeer.rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			unmarshalFunc, err := commandeer.getUnmarshalFunc(projBytes)
			if err != nil {
				return errors.Wrap(err, "Failed identifying input format")
			}

			// First try parsing multiple projects
			err = commandeer.parseMultipleProjectsImport(projBytes, commandeer.projectImportConfigs, unmarshalFunc)
			if err != nil {

				// If that fails, try parsing a single project
				commandeer.projectImportConfigs = map[string]*ProjectImportConfig{}
				err = commandeer.parseProjectImport(projBytes, commandeer.projectImportConfigs, unmarshalFunc)

				if err != nil {
					return errors.Wrap(err, "Failed to parse function data")
				}
			}

			return commandeer.importProjects(commandeer.projectImportConfigs, commandeer.deploy)
		},
	}

	commandeer.addImportCommandFlags(cmd)

	commandeer.cmd = cmd

	return commandeer
}

func (i *importProjectCommandeer) parseProjectImport(projBytes []byte,
	projectConfigs map[string]*ProjectImportConfig,
	unmarshalFunc func(data []byte, v interface{}) error) error {

	projectConfig := &ProjectImportConfig{}
	if err := unmarshalFunc(projBytes, projectConfig); err != nil {
		return errors.Wrap(err, "Failed encoding function import config")
	}

	projectConfigs[projectConfig.Project.Meta.Name] = projectConfig

	return nil
}

func (i *importProjectCommandeer) parseMultipleProjectsImport(projBytes []byte,
	projectConfigs map[string]*ProjectImportConfig,
	unmarshalFunc func(data []byte, v interface{}) error) error {
	if err := unmarshalFunc(projBytes, projectConfigs); err != nil {
		return errors.Wrap(err, "Failed encoding function import config")
	}

	return nil
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
	var g errgroup.Group

	for _, functionEventConfig := range functionEvents {
		functionEventConfig := functionEventConfig // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			return i.importFunctionEvent(functionEventConfig)
		})
	}

	return g.Wait()
}

func (i *importProjectCommandeer) importProject(projectConfig *ProjectImportConfig, deploy bool) error {
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

	functionImportErr := i.importFunctions(projectConfig.Functions, deploy, projectConfig.Project.Meta.Name)
	functionEventImportErr := i.importFunctionEvents(projectConfig.FunctionEvents)

	if functionImportErr != nil {
		return errors.Wrap(functionImportErr, "Failed to import some functions")
	}
	if functionEventImportErr != nil {
		return errors.Wrap(functionImportErr, "Failed to import some function events")
	}

	return nil
}

func (i *importProjectCommandeer) importProjects(projectImportConfigs map[string]*ProjectImportConfig, deploy bool) error {
	var g errgroup.Group

	for _, projectConfig := range projectImportConfigs {
		g.Go(func() error {
			return i.importProject(projectConfig, deploy)
		})
	}

	return g.Wait()
}
