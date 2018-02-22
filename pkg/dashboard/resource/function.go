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
		fr.Logger.WarnWith("Failed to get function config and status from body", "err", err)

		responseErr = nuclio.ErrBadRequest
		return
	}

	go fr.deployFunction(&functionconfig.Config{
		Meta: *functionInfo.Meta,
		Spec: *functionInfo.Spec,
	})

	// in progress
	responseErr = nuclio.ErrAccepted

	return
}

// returns attributes (optionally)
func (fr *functionResource) Update(request *http.Request, id string) (restful.Attributes, error) {

	// get function config and status from body
	functionInfo, err := fr.getFunctionInfoFromRequest(request)
	if err != nil {
		fr.Logger.WarnWith("Failed to get function config and status from body", "err", err)

		return nil, nuclio.ErrBadRequest
	}

	// populate function meta to identify the function we want to configure
	functionMeta := functionconfig.Meta{
		Namespace: fr.getNamespaceFromRequest(request),
		Name:      id,
	}

	err = fr.getPlatform().UpdateFunction(&platform.UpdateOptions{
		FunctionMeta:   &functionMeta,
		FunctionSpec:   functionInfo.Spec,
		FunctionStatus: functionInfo.Status,
	})

	if err != nil {
		fr.Logger.WarnWith("Failed to update function", "err", err)

		return nil, nuclio.ErrInternalServerError
	}

	// done
	return nil, nil
}

// Delete deletes a function
func (fr *functionResource) Delete(request *http.Request, id string) error {
	deleteOptions := platform.DeleteOptions{}
	deleteOptions.FunctionConfig.Meta.Name = id
	deleteOptions.FunctionConfig.Meta.Namespace = fr.getNamespaceFromRequest(request)

	return fr.platform.DeleteFunction(&deleteOptions)
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
		fr.Logger.WarnWith("Failed to read body", "err", err)

		return nil, nuclio.ErrInternalServerError
	}

	functionInfoInstance := functionInfo{}
	err = json.Unmarshal(body, &functionInfoInstance)
	if err != nil {
		fr.Logger.WarnWith("Failed to parse JSON body", "err", err)

		return nil, nuclio.ErrBadRequest
	}

	return &functionInfoInstance, nil
}

func (fr *functionResource) deployFunction(functionConfig *functionconfig.Config) {

	// just deploy. the status is async through polling
	_, err := fr.platform.DeployFunction(&platform.DeployOptions{
		Logger:         fr.Logger,
		FunctionConfig: *functionConfig,
	})

	if err != nil {
		fr.Logger.WarnWith("Failed to deploy function", "err", err)
	}
}

// register the resource
var functionResourceInstance = &functionResource{
	resource: newResource("functions", []restful.ResourceMethod{
		restful.ResourceMethodGetList,
		restful.ResourceMethodGetDetail,
		restful.ResourceMethodCreate,
		restful.ResourceMethodUpdate,
		restful.ResourceMethodDelete,
	}),
}

func init() {
	functionResourceInstance.Resource = functionResourceInstance
	functionResourceInstance.Register(dashboard.DashboardResourceRegistrySingleton)
}
