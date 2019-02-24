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
	"runtime/debug"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/nuclio/nuclio-sdk-go"
	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/restful"
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

// Create and deploy a function
func (fr *functionResource) Create(request *http.Request) (id string, attributes restful.Attributes, responseErr error) {
	functionInfo, responseErr := fr.getFunctionInfoFromRequest(request)
	if responseErr != nil {
		return
	}

	// validate there are no 2 functions with the same name
	getFunctionsOptions := &platform.GetFunctionsOptions{
		Name:      functionInfo.Meta.Name,
		Namespace: fr.getNamespaceFromRequest(request),
	}

	projectNameFilter, ok := functionInfo.Meta.Labels["nuclio.io/project-name"]
	if !ok || projectNameFilter == "" {
		responseErr = nuclio.WrapErrBadRequest(errors.New("No project name was given inside meta labels"))
		return
	}

	getFunctionsOptions.Labels = fmt.Sprintf("nuclio.io/project-name=%s", projectNameFilter)

	// TODO: Add a lock to prevent race conditions here (prevent 2 functions created with the same name)
	functions, err := fr.getPlatform().GetFunctions(getFunctionsOptions)
	if err != nil {
		responseErr = nuclio.WrapErrInternalServerError(errors.Wrap(err, "Failed to get functions"))
		return
	}

	if len(functions) > 0 {
		responseErr = nuclio.NewErrConflict("Cannot create two functions with the same name")
		return
	}

	// validation finished successfully - store and deploy the given function
	if responseErr = fr.storeAndDeployFunction(functionInfo, request); responseErr != nil {
		return
	}

	responseErr = nuclio.ErrAccepted
	return
}

// Update and deploy a function
func (fr *functionResource) Update(request *http.Request, id string) (attributes restful.Attributes, responseErr error) {
	functionInfo, responseErr := fr.getFunctionInfoFromRequest(request)
	if responseErr != nil {
		return
	}

	if responseErr = fr.storeAndDeployFunction(functionInfo, request); responseErr != nil {
		return
	}

	return nil, nuclio.ErrAccepted
}

func (fr *functionResource) storeAndDeployFunction(functionInfo *functionInfo, request *http.Request) error {

	creationStateUpdatedTimeout := 15 * time.Second

	doneChan := make(chan bool, 1)
	creationStateUpdatedChan := make(chan bool, 1)
	errDeployingChan := make(chan error, 1)

	// asynchronously, do the deploy so that the user doesn't wait
	go func() {
		defer func() {
			if err := recover(); err != nil {
				callStack := debug.Stack()

				fr.Logger.ErrorWith("Panic caught while creating function",
					"err",
					err,
					"stack",
					string(callStack))
			}
		}()

		dashboardServer := fr.GetServer().(*dashboard.Server)

		// if registry / run-registry aren't set - use dashboard settings
		if functionInfo.Spec.Build.Registry == "" {
			functionInfo.Spec.Build.Registry = dashboardServer.GetRegistryURL()
		}

		if functionInfo.Spec.RunRegistry == "" {
			functionInfo.Spec.RunRegistry = dashboardServer.GetRunRegistryURL()
		}

		functionInfo.Spec.Build.NoBaseImagesPull = dashboardServer.NoPullBaseImages
		functionInfo.Spec.Build.Offline = dashboardServer.Offline

		// just deploy. the status is async through polling
		_, err := fr.getPlatform().CreateFunction(&platform.CreateFunctionOptions{
			Logger: fr.Logger,
			FunctionConfig: functionconfig.Config{
				Meta: *functionInfo.Meta,
				Spec: *functionInfo.Spec,
			},
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
		return errDeploying
	case <-time.After(creationStateUpdatedTimeout):
		return nuclio.NewErrInternalServerError("Timed out waiting for creation state to be set")
	}

	// mostly for testing, but can also be for clients that want to wait for some reason
	if request.Header.Get("x-nuclio-wait-function-action") == "true" {
		<-doneChan
	}

	return nil
}

// returns a list of custom routes for the resource
func (fr *functionResource) GetCustomRoutes() ([]restful.CustomRoute, error) {

	// since delete and update by default assume /resource/{id} and we want to get the id/namespace from the body
	// we need to register custom routes
	return []restful.CustomRoute{
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

func (fr *functionResource) functionToAttributes(function platform.Function) restful.Attributes {
	functionSpec := function.GetConfig().Spec

	// artifacts are created unique to the cluster not needed to be returned to any client of nuclio REST API
	functionSpec.RunRegistry = ""
	functionSpec.Build.Registry = ""
	functionSpec.Build.Image = ""
	if functionSpec.Build.FunctionSourceCode != "" {
		functionSpec.Image = ""
	}

	attributes := restful.Attributes{
		"metadata": function.GetConfig().Meta,
		"spec":     functionSpec,
	}

	status := function.GetStatus()
	if status != nil {
		attributes["status"] = status
	}

	return attributes
}

func (fr *functionResource) getNamespaceFromRequest(request *http.Request) string {

	// get the namespace provided by the user or the default namespace
	return fr.getNamespaceOrDefault(request.Header.Get("x-nuclio-function-namespace"))
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

	// override namespace if applicable
	if functionInfoInstance.Meta != nil {
		functionInfoInstance.Meta.Namespace = fr.getNamespaceOrDefault(functionInfoInstance.Meta.Namespace)
	}

	// meta must exist
	if functionInfoInstance.Meta == nil ||
		functionInfoInstance.Meta.Name == "" ||
		functionInfoInstance.Meta.Namespace == "" {
		err := errors.New("Function name must be provided in metadata")

		return nil, nuclio.WrapErrBadRequest(err)
	}

	// validate function name is according to k8s convention
	errorMessages := validation.IsQualifiedName(functionInfoInstance.Meta.Name)
	if len(errorMessages) != 0 {
		joinedErrorMessage := strings.Join(errorMessages, ", ")
		return nil, nuclio.NewErrBadRequest("Function name doesn't conform to k8s naming convention. Errors: " + joinedErrorMessage)
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
		restful.ResourceMethodUpdate,
	}),
}

func init() {
	functionResourceInstance.Resource = functionResourceInstance
	functionResourceInstance.Register(dashboard.DashboardResourceRegistrySingleton)
}
