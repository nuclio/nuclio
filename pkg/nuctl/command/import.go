package command

import (
	"encoding/json"
	"fmt"
	"github.com/ghodss/yaml"
	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	uuid "github.com/satori/go.uuid"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"
)

const (
	InputFormatJSON = OutputFormatJSON
	InputFormatYAML = OutputFormatYAML
)

type importCommandeer struct {
	cmd            *cobra.Command
	rootCommandeer *RootCommandeer
	format         string
	deploy         bool
	multiple       bool
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
	cmd.Flags().StringVarP(&i.format, "input-format", "i", InputFormatJSON, "Input format - \"yaml\", or \"json\"")
	cmd.Flags().BoolVar(&i.deploy, "deploy", false, "Deploy the function or functions after import, false by default")
	cmd.Flags().BoolVarP(&i.multiple, "multiple", "m", false, "Input contains multiple functions/projects to import")
}

func (i *importCommandeer) readFromStdinOrFile(args []string) ([]byte, error) {
	if len(args) >= 1 {
		return ioutil.ReadFile(args[0])
	} else {
		return ioutil.ReadAll(os.Stdin)
	}
}

func (i *importCommandeer) getUnmarshalFunc(format string) func(data []byte, v interface{}) error {
	switch format {
	case InputFormatJSON:
		return json.Unmarshal
	case InputFormatYAML:
		return yaml.Unmarshal
	}
	return nil
}

func (i *importCommandeer) mapParallel(objMap map[string]interface{}, mapFunc func(obj interface{}, errCh chan error)) error {
	var wg sync.WaitGroup
	wg.Add(len(objMap))

	errCh := make(chan error, len(objMap))
	var mapErrorStrings []string

	for _, obj := range objMap {
		go func(obj interface{}, errCh chan error, wg *sync.WaitGroup) {
			mapFunc(obj, errCh)
			wg.Done()
		}(obj, errCh, &wg)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			mapErrorStrings = append(mapErrorStrings, err.Error())
		}
	}

	if len(mapErrorStrings) != 0 {
		return errors.New(strings.Join(mapErrorStrings, "\n"))
	}

	return nil
}

func (i *importCommandeer) importFunction(functionConfig functionconfig.Config, deploy bool, project string) error {

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

		// Ensure skip annotations exist
		if functionConfig.Meta.Annotations == nil {
			functionConfig.Meta.Annotations = map[string]string{}
		}
		functionConfig.Meta.Annotations[functionconfig.FunctionAnnotationSkipBuild] = strconv.FormatBool(true)
		functionConfig.Meta.Annotations[functionconfig.FunctionAnnotationSkipDeploy] = strconv.FormatBool(true)
	}

	functions, err := i.rootCommandeer.platform.GetFunctions(&platform.GetFunctionsOptions{
		Name:      functionConfig.Meta.Name,
		Namespace: i.rootCommandeer.namespace,
	})
	if err != nil {
		return errors.Wrap(err, "Failed to check existing functions")
	}

	if len(functions) > 0 {
		return errors.New(fmt.Sprintf("function with the name: %s already exists", functionConfig.Meta.Name))
	}

	_, err = i.rootCommandeer.platform.CreateFunction(&platform.CreateFunctionOptions{
		Logger:         i.rootCommandeer.loggerInstance,
		FunctionConfig: functionConfig,
	})

	return err
}

func (i *importCommandeer) importFunctions(functionConfigs map[string]functionconfig.Config,
	deploy bool,
	project string) error {
	interfaceMap := map[string]interface{}{}
	for functionName, functionConfig := range functionConfigs {
		interfaceMap[functionName] = functionConfig
	}

	return i.mapParallel(interfaceMap, func(obj interface{}, errCh chan error) {
		functionConfig, ok := obj.(functionconfig.Config)
		if !ok {
			errCh <- errors.New("Failed to assert function config")
			return
		}

		errCh <- i.importFunction(functionConfig, deploy, project)
	})
}

type importFunctionCommandeer struct {
	*importCommandeer
	functionConfigs map[string]functionconfig.Config
}

func newImportFunctionCommandeer(importCommandeer *importCommandeer) *importFunctionCommandeer {
	commandeer := &importFunctionCommandeer{
		importCommandeer: importCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "function function-config-file",
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

			unmarshalFunc := commandeer.getUnmarshalFunc(commandeer.format)

			if commandeer.multiple {
				err = commandeer.parseMultipleFunctionsImport(funcBytes, &commandeer.functionConfigs, unmarshalFunc)
			} else {
				commandeer.functionConfigs = map[string]functionconfig.Config{}
				err = commandeer.parseFunctionImport(funcBytes, &commandeer.functionConfigs, unmarshalFunc)
			}
			if err != nil {
				return errors.Wrap(err, "Failed to parse function data")
			}

			return commandeer.importFunctions(commandeer.functionConfigs, commandeer.deploy, "")
		},
	}

	commandeer.addImportCommandFlags(cmd)

	commandeer.cmd = cmd

	return commandeer
}

func (i *importFunctionCommandeer) parseFunctionImport(funcBytes []byte,
	functionConfigs *map[string]functionconfig.Config,
	unmarshalFunc func(data []byte, v interface{}) error) error {

	functionConfig := &functionconfig.Config{}
	if err := unmarshalFunc(funcBytes, functionConfig); err != nil {
		return errors.Wrap(err, "Failed encoding function import config")
	}

	(*functionConfigs)[functionConfig.Meta.Name] = *functionConfig

	return nil
}

func (i *importFunctionCommandeer) parseMultipleFunctionsImport(funcBytes []byte,
	functionConfigs *map[string]functionconfig.Config,
	unmarshalFunc func(data []byte, v interface{}) error) error {
	if err := unmarshalFunc(funcBytes, functionConfigs); err != nil {
		return errors.Wrap(err, "Failed encoding function import config")
	}

	return nil
}

type projectImportConfig struct {
	Project        platform.ProjectConfig
	Functions      map[string]functionconfig.Config
	FunctionEvents map[string]platform.FunctionEventConfig
}

type importProjectCommandeer struct {
	*importCommandeer
	projectImportConfigs map[string]projectImportConfig
}

func newImportProjectCommandeer(importCommandeer *importCommandeer) *importProjectCommandeer {
	commandeer := &importProjectCommandeer{
		importCommandeer: importCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "project project-config-file",
		Aliases: []string{"proj"},
		Short:   "Import project, all it's functions and functionEvents",
		RunE: func(cmd *cobra.Command, args []string) error {
			projBytes, err := commandeer.readFromStdinOrFile(args)
			if err != nil {
				return errors.Wrap(err, "Failed to read function data")
			}

			// initialize root
			if err := importCommandeer.rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			unmarshalFunc := commandeer.getUnmarshalFunc(commandeer.format)

			if commandeer.multiple {
				err = commandeer.parseMultipleProjectsImport(projBytes, &commandeer.projectImportConfigs, unmarshalFunc)
			} else {
				commandeer.projectImportConfigs = map[string]projectImportConfig{}
				err = commandeer.parseProjectImport(projBytes, &commandeer.projectImportConfigs, unmarshalFunc)
			}
			if err != nil {
				return errors.Wrap(err, "Failed to parse function data")
			}

			return commandeer.importProjects(commandeer.projectImportConfigs, commandeer.deploy)
		},
	}

	commandeer.addImportCommandFlags(cmd)

	commandeer.cmd = cmd

	return commandeer
}

func (i *importProjectCommandeer) parseProjectImport(projBytes []byte,
	projectConfigs *map[string]projectImportConfig,
	unmarshalFunc func(data []byte, v interface{}) error) error {

	projectConfig := &projectImportConfig{}
	if err := unmarshalFunc(projBytes, projectConfig); err != nil {
		return errors.Wrap(err, "Failed encoding function import config")
	}

	(*projectConfigs)[projectConfig.Project.Meta.Name] = *projectConfig

	return nil
}

func (i *importProjectCommandeer) parseMultipleProjectsImport(projBytes []byte,
	projectConfigs *map[string]projectImportConfig,
	unmarshalFunc func(data []byte, v interface{}) error) error {
	if err := unmarshalFunc(projBytes, projectConfigs); err != nil {
		return errors.Wrap(err, "Failed encoding function import config")
	}

	return nil
}

func (i *importProjectCommandeer) importFunctionEvent(functionEvent platform.FunctionEventConfig) error {
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

func (i *importProjectCommandeer) importFunctionEvents(functionEvents map[string]platform.FunctionEventConfig) error {
	interfaceMap := map[string]interface{}{}
	for functionEventName, functionEventConfig := range functionEvents {
		interfaceMap[functionEventName] = functionEventConfig
	}

	return i.mapParallel(interfaceMap, func(obj interface{}, errCh chan error) {
		functionEventConfig, ok := obj.(platform.FunctionEventConfig)
		if !ok {
			errCh <- errors.New("Failed to assert project config")
			return
		}

		errCh <- i.importFunctionEvent(functionEventConfig)
	})
}

func (i *importProjectCommandeer) importProject(projectConfig projectImportConfig, deploy bool) error {
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

	var importErrorStrings []string

	if err = i.importFunctions(projectConfig.Functions, deploy, projectConfig.Project.Meta.Name); err != nil {
		importErrorStrings = append(importErrorStrings, err.Error())
	}
	if err = i.importFunctionEvents(projectConfig.FunctionEvents); err != nil {
		importErrorStrings = append(importErrorStrings, err.Error())
	}

	if len(importErrorStrings) != 0 {
		return errors.New(strings.Join(importErrorStrings, "\n"))
	}

	return nil
}

func (i *importProjectCommandeer) importProjects(projectImportConfigs map[string]projectImportConfig, deploy bool) error {
	interfaceMap := map[string]interface{}{}
	for projectName, projectConfig := range projectImportConfigs {
		interfaceMap[projectName] = projectConfig
	}

	return i.mapParallel(interfaceMap, func(obj interface{}, errCh chan error) {
		projectConfig, ok := obj.(projectImportConfig)
		if !ok {
			errCh <- errors.New("Failed to assert project config")
			return
		}

		errCh <- i.importProject(projectConfig, deploy)
	})
}
