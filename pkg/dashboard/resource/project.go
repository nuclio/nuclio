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

package resource

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
)

type projectResource struct {
	*resource
}

type projectInfo struct {
	Meta *platform.ProjectMeta `json:"metadata,omitempty"`
	Spec *platform.ProjectSpec `json:"spec,omitempty"`
}

type functionImportMap map[string]*functionInfo

func (fim *functionImportMap) UnmarshalJSON(data []byte) error {
	var functionImports map[string]*struct {
		Function *functionInfo
	}
	if err := json.Unmarshal(data, &functionImports); err != nil {
		return err
	}

	unmarshalFunctionImports := map[string]*functionInfo{}
	for functionName, functionImport := range functionImports {
		unmarshalFunctionImports[functionName] = functionImport.Function
	}

	*fim = unmarshalFunctionImports
	return nil
}

type projectImportInfo struct {
	Project   *projectInfo
	Functions functionImportMap
}

const (
	ProjectGetUponCreationTimeout       = 30 * time.Second
	ProjectGetUponCreationRetryInterval = 1 * time.Second
)

// GetAll returns all projects
func (pr *projectResource) GetAll(request *http.Request) (map[string]restful.Attributes, error) {
	response := map[string]restful.Attributes{}

	// get namespace
	namespace := pr.getNamespaceFromRequest(request)
	if namespace == "" {
		return nil, nuclio.NewErrBadRequest("Namespace must exist")
	}

	projects, err := pr.getPlatform().GetProjects(&platform.GetProjectsOptions{
		Meta: platform.ProjectMeta{
			Name:      request.Header.Get("x-nuclio-project-name"),
			Namespace: pr.getNamespaceFromRequest(request),
		},
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get projects")
	}

	exportProject := pr.GetURLParamBoolOrDefault(request, restful.ParamExport, false)

	// create a map of attributes keyed by the project id (name)
	for _, project := range projects {
		if exportProject {
			response[project.GetConfig().Meta.Name] = pr.export(project)
		} else {
			response[project.GetConfig().Meta.Name] = pr.projectToAttributes(project)
		}
	}

	return response, nil
}

// GetByID returns a specific project by id
func (pr *projectResource) GetByID(request *http.Request, id string) (restful.Attributes, error) {

	// get namespace
	namespace := pr.getNamespaceFromRequest(request)
	if namespace == "" {
		return nil, nuclio.NewErrBadRequest("Namespace must exist")
	}

	projects, err := pr.getPlatform().GetProjects(&platform.GetProjectsOptions{
		Meta: platform.ProjectMeta{
			Name:      id,
			Namespace: pr.getNamespaceFromRequest(request),
		},
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get projects")
	}

	if len(projects) == 0 {
		return nil, nuclio.NewErrNotFound("Project not found")
	}
	project := projects[0]

	exportProject := pr.GetURLParamBoolOrDefault(request, restful.ParamExport, false)
	if exportProject {
		return pr.export(project), nil
	}

	return pr.projectToAttributes(project), nil
}

// Create deploys a project
func (pr *projectResource) Create(request *http.Request) (id string, attributes restful.Attributes, responseErr error) {

	// get the authentication configuration for the request
	authConfig, responseErr := pr.getRequestAuthConfig(request)
	if responseErr != nil {
		return
	}

	importProject := pr.GetURLParamBoolOrDefault(request, restful.ParamImport, false)
	if importProject {
		projectImportInfo, responseErr := pr.getProjectImportInfoFromRequest(request)
		if responseErr != nil {
			return "", nil, responseErr
		}

		return pr.importProject(projectImportInfo, authConfig)
	}

	projectInfo, responseErr := pr.getProjectInfoFromRequest(request, true)
	if responseErr != nil {
		return
	}

	return pr.createProject(projectInfo)
}

// returns a list of custom routes for the resource
func (pr *projectResource) GetCustomRoutes() ([]restful.CustomRoute, error) {

	// since delete and update by default assume /resource/{id} and we want to get the id/namespace from the body
	// we need to register custom routes
	return []restful.CustomRoute{
		{
			Pattern:   "/",
			Method:    http.MethodPut,
			RouteFunc: pr.updateProject,
		},
		{
			Pattern:   "/",
			Method:    http.MethodDelete,
			RouteFunc: pr.deleteProject,
		},
	}, nil
}

func (pr *projectResource) export(project platform.Project) restful.Attributes {
	projectMeta := project.GetConfig().Meta

	pr.Logger.InfoWith("Exporting project", "projectName", projectMeta.Name)

	// scrub namespace from project
	projectMeta.Namespace = ""

	projectAttributes := pr.projectToAttributes(project)
	projectAttributes["metadata"] = projectMeta

	attributes := restful.Attributes{
		"project":   projectAttributes,
		"functions": map[string]restful.Attributes{},
	}

	functionsMap := map[string]restful.Attributes{}

	getFunctionsOptions := &platform.GetFunctionsOptions{
		Name:      "",
		Namespace: project.GetConfig().Meta.Namespace,
		Labels:    fmt.Sprintf("nuclio.io/project-name=%s", projectMeta.Name),
	}

	functions, err := pr.getPlatform().GetFunctions(getFunctionsOptions)

	if err != nil {
		return attributes
	}

	// create a map of attributes keyed by the function id (name)
	for _, function := range functions {
		functionsMap[function.GetConfig().Meta.Name] = restful.Attributes{
			"function": functionResourceInstance.export(function),
			"events":   functionResourceInstance.exportFunctionEvents(function),
		}
	}

	attributes["functions"] = functionsMap

	return attributes
}

func (pr *projectResource) createProject(projectInfoInstance *projectInfo) (id string,
	attributes restful.Attributes, responseErr error) {

	// create a project config
	projectConfig := platform.ProjectConfig{
		Meta: *projectInfoInstance.Meta,
		Spec: *projectInfoInstance.Spec,
	}

	// create a project
	newProject, err := platform.NewAbstractProject(pr.Logger, pr.getPlatform(), projectConfig)
	if err != nil {
		return "", nil, nuclio.WrapErrInternalServerError(err)
	}

	// just deploy. the status is async through polling
	err = pr.getPlatform().CreateProject(&platform.CreateProjectOptions{
		ProjectConfig: *newProject.GetConfig(),
	})

	if err != nil {
		if strings.Contains(errors.Cause(err).Error(), "already exists") {
			return "", nil, nuclio.WrapErrConflict(err)
		}

		return "", nil, nuclio.WrapErrInternalServerError(err)
	}

	// set attributes
	attributes = pr.projectToAttributes(newProject)

	return
}

func (pr *projectResource) importProject(projectImportInfoInstance *projectImportInfo, authConfig *platform.AuthConfig) (id string,
	attributes restful.Attributes, responseErr error) {

	pr.Logger.InfoWith("Importing project", "projectName", projectImportInfoInstance.Project.Meta.Name)
	pr.Logger.DebugWith("Checking if project exists", "projectName", projectImportInfoInstance.Project.Meta.Name)

	projects, err := pr.getPlatform().GetProjects(&platform.GetProjectsOptions{
		Meta: *projectImportInfoInstance.Project.Meta,
	})
	if err != nil || len(projects) == 0 {
		pr.Logger.InfoWith("Project doesn't exist, creating it", "project", projectImportInfoInstance.Project.Meta.Name)
		err = pr.createAndWaitForProjectCreation(projectImportInfoInstance.Project)
		if err != nil {
			return "", nil, nuclio.WrapErrInternalServerError(err)
		}
	}

	attributes = pr.importProjectFunctions(projectImportInfoInstance, authConfig)

	return
}

func (pr *projectResource) importProjectFunctions(projectImportInfoInstance *projectImportInfo,
	authConfig *platform.AuthConfig) restful.Attributes {

	pr.Logger.InfoWith("Importing project functions", "project", projectImportInfoInstance.Project.Meta.Name)

	functionCreateChan := make(chan restful.Attributes, len(projectImportInfoInstance.Functions))
	var functionCreateWaitGroup sync.WaitGroup
	functionCreateWaitGroup.Add(len(projectImportInfoInstance.Functions))

	var failedFunctions []restful.Attributes
	for functionName, functionImport := range projectImportInfoInstance.Functions {
		go func(functionName string, functionImport *functionInfo, wg *sync.WaitGroup) {
			functionImport.Meta.Namespace = projectImportInfoInstance.Project.Meta.Namespace
			if functionImport.Meta.Labels == nil {
				functionImport.Meta.Labels = map[string]string{}
			}
			functionImport.Meta.Labels["nuclio.io/project-name"] = projectImportInfoInstance.Project.Meta.Name

			if err := pr.importFunction(functionImport, authConfig); err != nil {
				pr.Logger.WarnWith("Failed importing function upon project import ",
					"functionName", functionName,
					"err", err,
					"projectName", projectImportInfoInstance.Project.Meta.Name)
				functionCreateChan <- restful.Attributes{
					"function": functionName,
					"error":    err.Error(),
				}
			}

			wg.Done()
		}(functionName, functionImport, &functionCreateWaitGroup)
	}

	functionCreateWaitGroup.Wait()
	close(functionCreateChan)

	for creationError := range functionCreateChan {
		if creationError != nil {
			failedFunctions = append(failedFunctions, creationError)
		}
	}

	return restful.Attributes{
		"createdFunctionsAmount": len(projectImportInfoInstance.Functions) - len(failedFunctions),
		"failedFunctionsAmount":  len(failedFunctions),
		"failedFunctions":        failedFunctions,
	}
}

func (pr *projectResource) importFunction(functionInfo *functionInfo, authConfig *platform.AuthConfig) error {
	pr.Logger.InfoWith("Importing project function",
		"function", functionInfo.Meta.Name,
		"project", functionInfo.Meta.Labels["nuclio.io/project-name"])
	functions, err := pr.getPlatform().GetFunctions(&platform.GetFunctionsOptions{
		Name:      functionInfo.Meta.Name,
		Namespace: functionInfo.Meta.Namespace,
	})
	if err != nil {
		return errors.New("Failed to get functions")
	}
	if len(functions) > 0 {
		return errors.New("Function name already exists")
	}

	// validation finished successfully - store and deploy the given function
	return functionResourceInstance.storeAndDeployFunction(functionInfo, authConfig, false)
}

func (pr *projectResource) createAndWaitForProjectCreation(projectInfoInstance *projectInfo) error {

	// create a project config
	projectConfig := platform.ProjectConfig{
		Meta: *projectInfoInstance.Meta,
		Spec: *projectInfoInstance.Spec,
	}

	// create a project
	newProject, err := platform.NewAbstractProject(pr.Logger, pr.getPlatform(), projectConfig)
	if err != nil {
		return nuclio.WrapErrInternalServerError(err)
	}

	// just deploy. the status is async through polling
	err = pr.getPlatform().CreateProject(&platform.CreateProjectOptions{
		ProjectConfig: *newProject.GetConfig(),
	})
	if err != nil {
		return nuclio.WrapErrInternalServerError(err)
	}

	err = common.RetryUntilSuccessful(ProjectGetUponCreationTimeout, ProjectGetUponCreationRetryInterval, func() bool {
		pr.Logger.DebugWith("Trying to get created project",
			"projectName", projectConfig.Meta.Name,
			"timeout", ProjectGetUponCreationTimeout,
			"retryInterval", ProjectGetUponCreationRetryInterval)
		projects, err := pr.getPlatform().GetProjects(&platform.GetProjectsOptions{
			Meta: *projectInfoInstance.Meta,
		})
		return err == nil && len(projects) > 0
	})
	if err != nil {
		return nuclio.WrapErrInternalServerError(errors.Wrapf(err,
			"Failed to wait for a created project %s",
			projectConfig.Meta.Name))
	}

	return nil
}

func (pr *projectResource) deleteProject(request *http.Request) (*restful.CustomRouteFuncResponse, error) {

	// get project config and status from body
	projectInfo, err := pr.getProjectInfoFromRequest(request, true)
	if err != nil {
		pr.Logger.WarnWith("Failed to get project config and status from body", "err", err)

		return &restful.CustomRouteFuncResponse{
			Single:     true,
			StatusCode: http.StatusBadRequest,
		}, err
	}

	deleteProjectOptions := platform.DeleteProjectOptions{}
	deleteProjectOptions.Meta = *projectInfo.Meta

	err = pr.getPlatform().DeleteProject(&deleteProjectOptions)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if errWithStatus, ok := err.(*nuclio.ErrorWithStatusCode); ok {
			statusCode = errWithStatus.StatusCode()
		}

		return &restful.CustomRouteFuncResponse{
			Single:     true,
			StatusCode: statusCode,
		}, err
	}

	return &restful.CustomRouteFuncResponse{
		ResourceType: "project",
		Single:       true,
		StatusCode:   http.StatusNoContent,
	}, err
}

func (pr *projectResource) updateProject(request *http.Request) (*restful.CustomRouteFuncResponse, error) {

	statusCode := http.StatusNoContent

	// get project config and status from body
	projectInfo, err := pr.getProjectInfoFromRequest(request, true)
	if err != nil {
		pr.Logger.WarnWith("Failed to get project config and status from body", "err", err)

		return &restful.CustomRouteFuncResponse{
			Single:     true,
			StatusCode: http.StatusBadRequest,
		}, err
	}

	projectConfig := platform.ProjectConfig{
		Meta: *projectInfo.Meta,
		Spec: *projectInfo.Spec,
	}

	err = pr.getPlatform().UpdateProject(&platform.UpdateProjectOptions{
		ProjectConfig: projectConfig,
	})

	if err != nil {
		pr.Logger.WarnWith("Failed to update project", "err", err)
	}

	// if there was an error, try to get the status code
	if err != nil {
		if errWithStatusCode, ok := err.(nuclio.ErrorWithStatusCode); ok {
			statusCode = errWithStatusCode.StatusCode()
		}
	}

	// return the stuff
	return &restful.CustomRouteFuncResponse{
		ResourceType: "project",
		Single:       true,
		StatusCode:   statusCode,
	}, err
}

func (pr *projectResource) projectToAttributes(project platform.Project) restful.Attributes {
	attributes := restful.Attributes{
		"metadata": project.GetConfig().Meta,
		"spec":     project.GetConfig().Spec,
	}

	return attributes
}

func (pr *projectResource) getNamespaceFromRequest(request *http.Request) string {
	return pr.getNamespaceOrDefault(request.Header.Get("x-nuclio-project-namespace"))
}

func (pr *projectResource) getProjectInfoFromRequest(request *http.Request, nameRequired bool) (*projectInfo, error) {

	// read body
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		return nil, nuclio.WrapErrInternalServerError(errors.Wrap(err, "Failed to read body"))
	}

	projectInfoInstance := projectInfo{}
	err = json.Unmarshal(body, &projectInfoInstance)
	if err != nil {
		return nil, nuclio.WrapErrBadRequest(errors.Wrap(err, "Failed to parse JSON body"))
	}

	err = pr.processProjectInfo(&projectInfoInstance, nameRequired)
	if err != nil {
		return nil, nuclio.WrapErrBadRequest(errors.Wrap(err, "Failed to process project info"))
	}

	return &projectInfoInstance, nil
}

func (pr *projectResource) getProjectImportInfoFromRequest(request *http.Request) (*projectImportInfo, error) {

	// read body
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		return nil, nuclio.WrapErrInternalServerError(errors.Wrap(err, "Failed to read body"))
	}

	projectImportInfoInstance := projectImportInfo{}
	if err = json.Unmarshal(body, &projectImportInfoInstance); err != nil {
		return nil, nuclio.WrapErrBadRequest(errors.Wrap(err, "Failed to parse JSON body"))
	}

	if err = pr.processProjectInfo(projectImportInfoInstance.Project, true); err != nil {
		return nil, nuclio.WrapErrBadRequest(errors.Wrap(err, "Failed to process project info"))
	}

	return &projectImportInfoInstance, nil
}

func (pr *projectResource) processProjectInfo(projectInfoInstance *projectInfo, nameRequired bool) error {

	// override namespace if applicable
	if projectInfoInstance.Meta != nil {
		projectInfoInstance.Meta.Namespace = pr.getNamespaceOrDefault(projectInfoInstance.Meta.Namespace)
	}

	// meta must exist
	if projectInfoInstance.Meta == nil ||
		(nameRequired && projectInfoInstance.Meta.Name == "") ||
		projectInfoInstance.Meta.Namespace == "" {
		err := errors.New("Project name must be provided in metadata")

		return nuclio.WrapErrBadRequest(err)
	}

	return nil
}

// register the resource
var projectResourceInstance = &projectResource{
	resource: newResource("api/projects", []restful.ResourceMethod{
		restful.ResourceMethodGetList,
		restful.ResourceMethodGetDetail,
		restful.ResourceMethodCreate,
	}),
}

func init() {
	projectResourceInstance.Resource = projectResourceInstance
	projectResourceInstance.Register(dashboard.DashboardResourceRegistrySingleton)
}
