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
	"strconv"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
	"k8s.io/apimachinery/pkg/util/validation"
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

	exportFunction := fr.GetURLParamBoolOrDefault(request, restful.ParamExport, false)

	// create a map of attributes keyed by the function id (name)
	for _, function := range functions {
		if exportFunction {
			response[function.GetConfig().Meta.Name] = fr.export(function)
		} else {
			response[function.GetConfig().Meta.Name] = fr.functionToAttributes(function)
		}
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

	functions, err := fr.getPlatform().GetFunctions(&platform.GetFunctionsOptions{
		Namespace: fr.getNamespaceFromRequest(request),
		Name:      id,
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get functions")
	}

	if len(functions) == 0 {
		return nil, nuclio.NewErrNotFound("Function not found")
	}
	function := functions[0]

	exportFunction := fr.GetURLParamBoolOrDefault(request, restful.ParamExport, false)
	if exportFunction {
		return fr.export(function), nil
	}

	return fr.functionToAttributes(function), nil
}

// Create and deploy a function
func (fr *functionResource) Create(request *http.Request) (id string, attributes restful.Attributes, responseErr error) {
	functionInfo, responseErr := fr.getFunctionInfoFromRequest(request)
	if responseErr != nil {
		return
	}

	// TODO: Add a lock to prevent race conditions here (prevent 2 functions created with the same name)
	// validate there are no 2 functions with the same name
	functions, err := fr.getPlatform().GetFunctions(&platform.GetFunctionsOptions{
		Name:      functionInfo.Meta.Name,
		Namespace: fr.getNamespaceFromRequest(request),
	})
	if err != nil {
		responseErr = nuclio.WrapErrInternalServerError(errors.Wrap(err, "Failed to get functions"))
		return
	}
	if len(functions) > 0 {
		responseErr = nuclio.NewErrConflict("Cannot create two functions with the same name")
		return
	}

	// get the authentication configuration for the request
	authConfig, responseErr := fr.getRequestAuthConfig(request)
	if responseErr != nil {
		return
	}

	waitForFunction := request.Header.Get("x-nuclio-wait-function-action") == "true"

	// validation finished successfully - store and deploy the given function
	if responseErr = fr.storeAndDeployFunction(functionInfo, authConfig, waitForFunction); responseErr != nil {
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

	// TODO: Add a lock to prevent race conditions here
	// validate the function exists
	functions, err := fr.getPlatform().GetFunctions(&platform.GetFunctionsOptions{
		Name:      functionInfo.Meta.Name,
		Namespace: fr.getNamespaceFromRequest(request),
	})
	if err != nil {
		responseErr = nuclio.WrapErrInternalServerError(errors.Wrap(err, "Failed to get functions"))
		return
	}
	if len(functions) == 0 {
		responseErr = nuclio.NewErrNotFound("Cannot update non existing function")
		return
	}

	if err = fr.validateUpdateInfo(functionInfo, functions[0]); err != nil {
		responseErr = nuclio.WrapErrBadRequest(errors.Wrap(err, "Requested update fields are invalid"))
		return
	}

	// get the authentication configuration for the request
	authConfig, responseErr := fr.getRequestAuthConfig(request)
	if responseErr != nil {
		return
	}

	waitForFunction := request.Header.Get("x-nuclio-wait-function-action") == "true"

	if responseErr = fr.storeAndDeployFunction(functionInfo, authConfig, waitForFunction); responseErr != nil {
		return
	}

	return nil, nuclio.ErrAccepted
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

func (fr *functionResource) export(function platform.Function) restful.Attributes {
	functionSpec := fr.cleanFunctionSpec(function.GetConfig().Spec)
	functionMeta := function.GetConfig().Meta

	fr.Logger.DebugWith("Exporting function", "functionName", functionMeta.Name)

	fr.prepareFunctionForExport(&functionMeta, &functionSpec)

	attributes := restful.Attributes{
		"metadata": functionMeta,
		"spec":     functionSpec,
	}

	return attributes
}

func (fr *functionResource) prepareFunctionForExport(functionMeta *functionconfig.Meta, functionSpec *functionconfig.Spec) {

	fr.Logger.DebugWith("Preparing function for export", "functionName", functionMeta.Name)

	if functionMeta.Annotations == nil {
		functionMeta.Annotations = map[string]string{}
	}

	// add annotations for not deploying or building on import
	functionMeta.Annotations[functionconfig.FunctionAnnotationSkipBuild] = strconv.FormatBool(true)
	functionMeta.Annotations[functionconfig.FunctionAnnotationSkipDeploy] = strconv.FormatBool(true)

	// scrub namespace from function meta
	functionMeta.Namespace = ""

	// remove secrets and passwords from triggers
	newTriggers := functionSpec.Triggers
	for triggerName, trigger := range newTriggers {
		trigger.Password = ""
		trigger.Secret = ""
		newTriggers[triggerName] = trigger
	}
	functionSpec.Triggers = newTriggers
}

func (fr *functionResource) storeAndDeployFunction(functionInfo *functionInfo, authConfig *platform.AuthConfig, waitForFunction bool) error {

	creationStateUpdatedTimeout := 45 * time.Second

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
			CreationStateUpdated:       creationStateUpdatedChan,
			AuthConfig:                 authConfig,
			DependantImagesRegistryURL: fr.GetServer().(*dashboard.Server).GetDependantImagesRegistryURL(),
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
		return errors.RootCause(errDeploying)
	case <-time.After(creationStateUpdatedTimeout):
		return nuclio.NewErrInternalServerError("Timed out waiting for creation state to be set")
	}

	// mostly for testing, but can also be for clients that want to wait for some reason
	if waitForFunction {
		<-doneChan
	}

	return nil
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

	// get the authentication configuration for the request
	authConfig, err := fr.getRequestAuthConfig(request)
	if err != nil {

		// get error
		if errWithStatus, ok := err.(*nuclio.ErrorWithStatusCode); ok {
			return &restful.CustomRouteFuncResponse{
				Single:     true,
				StatusCode: errWithStatus.StatusCode(),
			}, err
		}

		return &restful.CustomRouteFuncResponse{
			Single:     true,
			StatusCode: http.StatusInternalServerError,
		}, err
	}

	deleteFunctionOptions := platform.DeleteFunctionOptions{
		AuthConfig: authConfig,
	}

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

func (fr *functionResource) cleanFunctionSpec(functionSpec functionconfig.Spec) functionconfig.Spec {

	// artifacts are created unique to the cluster not needed to be returned to any client of nuclio REST API
	functionSpec.RunRegistry = ""
	functionSpec.Build.Registry = ""
	if functionSpec.Build.FunctionSourceCode != "" {
		functionSpec.Image = ""
	}

	return functionSpec
}

func (fr *functionResource) functionToAttributes(function platform.Function) restful.Attributes {
	functionSpec := fr.cleanFunctionSpec(function.GetConfig().Spec)

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

	return fr.processFunctionInfo(&functionInfoInstance, request.Header.Get("x-nuclio-project-name"))
}

func (fr *functionResource) validateUpdateInfo(functionInfo *functionInfo, function platform.Function) error {

	// in the imported state, after the function has the skip-build and skip-deploy annotations removed,
	// if the user tries to disable the function, it will in turn build and deploy the function and then disable it.
	// so here we don't allow users to disable an imported function.
	if functionInfo.Spec.Disable && function.GetStatus().State == functionconfig.FunctionStateImported {
		return errors.New("Failed to disable function: non-deployed functions cannot be disabled")
	}

	return nil
}

func (fr *functionResource) processFunctionInfo(functionInfoInstance *functionInfo, projectName string) (*functionInfo, error) {
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
	if projectName != "" {
		if functionInfoInstance.Meta.Labels == nil {
			functionInfoInstance.Meta.Labels = map[string]string{}
		}

		functionInfoInstance.Meta.Labels["nuclio.io/project-name"] = projectName
	}

	return functionInfoInstance, nil
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
