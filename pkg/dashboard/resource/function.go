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
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/nuclio/nuclio-sdk-go"
)

type functionResource struct {
	*resource
	platform platform.Platform
}

type functionInfo struct {
	Meta   *functionconfig.Meta   `json:"metadata,omitempty"`
	Spec   *functionconfig.Spec   `json:"spec,omitempty"`
	Status *functionconfig.Status `json:"status,omitempty"`
}

// OnAfterInitialize is called after initialization
func (fr *functionResource) OnAfterInitialize() error {
	fr.platform = fr.getPlatform()

	return nil
}

// GetAll returns all functions
func (fr *functionResource) GetAll(request *http.Request) (map[string]restful.Attributes, error) {
	response := map[string]restful.Attributes{}

	// get namespace
	namespace := fr.getNamespaceFromRequest(request)
	if namespace == "" {
		return nil, nuclio.NewErrBadRequest("Namespace must exist")
	}

	functions, err := fr.platform.GetFunctions(&platform.GetOptions{
		Name:      request.Header.Get("x-nuclio-function-name"),
		Namespace: fr.getNamespaceFromRequest(request),
	})

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

	function, err := fr.platform.GetFunctions(&platform.GetOptions{
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

	functionInfo, err := fr.getFunctionInfoFromRequest(request)
	if err != nil {
		responseErr = nuclio.WrapErrBadRequest(errors.Wrap(err,
			"Failed to get function config and status from body"))
		return
	}

	// asynchronously, do the deploy so that the user doesn't wait
	go func() {

		// just deploy. the status is async through polling
		_, err := fr.platform.DeployFunction(&platform.DeployOptions{
			Logger:         fr.Logger,
			FunctionConfig: functionconfig.Config{
				Meta: *functionInfo.Meta,
				Spec: *functionInfo.Spec,
			},
		})

		if err != nil {
			fr.Logger.WarnWith("Failed to deploy function", "err", err)
		}

	}()

	// in progress
	responseErr = &nuclio.ErrAccepted

	return
}

// returns a list of custom routes for the resource
func (fr *functionResource) GetCustomRoutes() ([]restful.CustomRoute, error) {

	// since delete and update by default assume /resource/{id} and we want to get the id/namespace from the body
	// we need to register custom routes
	return []restful.CustomRoute{
		{
			Pattern:   "/",
			Method:    http.MethodPut,
			RouteFunc: fr.updateFunction,
		},
		{
			Pattern:   "/",
			Method:    http.MethodDelete,
			RouteFunc: fr.deleteFunction,
		},
	}, nil
}

func (fr *functionResource) deleteFunction(request *http.Request) (string,
	map[string]restful.Attributes,
	bool,
	int,
	error) {

	// get function config and status from body
	functionInfo, err := fr.getFunctionInfoFromRequest(request)
	if err != nil {
		fr.Logger.WarnWith("Failed to get function config and status from body", "err", err)

		return "", nil, true, http.StatusBadRequest, err
	}

	deleteOptions := platform.DeleteOptions{}
	deleteOptions.FunctionConfig.Meta = *functionInfo.Meta

	fr.platform.DeleteFunction(&deleteOptions)

	return "function", map[string]restful.Attributes{"": nil}, true, http.StatusOK, err
}

func (fr *functionResource) updateFunction(request *http.Request) (string,
	map[string]restful.Attributes,
	bool,
	int,
	error) {

	statusCode := http.StatusAccepted

	// get function config and status from body
	functionInfo, err := fr.getFunctionInfoFromRequest(request)
	if err != nil {
		fr.Logger.WarnWith("Failed to get function config and status from body", "err", err)

		return "", nil, true, http.StatusBadRequest, err
	}

	go func() {

		// populate function meta to identify the function we want to configure
		functionMeta := functionconfig.Meta{
			Namespace: functionInfo.Meta.Namespace,
			Name:      functionInfo.Meta.Name,
		}

		err = fr.getPlatform().UpdateFunction(&platform.UpdateOptions{
			FunctionMeta:   &functionMeta,
			FunctionSpec:   functionInfo.Spec,
			FunctionStatus: functionInfo.Status,
		})

		if err != nil {
			fr.Logger.WarnWith("Failed to update function", "err", err)
		}
	}()

	// if there was an error, try to get the status code
	if err != nil {
		if errWithStatusCode, ok := err.(*nuclio.ErrorWithStatusCode); ok {
			statusCode = errWithStatusCode.StatusCode()
		}
	}

	// return the stuff
	return "function", map[string]restful.Attributes{"": nil}, true, statusCode, err
}

func (fr *functionResource) functionToAttributes(function platform.Function) restful.Attributes {
	return restful.Attributes{
		"metadata": function.GetConfig().Meta,
		"spec":     function.GetConfig().Spec,
		"status":   function.GetStatus(),
	}
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
		return nil, nuclio.WrapErrBadRequest(errors.Wrap(err,
			"Function name and namespace must be provided in metadata"))
	}

	return &functionInfoInstance, nil
}

// register the resource
var functionResourceInstance = &functionResource{
	resource: newResource("functions", []restful.ResourceMethod{
		restful.ResourceMethodGetList,
		restful.ResourceMethodGetDetail,
		restful.ResourceMethodCreate,
	}),
}

func init() {
	functionResourceInstance.Resource = functionResourceInstance
	functionResourceInstance.Register(dashboard.DashboardResourceRegistrySingleton)
}
