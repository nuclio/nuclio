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

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/dashboard/opa"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/satori/go.uuid"
)

type functionEventResource struct {
	*resource
}

type functionEventInfo struct {
	Meta *platform.FunctionEventMeta `json:"metadata,omitempty"`
	Spec *platform.FunctionEventSpec `json:"spec,omitempty"`
}

// GetAll returns all function events
func (fer *functionEventResource) GetAll(request *http.Request) (map[string]restful.Attributes, error) {
	response := map[string]restful.Attributes{}

	// get namespace
	namespace := fer.getNamespaceFromRequest(request)
	if namespace == "" {
		return nil, nuclio.NewErrBadRequest("Namespace must exist")
	}

	functionEventName := request.Header.Get("x-nuclio-function-event-name")
	getFunctionEventOptions := platform.GetFunctionEventsOptions{
		Meta: platform.FunctionEventMeta{
			Name:      functionEventName,
			Namespace: fer.getNamespaceFromRequest(request),
		},
	}

	// get function name
	functionName := fer.getFunctionNameFromRequest(request)
	if functionName != "" {
		getFunctionEventOptions.Meta.Labels = map[string]string{
			"nuclio.io/function-name": functionName,
		}
	}

	functionEvents, err := fer.getPlatform().GetFunctionEvents(&getFunctionEventOptions)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function events")
	}

	// create a map of attributes keyed by the function event id (name)
	for _, functionEvent := range functionEvents {

		// check opa permissions for resource
		function, err := fer.getFunctionEventFunction(functionEvent)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get function for function event")
		}
		allowed, err := fer.queryOPAFunctionEventPermissions(request,
			function.GetConfig().Meta.Labels["nuclio.io/project-name"],
			function.GetConfig().Meta.Name,
			functionEvent.GetConfig().Meta.Name,
			opa.ActionRead,
			false)
		if err != nil {
			return nil, err
		}
		if !allowed {
			continue
		}

		response[functionEvent.GetConfig().Meta.Name] = fer.functionEventToAttributes(functionEvent)
	}

	return response, nil
}

// GetByID returns a specific function event by id
func (fer *functionEventResource) GetByID(request *http.Request, id string) (restful.Attributes, error) {

	// get namespace
	namespace := fer.getNamespaceFromRequest(request)
	if namespace == "" {
		return nil, nuclio.NewErrBadRequest("Namespace must exist")
	}

	functionEvent, err := fer.getPlatform().GetFunctionEvents(&platform.GetFunctionEventsOptions{
		Meta: platform.FunctionEventMeta{
			Name:      id,
			Namespace: fer.getNamespaceFromRequest(request),
		},
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function event")
	}

	if len(functionEvent) == 0 {
		return nil, nuclio.NewErrNotFound("Function event not found")
	}

	// check opa permissions for resource
	_, err = fer.queryOPAFunctionEventPermissions(request,
		functionEvent[0].GetConfig().Meta.Labels["nuclio.io/project-name"],
		functionEvent[0].GetConfig().Meta.Labels["nuclio.io/function-name"],
		id,
		opa.ActionRead,
		true)
	if err != nil {
		return nil, err
	}

	return fer.functionEventToAttributes(functionEvent[0]), nil
}

// Create deploys a function event
func (fer *functionEventResource) Create(request *http.Request) (id string, attributes restful.Attributes, responseErr error) {

	functionEventInfo, responseErr := fer.getFunctionEventInfoFromRequest(request, false)
	if responseErr != nil {
		return
	}

	// if the name wasn't specified, generate something
	if functionEventInfo.Meta.Name == "" {
		functionEventInfo.Meta.Name = uuid.NewV4().String()
	}

	// check opa permissions for resource
	_, err := fer.queryOPAFunctionEventPermissions(request,
		functionEventInfo.Meta.Labels["nuclio.io/project-name"],
		functionEventInfo.Meta.Labels["nuclio.io/function-name"],
		functionEventInfo.Meta.Name,
		opa.ActionCreate,
		true)
	if err != nil {
		return "", nil, err
	}

	newFunctionEvent, err := fer.storeAndDeployFunctionEvent(functionEventInfo)
	if err != nil {
		return "", nil, nuclio.WrapErrInternalServerError(err)
	}

	// set attributes
	attributes = fer.functionEventToAttributes(newFunctionEvent)

	return
}

// returns a list of custom routes for the resource
func (fer *functionEventResource) GetCustomRoutes() ([]restful.CustomRoute, error) {

	// since delete and update by default assume /resource/{id} and we want to get the id/namespace from the body
	// we need to register custom routes
	return []restful.CustomRoute{
		{
			Pattern:   "/",
			Method:    http.MethodPut,
			RouteFunc: fer.updateFunctionEvent,
		},
		{
			Pattern:   "/",
			Method:    http.MethodDelete,
			RouteFunc: fer.deleteFunctionEvent,
		},
	}, nil
}

func (fer *functionEventResource) storeAndDeployFunctionEvent(functionEvent *functionEventInfo) (platform.FunctionEvent, error) {

	// create a functionEvent config
	functionEventConfig := platform.FunctionEventConfig{
		Meta: *functionEvent.Meta,
		Spec: *functionEvent.Spec,
	}

	// create a functionEvent
	newFunctionEvent, err := platform.NewAbstractFunctionEvent(fer.Logger, fer.getPlatform(), functionEventConfig)
	if err != nil {
		return nil, err
	}

	// just deploy. the status is async through polling
	err = fer.getPlatform().CreateFunctionEvent(&platform.CreateFunctionEventOptions{
		FunctionEventConfig: *newFunctionEvent.GetConfig(),
	})
	if err != nil {
		return nil, err
	}

	return newFunctionEvent, nil
}

func (fer *functionEventResource) getFunctionEvents(function platform.Function, namespace string) []platform.FunctionEvent {
	getFunctionEventOptions := platform.GetFunctionEventsOptions{
		Meta: platform.FunctionEventMeta{
			Name:      "",
			Namespace: namespace,
			Labels: map[string]string{
				"nuclio.io/function-name": function.GetConfig().Meta.Name,
			},
		},
	}

	functionEvents, err := fer.getPlatform().GetFunctionEvents(&getFunctionEventOptions)
	if err == nil {
		return functionEvents
	}

	return []platform.FunctionEvent{}
}

func (fer *functionEventResource) deleteFunctionEvent(request *http.Request) (*restful.CustomRouteFuncResponse, error) {

	// get function event config and status from body
	functionEventInfo, err := fer.getFunctionEventInfoFromRequest(request, true)
	if err != nil {
		fer.Logger.WarnWith("Failed to get function event config and status from body", "err", err)

		return &restful.CustomRouteFuncResponse{
			Single:     true,
			StatusCode: http.StatusBadRequest,
		}, err
	}

	deleteFunctionEventOptions := platform.DeleteFunctionEventOptions{}
	deleteFunctionEventOptions.Meta = *functionEventInfo.Meta

	err = fer.getPlatform().DeleteFunctionEvent(&deleteFunctionEventOptions)
	if err != nil {
		return &restful.CustomRouteFuncResponse{
			Single:     true,
			StatusCode: http.StatusInternalServerError,
		}, err
	}

	return &restful.CustomRouteFuncResponse{
		ResourceType: "functionEvent",
		Single:       true,
		StatusCode:   http.StatusNoContent,
	}, err
}

func (fer *functionEventResource) updateFunctionEvent(request *http.Request) (*restful.CustomRouteFuncResponse, error) {

	statusCode := http.StatusNoContent

	// get function event config and status from body
	functionEventInfo, err := fer.getFunctionEventInfoFromRequest(request, true)
	if err != nil {
		fer.Logger.WarnWith("Failed to get function event config and status from body", "err", err)

		return &restful.CustomRouteFuncResponse{
			Single:     true,
			StatusCode: http.StatusBadRequest,
		}, err
	}

	functionEventConfig := platform.FunctionEventConfig{
		Meta: *functionEventInfo.Meta,
		Spec: *functionEventInfo.Spec,
	}

	if err = fer.getPlatform().UpdateFunctionEvent(&platform.UpdateFunctionEventOptions{
		FunctionEventConfig: functionEventConfig,
	}); err != nil {
		fer.Logger.WarnWith("Failed to update function event", "err", err)
		statusCode = common.ResolveErrorStatusCodeOrDefault(err, http.StatusInternalServerError)
	}

	// return the stuff
	return &restful.CustomRouteFuncResponse{
		ResourceType: "functionEvent",
		Single:       true,
		StatusCode:   statusCode,
	}, err
}

func (fer *functionEventResource) functionEventToAttributes(functionEvent platform.FunctionEvent) restful.Attributes {
	attributes := restful.Attributes{
		"metadata": functionEvent.GetConfig().Meta,
		"spec":     functionEvent.GetConfig().Spec,
	}

	return attributes
}

func (fer *functionEventResource) getNamespaceFromRequest(request *http.Request) string {
	return fer.getNamespaceOrDefault(request.Header.Get("x-nuclio-function-event-namespace"))
}

func (fer *functionEventResource) getFunctionNameFromRequest(request *http.Request) string {
	return request.Header.Get("x-nuclio-function-name")
}

func (fer *functionEventResource) getFunctionEventInfoFromRequest(request *http.Request, nameRequired bool) (*functionEventInfo, error) {

	// read body
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		return nil, nuclio.WrapErrInternalServerError(errors.Wrap(err, "Failed to read body"))
	}

	functionEventInfoInstance := functionEventInfo{}
	err = json.Unmarshal(body, &functionEventInfoInstance)
	if err != nil {
		return nil, nuclio.WrapErrBadRequest(errors.Wrap(err, "Failed to parse JSON body"))
	}

	// override namespace if applicable
	if functionEventInfoInstance.Meta != nil {
		functionEventInfoInstance.Meta.Namespace = fer.getNamespaceOrDefault(functionEventInfoInstance.Meta.Namespace)
	}

	// meta must exist
	if functionEventInfoInstance.Meta == nil ||
		(nameRequired && functionEventInfoInstance.Meta.Name == "") ||
		functionEventInfoInstance.Meta.Namespace == "" {
		err := errors.New("Function event name must be provided in metadata")

		return nil, nuclio.WrapErrBadRequest(err)
	}

	return &functionEventInfoInstance, nil
}

func (fer *functionEventResource) getFunctionEventFunction(functionEvent platform.FunctionEvent) (platform.Function,
	error) {
	functionName := functionEvent.GetConfig().Meta.Labels["nuclio.io/function-name"]
	getFunctionsOptions := &platform.GetFunctionsOptions{
		Namespace: functionEvent.GetConfig().Meta.Namespace,
		Name:      functionName,
	}

	functions, err := fer.getPlatform().GetFunctions(getFunctionsOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get functions")
	}

	if len(functions) == 0 {
		return nil, nuclio.NewErrNotFound("Function not found")
	}
	return functions[0], nil
}

func (fer *functionEventResource) queryOPAFunctionEventPermissions(request *http.Request,
	projectName,
	functionName,
	functionEventName string,
	action opa.Action,
	raiseForbidden bool) (bool, error) {
	if projectName == "" {
		projectName = "*"
	}
	if functionName == "" {
		functionName = "*"
	}
	if functionEventName == "" {
		functionEventName = "*"
	}
	return fer.queryOPAPermissions(request,
		opa.GenerateFunctionEventResourceString(projectName, functionName, functionEventName),
		action,
		raiseForbidden)
}

// register the resource
var functionEventResourceInstance = &functionEventResource{
	resource: newResource("api/function_events", []restful.ResourceMethod{
		restful.ResourceMethodGetList,
		restful.ResourceMethodGetDetail,
		restful.ResourceMethodCreate,
	}),
}

func init() {
	functionEventResourceInstance.Resource = functionEventResourceInstance
	functionEventResourceInstance.Register(dashboard.DashboardResourceRegistrySingleton)
}
