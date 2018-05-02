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
	"time"

	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/nuclio/nuclio-sdk-go"
)

type functionResource struct {
	*resource
}

type functionInfo struct {
	Meta   *functionconfig.Meta   `json:"metadata,omitempty"`
	Spec   *functionconfig.Spec   `json:"spec,omitempty"`
	Status *functionconfig.Status `json:"status,omitempty"`
}

// GetAll returns all functions
func (fr *functionResource) GetAll(request *http.Request) (map[string]restful.Attributes, error) {
	response := map[string]restful.Attributes{}

	// get namespace
	namespace := fr.getNamespaceFromRequest(request)
	if namespace == "" {
		return nil, nuclio.NewErrBadRequest("Namespace must exist")
	}

	getFunctionsOptions := &platform.GetFunctionsOptions{
		Name:      request.Header.Get("x-nuclio-function-name"),
		Namespace: fr.getNamespaceFromRequest(request),
	}

	// if the user wants to filter by project, do that
	projectNameFilter := request.Header.Get("x-nuclio-project-name")
	if projectNameFilter != "" {
		getFunctionsOptions.Labels = fmt.Sprintf("nuclio.io/project-name=%s", projectNameFilter)
	}

	functions, err := fr.getPlatform().GetFunctions(getFunctionsOptions)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get functions")
	}

	// create a map of attributes keyed by the function id (name)
	for _, function := range functions {
		response[function.GetConfig().Meta.Name] = fr.functionToAttributes(function)
	}

	return response, nil
}

// GetByID returns a specific function by id
func (fr *functionResource) GetByID(request *http.Request, id string) (restful.Attributes, error) {

	// get namespace
	namespace := fr.getNamespaceFromRequest(request)
	if namespace == "" {
		return nil, nuclio.NewErrBadRequest("Namespace must exist")
	}

	function, err := fr.getPlatform().GetFunctions(&platform.GetFunctionsOptions{
		Namespace: fr.getNamespaceFromRequest(request),
		Name:      id,
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get functions")
	}

	if len(function) == 0 {
		return nil, nil
	}

	return fr.functionToAttributes(function[0]), nil
}

// Create deploys a function
func (fr *functionResource) Create(request *http.Request) (id string, attributes restful.Attributes, responseErr error) {

	functionInfo, responseErr := fr.getFunctionInfoFromRequest(request)
	if responseErr != nil {
		return
	}

	readinessTimeout := 60 * time.Second
	creationStateUpdatedTimeout := 15 * time.Second

	doneChan := make(chan bool, 1)
	creationStateUpdatedChan := make(chan bool, 1)
	errDeployingChan := make(chan error, 1)

	// asynchronously, do the deploy so that the user doesn't wait
	go func() {
		dashboardServer := fr.GetServer().(*dashboard.Server)

		// if registry / run-registry aren't set - use dashboard settings
		if functionInfo.Spec.Build.Registry == "" {
			functionInfo.Spec.Build.Registry = dashboardServer.GetRegistryURL()
		}

		if functionInfo.Spec.RunRegistry == "" {
			functionInfo.Spec.RunRegistry = dashboardServer.GetRunRegistryURL()
		}

		functionInfo.Spec.Build.NoBaseImagesPull = dashboardServer.NoPullBaseImages

		// just deploy. the status is async through polling
		_, err := fr.getPlatform().CreateFunction(&platform.CreateFunctionOptions{
			Logger: fr.Logger,
			FunctionConfig: functionconfig.Config{
				Meta: *functionInfo.Meta,
				Spec: *functionInfo.Spec,
			},
			ReadinessTimeout:     &readinessTimeout,
			CreationStateUpdated: creationStateUpdatedChan,
		})

		if err != nil {
			fr.Logger.WarnWith("Failed to deploy function", "err", err)
			errDeployingChan <- err
		}

		doneChan <- true
	}()

	// wait until the function is in "creating" state. we must return only once the correct function state
	// will be returned on an immediate get. for example, if the function exists and is in "ready" state, we don't
	// want to return before the function's state is in "building"
	select {
	case <-creationStateUpdatedChan:
		break
	case errDeploying := <-errDeployingChan:
		responseErr = errDeploying
		return
	case <-time.After(creationStateUpdatedTimeout):
		responseErr = nuclio.NewErrInternalServerError("Timed out waiting for creation state to be set")
		return
	}

	// mostly for testing, but can also be for clients that want to wait for some reason
	if request.Header.Get("x-nuclio-wait-function-action") == "true" {
		<-doneChan
	}

	// in progress
	responseErr = nuclio.ErrAccepted

	return
}

// returns a list of custom routes for the resource
func (fr *functionResource) GetCustomRoutes() ([]restful.CustomRoute, error) {

	// since delete and update by default assume /resource/{id} and we want to get the id/namespace from the body
	// we need to register custom routes
	return []restful.CustomRoute{
		//{
		//	Pattern:   "/",
		//	Method:    http.MethodPut,
		//	RouteFunc: fr.updateFunction,
		//},
		{
			Pattern:   "/",
			Method:    http.MethodDelete,
			RouteFunc: fr.deleteFunction,
		},
	}, nil
}

func (fr *functionResource) deleteFunction(request *http.Request) (*restful.CustomRouteFuncResponse, error) {

	// get function config and status from body
	functionInfo, err := fr.getFunctionInfoFromRequest(request)
	if err != nil {
		fr.Logger.WarnWith("Failed to get function config and status from body", "err", err)

		return &restful.CustomRouteFuncResponse{
			Single:     true,
			StatusCode: http.StatusBadRequest,
		}, err
	}

	deleteFunctionOptions := platform.DeleteFunctionOptions{}
	deleteFunctionOptions.FunctionConfig.Meta = *functionInfo.Meta

	err = fr.getPlatform().DeleteFunction(&deleteFunctionOptions)
	if err != nil {
		return &restful.CustomRouteFuncResponse{
			Single:     true,
			StatusCode: http.StatusInternalServerError,
		}, err
	}

	return &restful.CustomRouteFuncResponse{
		ResourceType: "function",
		Single:       true,
		StatusCode:   http.StatusNoContent,
	}, err
}

func (fr *functionResource) updateFunction(request *http.Request) (*restful.CustomRouteFuncResponse, error) {

	statusCode := http.StatusAccepted

	// get function config and status from body
	functionInfo, err := fr.getFunctionInfoFromRequest(request)
	if err != nil {
		fr.Logger.WarnWith("Failed to get function config and status from body", "err", err)

		return &restful.CustomRouteFuncResponse{
			Single:     true,
			StatusCode: http.StatusBadRequest,
		}, err
	}

	doneChan := make(chan bool, 1)

	go func() {

		// populate function meta to identify the function we want to configure
		functionMeta := functionconfig.Meta{
			Namespace: functionInfo.Meta.Namespace,
			Name:      functionInfo.Meta.Name,
		}

		err = fr.getPlatform().UpdateFunction(&platform.UpdateFunctionOptions{
			FunctionMeta:   &functionMeta,
			FunctionSpec:   functionInfo.Spec,
			FunctionStatus: functionInfo.Status,
		})

		if err != nil {
			fr.Logger.WarnWith("Failed to update function", "err", err)
		}

		doneChan <- true
	}()

	// mostly for testing, but can also be for clients that want to wait for some reason
	if request.Header.Get("x-nuclio-wait-function-action") == "true" {
		<-doneChan
	}

	// if there was an error, try to get the status code
	if err != nil {
		if errWithStatusCode, ok := err.(nuclio.ErrorWithStatusCode); ok {
			statusCode = errWithStatusCode.StatusCode()
		}
	}

	// return the stuff
	return &restful.CustomRouteFuncResponse{
		ResourceType: "function",
		Single:       true,
		StatusCode:   statusCode,
	}, err
}

func (fr *functionResource) functionToAttributes(function platform.Function) restful.Attributes {
	attributes := restful.Attributes{
		"metadata": function.GetConfig().Meta,
		"spec":     function.GetConfig().Spec,
	}

	status := function.GetStatus()
	if status != nil {
		attributes["status"] = status
	}

	return attributes
}

func (fr *functionResource) getNamespaceFromRequest(request *http.Request) string {
	return request.Header.Get("x-nuclio-function-namespace")
}

func (fr *functionResource) getFunctionInfoFromRequest(request *http.Request) (*functionInfo, error) {

	// read body
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		return nil, nuclio.WrapErrInternalServerError(errors.Wrap(err, "Failed to read body"))
	}

	functionInfoInstance := functionInfo{}
	err = json.Unmarshal(body, &functionInfoInstance)
	if err != nil {
		return nil, nuclio.WrapErrBadRequest(errors.Wrap(err, "Failed to parse JSON body"))
	}

	// meta must exist
	if functionInfoInstance.Meta == nil ||
		functionInfoInstance.Meta.Name == "" ||
		functionInfoInstance.Meta.Namespace == "" {
		err := errors.New("Function name and namespace must be provided in metadata")

		return nil, nuclio.WrapErrBadRequest(err)
	}

	// add project name label if given via header
	projectName := request.Header.Get("x-nuclio-project-name")
	if projectName != "" {
		if functionInfoInstance.Meta.Labels == nil {
			functionInfoInstance.Meta.Labels = map[string]string{}
		}

		functionInfoInstance.Meta.Labels["nuclio.io/project-name"] = projectName
	}

	return &functionInfoInstance, nil
}

// register the resource
var functionResourceInstance = &functionResource{
	resource: newResource("api/functions", []restful.ResourceMethod{
		restful.ResourceMethodGetList,
		restful.ResourceMethodGetDetail,
		restful.ResourceMethodCreate,
	}),
}

func init() {
	functionResourceInstance.Resource = functionResourceInstance
	functionResourceInstance.Register(dashboard.DashboardResourceRegistrySingleton)
}
