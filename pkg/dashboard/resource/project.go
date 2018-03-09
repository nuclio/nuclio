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
	"io/ioutil"
	"net/http"

	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/nuclio/nuclio-sdk-go"
)

type projectResource struct {
	*resource
	platform platform.Platform
}

type projectInfo struct {
	Meta   *platform.ProjectMeta   `json:"metadata,omitempty"`
	Spec   *platform.ProjectSpec   `json:"spec,omitempty"`
}

// OnAfterInitialize is called after initialization
func (fr *projectResource) OnAfterInitialize() error {
	fr.platform = fr.getPlatform()

	return nil
}

// GetAll returns all projects
func (fr *projectResource) GetAll(request *http.Request) (map[string]restful.Attributes, error) {
	response := map[string]restful.Attributes{}

	// get namespace
	namespace := fr.getNamespaceFromRequest(request)
	if namespace == "" {
		return nil, nuclio.NewErrBadRequest("Namespace must exist")
	}

	projects, err := fr.platform.GetProjects(&platform.GetProjectsOptions{
		Meta: platform.ProjectMeta{
			Name:      request.Header.Get("x-nuclio-project-name"),
			Namespace: fr.getNamespaceFromRequest(request),
		},
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get projects")
	}

	// create a map of attributes keyed by the project id (name)
	for _, project := range projects {
		response[project.GetConfig().Meta.Name] = fr.projectToAttributes(project)
	}

	return response, nil
}

// GetByID returns a specific project by id
func (fr *projectResource) GetByID(request *http.Request, id string) (restful.Attributes, error) {

	// get namespace
	namespace := fr.getNamespaceFromRequest(request)
	if namespace == "" {
		return nil, nuclio.NewErrBadRequest("Namespace must exist")
	}

	project, err := fr.platform.GetProjects(&platform.GetProjectsOptions{
		Meta: platform.ProjectMeta{
			Name:      id,
			Namespace: fr.getNamespaceFromRequest(request),
		},
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get projects")
	}

	if len(project) == 0 {
		return nil, nil
	}

	return fr.projectToAttributes(project[0]), nil
}

// Create deploys a project
func (fr *projectResource) Create(request *http.Request) (id string, attributes restful.Attributes, responseErr error) {

	projectInfo, responseErr := fr.getProjectInfoFromRequest(request)
	if responseErr != nil {
		return
	}

	// just deploy. the status is async through polling
	err := fr.platform.CreateProject(&platform.CreateProjectOptions{
		ProjectConfig: platform.ProjectConfig{
			Meta: *projectInfo.Meta,
			Spec: *projectInfo.Spec,
		},
	})

	if err != nil {
		return "", nil, nuclio.WrapErrInternalServerError(err)
	}

	return
}

// returns a list of custom routes for the resource
func (fr *projectResource) GetCustomRoutes() ([]restful.CustomRoute, error) {

	// since delete and update by default assume /resource/{id} and we want to get the id/namespace from the body
	// we need to register custom routes
	return []restful.CustomRoute{
		{
			Pattern:   "/",
			Method:    http.MethodPut,
			RouteFunc: fr.updateProject,
		},
		{
			Pattern:   "/",
			Method:    http.MethodDelete,
			RouteFunc: fr.deleteProject,
		},
	}, nil
}

func (fr *projectResource) deleteProject(request *http.Request) (string,
	map[string]restful.Attributes,
	map[string]string,
	bool,
	int,
	error) {

	// get project config and status from body
	projectInfo, err := fr.getProjectInfoFromRequest(request)
	if err != nil {
		fr.Logger.WarnWith("Failed to get project config and status from body", "err", err)

		return "", nil, nil, true, http.StatusBadRequest, err
	}

	deleteProjectOptions := platform.DeleteProjectOptions{}
	deleteProjectOptions.Meta = *projectInfo.Meta

	err = fr.platform.DeleteProject(&deleteProjectOptions)
	if err != nil {
		return "", nil, nil, true, http.StatusInternalServerError, err
	}

	return "project", nil, nil, true, http.StatusNoContent, err
}

func (fr *projectResource) updateProject(request *http.Request) (string,
	map[string]restful.Attributes,
	map[string]string,
	bool,
	int,
	error) {
	//
	//statusCode := http.StatusAccepted
	//
	//// get project config and status from body
	//projectInfo, err := fr.getProjectInfoFromRequest(request)
	//if err != nil {
	//	fr.Logger.WarnWith("Failed to get project config and status from body", "err", err)
	//
	//	return "", nil, nil, true, http.StatusBadRequest, err
	//}
	//
	//doneChan := make(chan bool, 1)
	//
	//go func() {
	//
	//	// populate project meta to identify the project we want to configure
	//	projectMeta := projectconfig.Meta{
	//		Namespace: projectInfo.Meta.Namespace,
	//		Name:      projectInfo.Meta.Name,
	//	}
	//
	//	err = fr.getPlatform().UpdateProject(&platform.UpdateProjectOptions{
	//		ProjectMeta:   &projectMeta,
	//		ProjectSpec:   projectInfo.Spec,
	//		ProjectStatus: projectInfo.Status,
	//	})
	//
	//	if err != nil {
	//		fr.Logger.WarnWith("Failed to update project", "err", err)
	//	}
	//
	//	doneChan <- true
	//}()
	//
	//// mostly for testing, but can also be for clients that want to wait for some reason
	//if request.Header.Get("x-nuclio-wait-project-action") == "true" {
	//	<-doneChan
	//}
	//
	//// if there was an error, try to get the status code
	//if err != nil {
	//	if errWithStatusCode, ok := err.(nuclio.ErrorWithStatusCode); ok {
	//		statusCode = errWithStatusCode.StatusCode()
	//	}
	//}

	//// return the stuff
	// return "project", nil, nil, true, statusCode, err

	return "project", nil, nil, true, 0, nil
}

func (fr *projectResource) projectToAttributes(project platform.Project) restful.Attributes {
	attributes := restful.Attributes{
		"metadata": project.GetConfig().Meta,
		"spec":     project.GetConfig().Spec,
	}

	return attributes
}

func (fr *projectResource) getNamespaceFromRequest(request *http.Request) string {
	return request.Header.Get("x-nuclio-project-namespace")
}

func (fr *projectResource) getProjectInfoFromRequest(request *http.Request) (*projectInfo, error) {

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

	// meta must exist
	if projectInfoInstance.Meta == nil ||
		projectInfoInstance.Meta.Name == "" ||
		projectInfoInstance.Meta.Namespace == "" {
		err := errors.New("Project name and namespace must be provided in metadata")

		return nil, nuclio.WrapErrBadRequest(err)
	}

	return &projectInfoInstance, nil
}

// register the resource
var projectResourceInstance = &projectResource{
	resource: newResource("projects", []restful.ResourceMethod{
		restful.ResourceMethodGetList,
		restful.ResourceMethodGetDetail,
		restful.ResourceMethodCreate,
	}),
}

func init() {
	projectResourceInstance.Resource = projectResourceInstance
	projectResourceInstance.Register(dashboard.DashboardResourceRegistrySingleton)
}
