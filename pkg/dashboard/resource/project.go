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
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	"github.com/nuclio/nuclio/pkg/common"
	nucliocontext "github.com/nuclio/nuclio/pkg/context"
	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/opa"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader/iguazio"
	"github.com/nuclio/nuclio/pkg/platform/kube"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/satori/go.uuid"
)

type projectResource struct {
	*resource
}

type projectInfo struct {
	Meta *platform.ProjectMeta `json:"metadata,omitempty"`
	Spec *platform.ProjectSpec `json:"spec,omitempty"`
}

type projectImportInfo struct {
	Project        *projectInfo
	Functions      map[string]*functionInfo
	FunctionEvents map[string]*functionEventInfo
	APIGateways    map[string]*apiGatewayInfo
}

type ProjectImportOptions struct {
	projectInfo *projectImportInfo
	authConfig  *platform.AuthConfig
}

func (pr *projectResource) ExtendMiddlewares() error {
	pr.resource.addAuthMiddleware()
	return nil
}

// GetAll returns all projects
func (pr *projectResource) GetAll(request *http.Request) (map[string]restful.Attributes, error) {
	ctx := request.Context()
	response := map[string]restful.Attributes{}

	// get namespace
	namespace := pr.getNamespaceFromRequest(request)
	if namespace == "" {
		return nil, nuclio.NewErrBadRequest("Namespace must exist")
	}

	requestOrigin, sessionCookie := pr.getRequestOriginAndSessionCookie(request)
	projects, err := pr.getPlatform().GetProjects(ctx, &platform.GetProjectsOptions{
		Meta: platform.ProjectMeta{
			Name:      request.Header.Get("x-nuclio-project-name"),
			Namespace: namespace,
		},
		PermissionOptions: opa.PermissionOptions{
			MemberIds:           opa.GetUserAndGroupIdsFromAuthSession(pr.getCtxSession(request)),
			OverrideHeaderValue: request.Header.Get(opa.OverrideHeader),
		},
		AuthSession:   pr.getCtxSession(request),
		SessionCookie: sessionCookie,
		RequestOrigin: requestOrigin,
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get projects")
	}

	exportProject := pr.GetURLParamBoolOrDefault(request, restful.ParamExport, false)

	// create a map of attributes keyed by the project id (name)
	for _, project := range projects {
		if exportProject {
			response[project.GetConfig().Meta.Name] = pr.export(request, project)
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

	project, err := pr.getProjectByName(request, id, namespace)
	if err != nil {
		return nil, err
	}

	exportProject := pr.GetURLParamBoolOrDefault(request, restful.ParamExport, false)
	if exportProject {
		return pr.export(request, project), nil
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
		projectImportOptions, responseErr := pr.getProjectImportOptions(request)
		if responseErr != nil {
			return "", nil, responseErr
		}
		projectImportOptions.authConfig = authConfig

		return pr.importProject(request, projectImportOptions)
	}

	projectInfo, responseErr := pr.getProjectInfoFromRequest(request)
	if responseErr != nil {
		return
	}

	return pr.createProject(request, projectInfo)
}

// GetCustomRoutes returns a list of custom routes for the resource
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

func (pr *projectResource) export(request *http.Request, project platform.Project) restful.Attributes {
	projectMeta := project.GetConfig().Meta
	ctx := request.Context()
	pr.Logger.InfoWithCtx(ctx, "Exporting project", "projectName", projectMeta.Name)

	// scrub namespace from project
	projectMeta.Namespace = ""

	projectAttributes := pr.projectToAttributes(project)
	projectAttributes["metadata"] = projectMeta

	attributes := restful.Attributes{
		"project":        projectAttributes,
		"functions":      map[string]restful.Attributes{},
		"functionEvents": map[string]restful.Attributes{},
		"apiGateways":    map[string]restful.Attributes{},
	}

	// get functions and function events to export
	functionsMap, functionEventsMap := pr.getFunctionsAndFunctionEventsMap(request, project)

	// get api-gateways to export
	apiGatewaysMap := pr.getAPIGatewaysMap(ctx, project)

	attributes["functions"] = functionsMap
	attributes["functionEvents"] = functionEventsMap
	attributes["apiGateways"] = apiGatewaysMap

	return attributes
}

func (pr *projectResource) getAPIGatewaysMap(ctx context.Context, project platform.Project) map[string]restful.Attributes {
	getAPIGatewaysOptions := platform.GetAPIGatewaysOptions{Namespace: project.GetConfig().Meta.Namespace}
	apiGatewaysMap, err := apiGatewayResourceInstance.GetAllByNamespace(ctx, &getAPIGatewaysOptions, true)
	if err != nil {
		pr.Logger.WarnWithCtx(ctx, "Failed to get all api-gateways in the namespace",
			"namespace", project.GetConfig().Meta.Namespace,
			"err", err)
	}

	return apiGatewaysMap
}

func (pr *projectResource) getFunctionsAndFunctionEventsMap(request *http.Request, project platform.Project) (map[string]restful.Attributes,
	map[string]restful.Attributes) {

	ctx := request.Context()
	functionsMap := map[string]restful.Attributes{}
	functionEventsMap := map[string]restful.Attributes{}

	getFunctionsOptions := &platform.GetFunctionsOptions{
		Name:      "",
		Namespace: project.GetConfig().Meta.Namespace,
		Labels: fmt.Sprintf("%s=%s",
			common.NuclioResourceLabelKeyProjectName,
			project.GetConfig().Meta.Name),
		AuthSession: pr.getCtxSession(request),
		PermissionOptions: opa.PermissionOptions{
			MemberIds:           opa.GetUserAndGroupIdsFromAuthSession(pr.getCtxSession(request)),
			OverrideHeaderValue: request.Header.Get(opa.OverrideHeader),
		},
	}

	functions, err := pr.getPlatform().GetFunctions(ctx, getFunctionsOptions)

	if err != nil {
		return functionsMap, functionEventsMap
	}

	namespace := project.GetConfig().Meta.Namespace

	// create a map of attributes keyed by the function id (name)
	for _, function := range functions {
		functionsMap[function.GetConfig().Meta.Name] = functionResourceInstance.export(ctx, function)

		functionEvents := functionEventResourceInstance.getFunctionEvents(request, function, namespace)
		for _, functionEvent := range functionEvents {
			functionEventsMap[functionEvent.GetConfig().Meta.Name] =
				functionEventResourceInstance.functionEventToAttributes(functionEvent)
		}
	}

	return functionsMap, functionEventsMap
}

func (pr *projectResource) createProject(request *http.Request, projectInfoInstance *projectInfo) (id string,
	attributes restful.Attributes, responseErr error) {

	ctx := nucliocontext.NewDetached(request.Context())

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

	requestOrigin, sessionCookie := pr.getRequestOriginAndSessionCookie(request)

	// just deploy. the status is async through polling
	pr.Logger.DebugWithCtx(ctx, "Creating project", "newProject", newProject)
	if err := pr.getPlatform().CreateProject(ctx, &platform.CreateProjectOptions{
		ProjectConfig: newProject.GetConfig(),
		RequestOrigin: requestOrigin,
		SessionCookie: sessionCookie,
		AuthSession:   pr.getCtxSession(request),
		PermissionOptions: opa.PermissionOptions{
			OverrideHeaderValue: request.Header.Get(opa.OverrideHeader),
		},

		// TODO: read from request header
		// if false - return "202" and let client to poll on resource until it becomes ready
		WaitForCreateCompletion: true,
	}); err != nil {
		if strings.Contains(errors.Cause(err).Error(), "already exists") {
			return "", nil, nuclio.WrapErrConflict(err)
		}
		return "", nil, err
	}

	// set attributes
	attributes = pr.projectToAttributes(newProject)
	pr.Logger.DebugWithCtx(ctx, "Successfully created project",
		"id", id,
		"attributes", attributes)
	return
}

func (pr *projectResource) getRequestOriginAndSessionCookie(request *http.Request) (platformconfig.ProjectsLeaderKind, *http.Cookie) {
	requestOrigin := platformconfig.ProjectsLeaderKind(request.Header.Get(iguazio.ProjectsRoleHeaderKey))

	// ignore error here, and just return a nil cookie when no session was passed (relevant only on leader/follower mode)
	sessionCookie, _ := request.Cookie("session")

	return requestOrigin, sessionCookie
}

func (pr *projectResource) importProject(request *http.Request, projectImportOptions *ProjectImportOptions) (
	id string, attributes restful.Attributes, responseErr error) {
	ctx := request.Context()
	project, err := pr.importProjectIfMissing(request, projectImportOptions)
	if err != nil {
		return "", nil, err
	}

	// assign imported project
	projectImportOptions.projectInfo.Project = &projectInfo{
		Meta: &project.GetConfig().Meta,
		Spec: &project.GetConfig().Spec,
	}

	// enrich
	pr.enrichProjectImportInfoImportResources(ctx, projectImportOptions.projectInfo)

	// import
	failedFunctions := pr.importProjectFunctions(request, projectImportOptions.projectInfo, projectImportOptions.authConfig)
	failedFunctionEvents := pr.importProjectFunctionEvents(request, projectImportOptions.projectInfo, failedFunctions)
	failedAPIGateways := pr.importProjectAPIGateways(request, projectImportOptions.projectInfo)

	attributes = restful.Attributes{
		"functionImportResult": restful.Attributes{
			"createdAmount":   len(projectImportOptions.projectInfo.Functions) - len(failedFunctions),
			"failedAmount":    len(failedFunctions),
			"failedFunctions": failedFunctions,
		},
		"functionEventImportResult": restful.Attributes{
			"createdAmount":        len(projectImportOptions.projectInfo.FunctionEvents) - len(failedFunctionEvents),
			"failedAmount":         len(failedFunctionEvents),
			"failedFunctionEvents": failedFunctionEvents,
		},
		"apiGatewayImportResult": restful.Attributes{
			"createdAmount":     len(projectImportOptions.projectInfo.APIGateways) - len(failedAPIGateways),
			"failedAmount":      len(failedAPIGateways),
			"failedAPIGateways": failedAPIGateways,
		},
	}

	return
}

func (pr *projectResource) importProjectIfMissing(request *http.Request, projectImportOptions *ProjectImportOptions) (
	platform.Project, error) {

	ctx := request.Context()

	projectName := projectImportOptions.projectInfo.Project.Meta.Name
	projectNamespace := projectImportOptions.projectInfo.Project.Meta.Namespace
	pr.Logger.InfoWithCtx(ctx, "Importing project",
		"projectNamespace", projectNamespace,
		"projectName", projectName)

	projects, err := pr.getPlatform().GetProjects(ctx, &platform.GetProjectsOptions{
		Meta:        *projectImportOptions.projectInfo.Project.Meta,
		AuthSession: pr.getCtxSession(request),
		PermissionOptions: opa.PermissionOptions{
			RaiseForbidden:      true,
			OverrideHeaderValue: request.Header.Get(opa.OverrideHeader),
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get projects")
	}

	// if not exists, create it
	if len(projects) == 0 {
		pr.Logger.DebugWithCtx(ctx, "Project doesn't exist, creating it",
			"projectNamespace", projectNamespace,
			"projectName", projectName)

		// process (enrich/validate) projectInfo instance
		if err := pr.processProjectInfo(projectImportOptions.projectInfo.Project); err != nil {
			return nil, errors.Wrap(err, "Failed to process project info")
		}

		projectConfig := platform.ProjectConfig{
			Meta: *projectImportOptions.projectInfo.Project.Meta,
			Spec: *projectImportOptions.projectInfo.Project.Spec,
		}

		// create a project
		newProject, err := platform.NewAbstractProject(pr.Logger, pr.getPlatform(), projectConfig)
		if err != nil {
			return nil, nuclio.WrapErrInternalServerError(err)
		}

		if err := newProject.CreateAndWait(ctx, &platform.CreateProjectOptions{
			ProjectConfig: newProject.GetConfig(),
			AuthSession:   pr.getCtxSession(request),
			PermissionOptions: opa.PermissionOptions{
				OverrideHeaderValue: request.Header.Get(opa.OverrideHeader),
			},
		}); err != nil {

			// preserve err - it might contain an informative status code (validation failure, etc)
			return nil, err
		}

		// reassign created instance, it holds changes made during project creation
		projectImportOptions.projectInfo.Project.Meta = &newProject.GetConfig().Meta
		projectImportOptions.projectInfo.Project.Spec = &newProject.GetConfig().Spec

		// get imported project
		return pr.getProjectByName(request, newProject.GetConfig().Meta.Name, newProject.GetConfig().Meta.Namespace)
	}
	return projects[0], nil
}

func (pr *projectResource) importProjectFunctions(request *http.Request, projectImportInfoInstance *projectImportInfo,
	authConfig *platform.AuthConfig) []restful.Attributes {

	ctx := request.Context()

	pr.Logger.InfoWithCtx(ctx, "Importing project functions", "project", projectImportInfoInstance.Project.Meta.Name)

	functionCreateChan := make(chan restful.Attributes, len(projectImportInfoInstance.Functions))
	var functionCreateWaitGroup sync.WaitGroup
	functionCreateWaitGroup.Add(len(projectImportInfoInstance.Functions))

	var failedFunctions []restful.Attributes
	for functionName, function := range projectImportInfoInstance.Functions {
		go func(functionName string, function *functionInfo, wg *sync.WaitGroup) {
			function.Meta.Namespace = projectImportInfoInstance.Project.Meta.Namespace
			if function.Meta.Labels == nil {
				function.Meta.Labels = map[string]string{}
			}
			function.Meta.Labels[common.NuclioResourceLabelKeyProjectName] = projectImportInfoInstance.Project.Meta.Name

			if err := pr.importFunction(request, function, authConfig); err != nil {
				pr.Logger.WarnWithCtx(ctx, "Failed importing function upon project import ",
					"functionName", functionName,
					"err", err,
					"projectName", projectImportInfoInstance.Project.Meta.Name)
				functionCreateChan <- restful.Attributes{
					"function": functionName,
					"error":    err.Error(),
				}
			}

			wg.Done()
		}(functionName, function, &functionCreateWaitGroup)
	}

	functionCreateWaitGroup.Wait()
	close(functionCreateChan)

	for creationError := range functionCreateChan {
		if creationError != nil {
			failedFunctions = append(failedFunctions, creationError)
		}
	}

	return failedFunctions
}

func (pr *projectResource) importFunction(request *http.Request, function *functionInfo, authConfig *platform.AuthConfig) error {
	ctx := request.Context()

	pr.Logger.InfoWithCtx(ctx,
		"Importing project function",
		"function", function.Meta.Name,
		"project", function.Meta.Labels[common.NuclioResourceLabelKeyProjectName])
	functions, err := pr.getPlatform().GetFunctions(ctx, &platform.GetFunctionsOptions{
		Name:        function.Meta.Name,
		Namespace:   function.Meta.Namespace,
		AuthSession: pr.getCtxSession(request),
		PermissionOptions: opa.PermissionOptions{
			MemberIds:           opa.GetUserAndGroupIdsFromAuthSession(pr.getCtxSession(request)),
			RaiseForbidden:      true,
			OverrideHeaderValue: request.Header.Get(opa.OverrideHeader),
		},
	})
	if err != nil {
		return errors.New("Failed to get functions")
	}
	if len(functions) > 0 {
		return errors.New("Function name already exists")
	}

	// validation finished successfully - store and deploy the given function
	return functionResourceInstance.storeAndDeployFunction(request, function, authConfig, false)
}

func (pr *projectResource) importProjectAPIGateways(request *http.Request,
	projectImportInfoInstance *projectImportInfo) []restful.Attributes {
	var failedAPIGateways []restful.Attributes

	if projectImportInfoInstance.APIGateways == nil {
		return nil
	}

	// iterate over all api gateways and try to create each
	for _, apiGateway := range projectImportInfoInstance.APIGateways {

		if err := kube.ValidateAPIGatewaySpec(apiGateway.Spec); err != nil {
			failedAPIGateways = append(failedAPIGateways, restful.Attributes{
				"apiGateway": apiGateway.Spec.Name,
				"error":      err.Error(),
			})

			// when it is invalid continue to the next api gateway
			continue
		}

		// create the api gateway
		_, _, err := apiGatewayResourceInstance.createAPIGateway(request, apiGateway)
		if err != nil {
			failedAPIGateways = append(failedAPIGateways, restful.Attributes{
				"apiGateway": apiGateway.Spec.Name,
				"error":      err.Error(),
			})
		}
	}

	return failedAPIGateways
}

func (pr *projectResource) importProjectFunctionEvents(request *http.Request,
	projectImportInfoInstance *projectImportInfo,
	failedFunctions []restful.Attributes) []restful.Attributes {

	creationErrorContainsFunction := func(functionName string) bool {
		for _, functionCreationError := range failedFunctions {
			if functionCreationError["function"] == functionName {
				return true
			}
		}
		return false
	}

	var failedFunctionEvents []restful.Attributes

	for _, functionEvent := range projectImportInfoInstance.FunctionEvents {
		switch functionName, found := functionEvent.Meta.Labels["nuclio.io/function-name"]; {
		case !found:
			failedFunctionEvents = append(failedFunctionEvents, restful.Attributes{
				"functionEvent": functionEvent.Spec.DisplayName,
				"error":         "Event doesn't belong to any function",
			})
		case creationErrorContainsFunction(functionName):
			failedFunctionEvents = append(failedFunctionEvents, restful.Attributes{
				"functionEvent": functionEvent.Spec.DisplayName,
				"error":         fmt.Sprintf("Event belongs to function that failed import: %s", functionName),
			})
		default:

			// generate new name for events to avoid collisions
			functionEvent.Meta.Name = uuid.NewV4().String()

			_, err := functionEventResourceInstance.storeAndDeployFunctionEvent(request, functionEvent)
			if err != nil {
				failedFunctionEvents = append(failedFunctionEvents, restful.Attributes{
					"functionEvent": functionEvent.Spec.DisplayName,
					"error":         err.Error(),
				})
			}
		}
	}
	return failedFunctionEvents
}

func (pr *projectResource) getProjectByName(request *http.Request, projectName, projectNamespace string) (platform.Project, error) {
	ctx := request.Context()

	requestOrigin, sessionCookie := pr.getRequestOriginAndSessionCookie(request)
	projects, err := pr.getPlatform().GetProjects(ctx, &platform.GetProjectsOptions{
		Meta: platform.ProjectMeta{
			Name:      projectName,
			Namespace: projectNamespace,
		},
		AuthSession: pr.getCtxSession(request),
		PermissionOptions: opa.PermissionOptions{
			MemberIds:           opa.GetUserAndGroupIdsFromAuthSession(pr.getCtxSession(request)),
			RaiseForbidden:      true,
			OverrideHeaderValue: request.Header.Get(opa.OverrideHeader),
		},
		RequestOrigin: requestOrigin,
		SessionCookie: sessionCookie,
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get projects")
	}

	if len(projects) == 0 {
		return nil, nuclio.NewErrNotFound("Project not found")
	}
	return projects[0], nil
}

func (pr *projectResource) deleteProject(request *http.Request) (*restful.CustomRouteFuncResponse, error) {
	ctx := request.Context()

	// get project config and status from body
	projectInfo, err := pr.getProjectInfoFromRequest(request)
	if err != nil {
		pr.Logger.WarnWithCtx(ctx, "Failed to get project config and status from body", "err", err)

		return &restful.CustomRouteFuncResponse{
			Single:     true,
			StatusCode: http.StatusBadRequest,
		}, err
	}

	projectDeletionStrategy := request.Header.Get("x-nuclio-delete-project-strategy")
	requestOrigin, sessionCookie := pr.getRequestOriginAndSessionCookie(request)

	if err := pr.getPlatform().DeleteProject(ctx, &platform.DeleteProjectOptions{
		Meta:          *projectInfo.Meta,
		Strategy:      platform.ResolveProjectDeletionStrategyOrDefault(projectDeletionStrategy),
		RequestOrigin: requestOrigin,
		SessionCookie: sessionCookie,
		AuthSession:   pr.getCtxSession(request),
		PermissionOptions: opa.PermissionOptions{
			OverrideHeaderValue: request.Header.Get(opa.OverrideHeader),
		},
	}); err != nil {
		return &restful.CustomRouteFuncResponse{
			Single:     true,
			StatusCode: common.ResolveErrorStatusCodeOrDefault(err, http.StatusInternalServerError),
		}, err
	}

	return &restful.CustomRouteFuncResponse{
		ResourceType: "project",
		Single:       true,
		StatusCode:   http.StatusNoContent,
	}, nil
}

func (pr *projectResource) updateProject(request *http.Request) (*restful.CustomRouteFuncResponse, error) {
	ctx := nucliocontext.NewDetached(request.Context())
	statusCode := http.StatusNoContent

	// get project config and status from body
	projectInfo, err := pr.getProjectInfoFromRequest(request)
	if err != nil {
		pr.Logger.WarnWithCtx(ctx, "Failed to get project config and status from body", "err", err)
		return &restful.CustomRouteFuncResponse{
			Single:     true,
			StatusCode: http.StatusBadRequest,
		}, err
	}

	requestOrigin, sessionCookie := pr.getRequestOriginAndSessionCookie(request)

	if err := pr.getPlatform().UpdateProject(ctx, &platform.UpdateProjectOptions{
		ProjectConfig: platform.ProjectConfig{
			Meta: *projectInfo.Meta,
			Spec: *projectInfo.Spec,
		},
		AuthSession:   pr.getCtxSession(request),
		RequestOrigin: requestOrigin,
		SessionCookie: sessionCookie,
		PermissionOptions: opa.PermissionOptions{
			OverrideHeaderValue: request.Header.Get(opa.OverrideHeader),
		},
	}); err != nil {
		statusCode = common.ResolveErrorStatusCodeOrDefault(err, http.StatusInternalServerError)
		if statusCode > 300 {
			pr.Logger.WarnWithCtx(ctx, "Failed to update project",
				"err", errors.GetErrorStackString(err, 10))
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
		"status":   project.GetConfig().Status,
	}

	return attributes
}

func (pr *projectResource) getNamespaceFromRequest(request *http.Request) string {
	return pr.getNamespaceOrDefault(request.Header.Get("x-nuclio-project-namespace"))
}

func (pr *projectResource) getProjectInfoFromRequest(request *http.Request) (*projectInfo, error) {

	// read body
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		return nil, nuclio.WrapErrInternalServerError(errors.Wrap(err, "Failed to read body"))
	}

	projectInfoInstance := projectInfo{}
	if err := json.Unmarshal(body, &projectInfoInstance); err != nil {
		return nil, nuclio.WrapErrBadRequest(errors.Wrap(err, "Failed to parse JSON body"))
	}

	if err := pr.processProjectInfo(&projectInfoInstance); err != nil {
		return nil, nuclio.WrapErrBadRequest(errors.Wrap(err, "Failed to process project info"))
	}

	return &projectInfoInstance, nil
}

func (pr *projectResource) getProjectImportOptions(request *http.Request) (*ProjectImportOptions, error) {

	// read body
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		return nil, nuclio.WrapErrInternalServerError(errors.Wrap(err, "Failed to read body"))
	}

	projectImportInfoInstance := projectImportInfo{}
	if err = json.Unmarshal(body, &projectImportInfoInstance); err != nil {
		return nil, nuclio.WrapErrBadRequest(errors.Wrap(err, "Failed to parse JSON body"))
	}

	if err = pr.processProjectInfo(projectImportInfoInstance.Project); err != nil {
		return nil, nuclio.WrapErrBadRequest(errors.Wrap(err, "Failed to process project info"))
	}

	return &ProjectImportOptions{
		projectInfo: &projectImportInfoInstance,
	}, nil
}

func (pr *projectResource) processProjectInfo(projectInfoInstance *projectInfo) error {

	// ensure meta
	if projectInfoInstance.Meta == nil {
		projectInfoInstance.Meta = &platform.ProjectMeta{}
	}

	// ensure spec
	if projectInfoInstance.Spec == nil {
		projectInfoInstance.Spec = &platform.ProjectSpec{}
	}

	// ensure namespace
	projectInfoInstance.Meta.Namespace = pr.getNamespaceOrDefault(projectInfoInstance.Meta.Namespace)

	// name must exist
	if projectInfoInstance.Meta.Name == "" {
		return nuclio.NewErrBadRequest("Project name must be provided in metadata")
	}

	// namespace must exists (sanity)
	// TODO: is this really possible considering the fact namespace was enriched beforehand?
	if projectInfoInstance.Meta.Namespace == "" {
		return nuclio.NewErrBadRequest("Project namespace must be provided in metadata")
	}

	return nil
}

func (pr *projectResource) enrichProjectImportInfoImportResources(ctx context.Context,
	projectImportInfoInstance *projectImportInfo) {
	projectName := projectImportInfoInstance.Project.Meta.Name
	projectNamespace := projectImportInfoInstance.Project.Meta.Namespace

	pr.Logger.DebugWithCtx(ctx, "Enriching project resources with project name",
		"projectNamespace", projectNamespace,
		"projectName", projectName)

	for _, functionConfig := range projectImportInfoInstance.Functions {
		if functionConfig.Meta.Labels != nil {
			functionConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName] = projectImportInfoInstance.Project.Meta.Name
		}
	}

	for _, apiGateway := range projectImportInfoInstance.APIGateways {
		if apiGateway.Meta.Labels != nil {
			apiGateway.Meta.Labels[common.NuclioResourceLabelKeyProjectName] = projectImportInfoInstance.Project.Meta.Name
		}
	}

	for _, functionEvent := range projectImportInfoInstance.FunctionEvents {
		if functionEvent.Meta.Labels != nil {
			functionEvent.Meta.Labels[common.NuclioResourceLabelKeyProjectName] = projectImportInfoInstance.Project.Meta.Name
		}
	}
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
