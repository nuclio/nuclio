/*
Copyright 2023 The Nuclio Authors.

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
	"io"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/opa"
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

type PatchOptions struct {
	DesiredState *functionconfig.FunctionState `json:"desiredState,omitempty"`
}

func (fr *functionResource) ExtendMiddlewares() error {
	fr.resource.addAuthMiddleware(nil)
	return nil
}

// GetAll returns all functions
func (fr *functionResource) GetAll(request *http.Request) (map[string]restful.Attributes, error) {
	ctx := request.Context()
	response := map[string]restful.Attributes{}

	// get namespace
	namespace := fr.getNamespaceFromRequest(request)
	if namespace == "" {
		return nil, nuclio.NewErrBadRequest("Namespace must exist")
	}

	functionName := request.Header.Get(headers.FunctionName)
	getFunctionOptions := fr.resolveGetFunctionOptionsFromRequest(request, functionName, false)
	functions, err := fr.getPlatform().GetFunctions(ctx, getFunctionOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get functions")
	}

	exportFunction := fr.GetURLParamBoolOrDefault(request, restful.ParamExport, false)
	exportOptions := fr.getExportOptionsFromRequest(request)
	// create a map of attributes keyed by the function id (name)
	for _, function := range functions {
		if exportFunction {
			response[function.GetConfig().Meta.Name] = fr.export(ctx, function, exportOptions)
		} else {
			response[function.GetConfig().Meta.Name] = fr.functionToAttributes(function, exportOptions.CleanupSpec)
		}
	}

	return response, nil
}

// GetByID returns a specific function by id
func (fr *functionResource) GetByID(request *http.Request, id string) (restful.Attributes, error) {
	ctx := request.Context()

	// get and validate namespace
	namespace := fr.getNamespaceFromRequest(request)
	if namespace == "" {
		return nil, nuclio.NewErrBadRequest("Namespace must exist")
	}

	// get function
	function, err := fr.getFunction(request, id)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get get function")
	}
	exportOptions := fr.getExportOptionsFromRequest(request)
	if fr.GetURLParamBoolOrDefault(request, restful.ParamExport, false) {
		return fr.export(ctx, function, exportOptions), nil
	}

	return fr.functionToAttributes(function, exportOptions.CleanupSpec), nil
}

// Create and deploy a function
func (fr *functionResource) Create(request *http.Request) (id string, attributes restful.Attributes, responseErr error) {

	functionInfo, responseErr := fr.getFunctionInfoFromRequest(request)
	if responseErr != nil {
		return
	}

	// TODO: Add a lock to prevent race conditions here (prevent 2 functions created with the same name)
	// validate there are no 2 functions with the same name
	functions, err := fr.getPlatform().GetFunctions(request.Context(), &platform.GetFunctionsOptions{
		Name:        functionInfo.Meta.Name,
		Namespace:   fr.resolveNamespace(request, functionInfo),
		AuthSession: fr.getCtxSession(request.Context()),
		PermissionOptions: opa.PermissionOptions{
			MemberIds:           opa.GetUserAndGroupIdsFromAuthSession(fr.getCtxSession(request.Context())),
			RaiseForbidden:      true,
			OverrideHeaderValue: request.Header.Get(opa.OverrideHeader),
		},
	})
	if err != nil {
		responseErr = errors.Wrap(err, "Failed to get functions")
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

	waitForFunction := fr.headerValueIsTrue(request, headers.WaitFunctionAction)

	// validation finished successfully - store and deploy the given function
	if responseErr = fr.storeAndDeployFunction(request, functionInfo, authConfig, waitForFunction); responseErr != nil {
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

	// get the authentication configuration for the request
	authConfig, responseErr := fr.getRequestAuthConfig(request)
	if responseErr != nil {
		return
	}

	waitForFunction := fr.headerValueIsTrue(request, headers.WaitFunctionAction)

	if responseErr = fr.storeAndDeployFunction(request, functionInfo, authConfig, waitForFunction); responseErr != nil {
		return
	}

	return nil, nuclio.ErrAccepted
}

// Patch applies partial modifications to a function
func (fr *functionResource) Patch(request *http.Request, id string) error {

	// if external registry required, but user has an internal one, then return 412 code (PreconditionFailed)
	if fr.headerValueIsTrue(request, headers.VerifyExternalRegistry) && fr.getPlatform().GetRegistryKind() == "onCluster" {
		return nuclio.NewErrPreconditionFailed("Can not patch function because registry is internal")
	}
	// get the desired state of the function from the request body
	patchOptionsInstance, err := fr.getPatchFunctionOptionsFromRequest(request)
	if err != nil {
		return errors.Wrap(err, "Failed to get patch options")
	}

	// get the authentication configuration for the request
	authConfig, err := fr.getRequestAuthConfig(request)
	if err != nil {
		return errors.Wrap(err, "Failed to get auth config")
	}

	if patchOptionsInstance.DesiredState != nil {
		return fr.patchFunctionDesiredState(request, id, patchOptionsInstance, authConfig)
	}

	return nil
}

// GetCustomRoutes returns a list of custom routes for the resource
func (fr *functionResource) GetCustomRoutes() ([]restful.CustomRoute, error) {

	// since delete and update by default assume /resource/{id} and we want to get the id/namespace from the body
	// we need to register custom routes
	return []restful.CustomRoute{
		{
			Pattern:   "/",
			Method:    http.MethodDelete,
			RouteFunc: fr.deleteFunction,
		},
		{
			Pattern:   "/{id}/replicas",
			Method:    http.MethodGet,
			RouteFunc: fr.getFunctionReplicas,
		},
		{
			Pattern:         "/{id}/logs/{replicaName}",
			Method:          http.MethodGet,
			StreamRouteFunc: fr.getFunctionLogs,
			Stream:          true,
		},
	}, nil
}

func (fr *functionResource) export(ctx context.Context, function platform.Function, exportOptions *common.ExportFunctionOptions) restful.Attributes {

	functionConfig := function.GetConfig()
	fr.Logger.DebugWithCtx(ctx, "Preparing function for export", "functionName", functionConfig.Meta.Name)
	exportOptions.PrevState = string(function.GetStatus().State)
	functionConfig.PrepareFunctionForExport(exportOptions)

	fr.Logger.DebugWithCtx(ctx, "Exporting function", "functionName", functionConfig.Meta.Name)

	attributes := restful.Attributes{
		"metadata": functionConfig.Meta,
		"spec":     functionConfig.Spec,
	}

	return attributes
}

func (fr *functionResource) storeAndDeployFunction(request *http.Request,
	functionInfo *functionInfo,
	authConfig *platform.AuthConfig,
	waitForFunction bool) error {
	creationStateUpdatedTimeout := fr.getCreationStateUpdatedTimeout(request)

	doneChan := make(chan bool, 1)
	creationStateUpdatedChan := make(chan bool, 1)
	errDeployingChan := make(chan error, 1)
	autofix := fr.headerValueIsTrue(request, headers.AutofixFunctionConfiguration)

	// deploy asynchronously, so that the user doesn't wait
	go func() {

		// create a cancel function independent of the parent context
		ctx, cancelCtx := context.WithCancel(context.WithoutCancel(request.Context()))
		defer cancelCtx()

		// inject auth session to new context
		ctx = context.WithValue(ctx, auth.AuthSessionContextKey, fr.getCtxSession(ctx))

		defer func() {
			if err := recover(); err != nil {
				callStack := debug.Stack()
				fr.Logger.ErrorWithCtx(ctx, "Panic caught while creating function",
					"err", err,
					"stack", string(callStack))
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
		if _, err := fr.getPlatform().CreateFunction(ctx,
			&platform.CreateFunctionOptions{
				Logger: fr.Logger,
				FunctionConfig: functionconfig.Config{
					Meta: *functionInfo.Meta,
					Spec: *functionInfo.Spec,
				},
				CreationStateUpdated:       creationStateUpdatedChan,
				AuthConfig:                 authConfig,
				AutofixConfiguration:       autofix,
				DependantImagesRegistryURL: fr.GetServer().(*dashboard.Server).GetDependantImagesRegistryURL(),
				AuthSession:                ctx.Value(auth.AuthSessionContextKey).(auth.Session),
				PermissionOptions: opa.PermissionOptions{
					MemberIds:           opa.GetUserAndGroupIdsFromAuthSession(fr.getCtxSession(ctx)),
					OverrideHeaderValue: request.Header.Get(opa.OverrideHeader),
				},
			}); err != nil {
			fr.Logger.WarnWithCtx(ctx,
				"Failed to deploy function",
				"err", errors.GetErrorStackString(err, 10))
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
		return errors.Wrapf(errDeploying, "Failed to deploy function %s", functionInfo.Meta.Name)
	case <-time.After(creationStateUpdatedTimeout):
		return nuclio.NewErrInternalServerError("Timed out waiting for creation state to be set")
	}

	// mostly for testing, but can also be for clients that want to wait for some reason
	if waitForFunction {
		<-doneChan
	}

	return nil
}

func (fr *functionResource) getFunctionLogs(request *http.Request) (*restful.CustomRouteFuncStreamResponse, error) {

	// ensure namespace
	namespace := fr.getNamespaceFromRequest(request)
	if namespace == "" {
		return nil, nuclio.NewErrBadRequest("Namespace must exist")
	}

	// ensure function name
	functionName := fr.GetRouterURLParam(request, "id")
	if functionName == "" {
		return nil, errors.New("Function name must not be empty")
	}

	// ensure replica name
	functionReplicaName := fr.GetRouterURLParam(request, "replicaName")
	if functionReplicaName == "" {
		return nil, errors.New("Function instance must not be empty")
	}

	// populate get options
	getFunctionReplicaLogsStreamOptions, err := fr.populateGetFunctionReplicaLogsStreamOptions(request,
		functionReplicaName,
		namespace)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to populate get function replica logs stream options")
	}

	// get function
	function, err := fr.getFunction(request, functionName)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function")
	}

	replicaNames, err := fr.getPlatform().GetFunctionReplicaNames(request.Context(), function.GetConfig())
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function replica names")
	}

	// ensure replica belongs to function
	if !common.StringSliceContainsStringCaseInsensitive(replicaNames, functionReplicaName) {
		return nil, nuclio.NewErrBadRequest(fmt.Sprintf("%s replica does not belong to function %s",
			functionReplicaName,
			function.GetConfig().Meta.Name))
	}

	// get function instance logs stream
	stream, err := fr.getPlatform().GetFunctionReplicaLogsStream(request.Context(), getFunctionReplicaLogsStreamOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to stream function logs")
	}

	return &restful.CustomRouteFuncStreamResponse{
		ReadCloser: stream,
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Content-Type":  "text/plain",
			"Cache-Control": "no-cache, private",
		},
		ForceFlush:    true,
		FlushInternal: time.Second,
	}, nil
}

func (fr *functionResource) getFunctionReplicas(request *http.Request) (
	*restful.CustomRouteFuncResponse, error) {

	// ensure namespace
	namespace := fr.getNamespaceFromRequest(request)
	if namespace == "" {
		return nil, nuclio.NewErrBadRequest("Namespace must exist")
	}

	// ensure function name
	functionName := fr.GetRouterURLParam(request, "id")
	if functionName == "" {
		return nil, errors.New("Function name must not be empty")
	}

	function, err := fr.getFunction(request, functionName)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function")
	}

	replicaNames, err := fr.getPlatform().GetFunctionReplicaNames(request.Context(), function.GetConfig())
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function replicas")
	}
	return &restful.CustomRouteFuncResponse{
		Resources: map[string]restful.Attributes{
			"replicas": map[string]interface{}{
				"names": replicaNames,
			},
		},
		Single:     true,
		Headers:    map[string]string{"Content-Type": "application/json"},
		StatusCode: http.StatusOK,
	}, nil
}

func (fr *functionResource) deleteFunction(request *http.Request) (*restful.CustomRouteFuncResponse, error) {
	ctx := request.Context()

	// get function config and status from body
	functionInfo, err := fr.getFunctionInfoFromRequest(request)
	if err != nil {
		fr.Logger.WarnWithCtx(ctx, "Failed to get function config and status from body", "err", err)

		return &restful.CustomRouteFuncResponse{
			Single:     true,
			StatusCode: http.StatusBadRequest,
		}, err
	}

	// get the authentication configuration for the request
	authConfig, err := fr.getRequestAuthConfig(request)
	if err != nil {
		return &restful.CustomRouteFuncResponse{
			Single:     true,
			StatusCode: common.ResolveErrorStatusCodeOrDefault(err, http.StatusInternalServerError),
		}, err
	}

	deleteFunctionOptions := platform.DeleteFunctionOptions{
		AuthConfig:  authConfig,
		AuthSession: fr.getCtxSession(ctx),
		PermissionOptions: opa.PermissionOptions{
			MemberIds:           opa.GetUserAndGroupIdsFromAuthSession(fr.getCtxSession(ctx)),
			OverrideHeaderValue: request.Header.Get(opa.OverrideHeader),
		},
		IgnoreFunctionStateValidation: fr.headerValueIsTrue(request,
			headers.DeleteFunctionIgnoreStateValidation),
	}

	deleteFunctionOptions.FunctionConfig.Meta = *functionInfo.Meta

	if err := fr.getPlatform().DeleteFunction(ctx, &deleteFunctionOptions); err != nil {
		return &restful.CustomRouteFuncResponse{
			Single:     true,
			StatusCode: common.ResolveErrorStatusCodeOrDefault(err, http.StatusInternalServerError),
		}, err
	}

	return &restful.CustomRouteFuncResponse{
		ResourceType: "function",
		Single:       true,
		StatusCode:   http.StatusNoContent,
	}, nil
}

func (fr *functionResource) patchFunctionDesiredState(request *http.Request,
	id string,
	options *PatchOptions,
	authConfig *platform.AuthConfig) error {

	switch *options.DesiredState {
	case functionconfig.FunctionStateReady, functionconfig.FunctionStateScaledToZero:
		return fr.redeployFunction(request, id, authConfig, options)
	default:
		return nuclio.NewErrBadRequest(fmt.Sprintf("Unsupported desired state in patch request: %s",
			*options.DesiredState))
	}
}

func (fr *functionResource) redeployFunction(request *http.Request,
	id string,
	authConfig *platform.AuthConfig,
	options *PatchOptions) error {
	ctx := request.Context()

	// get function
	function, err := fr.getFunction(request, id)
	if err != nil {
		return errors.Wrap(err, "Failed to get get function")
	}

	if function.GetConfig().Spec.Image == "" {
		return nuclio.NewErrPreconditionFailed("No image field in function config spec, unable to redeploy")
	}

	if *options.DesiredState == functionconfig.FunctionStateScaledToZero &&
		*function.GetConfig().Spec.MinReplicas > 0 {
		return nuclio.NewErrPreconditionFailed("Cannot scale to zero a function with non-zero min replicas")
	}

	importedOnly := fr.headerValueIsTrue(request, headers.ImportedFunctionOnly)

	if importedOnly && function.GetStatus().State != functionconfig.FunctionStateImported {
		fr.Logger.DebugWithCtx(ctx, "Function is not imported, skipping redeploy", "functionName", id, "functionState", function.GetStatus().State)
		return nil
	}

	fr.Logger.DebugWith("Redeploying function",
		"functionName", id,
		"desiredState", *options.DesiredState)

	if err := fr.getPlatform().RedeployFunction(ctx, &platform.RedeployFunctionOptions{
		FunctionMeta:               &function.GetConfig().Meta,
		FunctionSpec:               &function.GetConfig().Spec,
		AuthConfig:                 authConfig,
		DependantImagesRegistryURL: fr.GetServer().(*dashboard.Server).GetDependantImagesRegistryURL(),
		AuthSession:                fr.getCtxSession(ctx),
		PermissionOptions: opa.PermissionOptions{
			MemberIds:           opa.GetUserAndGroupIdsFromAuthSession(fr.getCtxSession(ctx)),
			OverrideHeaderValue: request.Header.Get(opa.OverrideHeader),
		},
		DesiredState:                *options.DesiredState,
		CreationStateUpdatedTimeout: fr.getCreationStateUpdatedTimeout(request),
	}); err != nil {
		return errors.Wrap(err, "Failed to redeploy function")
	}

	return nuclio.ErrAccepted
}

func (fr *functionResource) functionToAttributes(function platform.Function, cleanupSpec bool) restful.Attributes {
	functionConfig := function.GetConfig()
	if cleanupSpec {
		functionConfig.CleanFunctionSpec()
	}

	attributes := restful.Attributes{
		"metadata": functionConfig.Meta,
		"spec":     functionConfig.Spec,
	}

	if status := function.GetStatus(); status != nil {
		attributes["status"] = status
	}

	return attributes
}

func (fr *functionResource) getNamespaceFromRequest(request *http.Request) string {

	// get the namespace provided by the user or the default namespace
	return fr.getNamespaceOrDefault(request.Header.Get(headers.FunctionNamespace))
}

func (fr *functionResource) getFunctionInfoFromRequest(request *http.Request) (*functionInfo, error) {

	// read body
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read body")
	}

	functionInfoInstance := functionInfo{}
	if err := json.Unmarshal(body, &functionInfoInstance); err != nil {
		return nil, nuclio.WrapErrBadRequest(errors.Wrap(err, "Failed to parse JSON body"))
	}
	return fr.processFunctionInfo(&functionInfoInstance, request.Header.Get(headers.ProjectName))
}

func (fr *functionResource) getPatchFunctionOptionsFromRequest(request *http.Request) (*PatchOptions, error) {

	// read body
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read body")
	}

	patchOptionsInstance := PatchOptions{}
	if err := json.Unmarshal(body, &patchOptionsInstance); err != nil {
		return nil, nuclio.WrapErrBadRequest(errors.Wrap(err, "Failed to parse JSON body"))
	}
	return &patchOptionsInstance, nil
}

func (fr *functionResource) resolveNamespace(request *http.Request, function *functionInfo) string {
	namespace := fr.getNamespaceFromRequest(request)
	if namespace != "" {
		return namespace
	}
	return function.Meta.Namespace
}

func (fr *functionResource) getFunction(request *http.Request, name string) (platform.Function, error) {
	ctx := request.Context()
	getFunctionOptions := fr.resolveGetFunctionOptionsFromRequest(request, name, true)
	functions, err := fr.getPlatform().GetFunctions(ctx, getFunctionOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get functions")
	}

	if len(functions) == 0 {
		return nil, nuclio.NewErrNotFound("Function not found")
	}
	return functions[0], nil
}

func (fr *functionResource) resolveGetFunctionOptionsFromRequest(request *http.Request,
	functionName string,
	raiseForbidden bool) *platform.GetFunctionsOptions {

	ctx := request.Context()

	getFunctionsOptions := &platform.GetFunctionsOptions{
		Namespace:             fr.getNamespaceFromRequest(request),
		Name:                  functionName,
		EnrichWithAPIGateways: fr.headerValueIsTrue(request, headers.FunctionEnrichApiGateways),
		AuthSession:           fr.getCtxSession(ctx),
		PermissionOptions: opa.PermissionOptions{
			MemberIds:           opa.GetUserAndGroupIdsFromAuthSession(fr.getCtxSession(ctx)),
			RaiseForbidden:      raiseForbidden,
			OverrideHeaderValue: request.Header.Get(opa.OverrideHeader),
		},
	}

	// if the user wants to filter by project, do that
	projectNameFilter := request.Header.Get(headers.ProjectName)
	if projectNameFilter != "" {
		getFunctionsOptions.Labels = fmt.Sprintf("%s=%s", common.NuclioResourceLabelKeyProjectName,
			projectNameFilter)
	}
	return getFunctionsOptions
}

func (fr *functionResource) processFunctionInfo(functionInfoInstance *functionInfo, projectName string) (
	*functionInfo, error) {

	//
	// enrichment
	//
	if functionInfoInstance.Meta == nil {
		functionInfoInstance.Meta = &functionconfig.Meta{}
	}

	functionInfoInstance.Meta.Namespace = fr.getNamespaceOrDefault(functionInfoInstance.Meta.Namespace)

	// add project name label if given via header
	if projectName != "" {
		if functionInfoInstance.Meta.Labels == nil {
			functionInfoInstance.Meta.Labels = map[string]string{}
		}

		functionInfoInstance.Meta.Labels[common.NuclioResourceLabelKeyProjectName] = projectName
	}

	//
	// validate for missing / malformed fields
	//

	// name must exists
	if functionInfoInstance.Meta.Name == "" {
		return nil, nuclio.NewErrBadRequest("Function name must be provided in metadata")
	}

	// namespace must exists (sanity)
	// TODO: is this really possible considering the fact namespace was enriched beforehand?
	if functionInfoInstance.Meta.Namespace == "" {
		return nil, nuclio.NewErrBadRequest("Function namespace must be provided in metadata")
	}

	// validate function name is according to k8s convention
	errorMessages := validation.IsQualifiedName(functionInfoInstance.Meta.Name)
	if len(errorMessages) != 0 {
		joinedErrorMessage := strings.Join(errorMessages, ", ")
		return nil, nuclio.NewErrBadRequest("Function name doesn't conform to k8s naming convention. Errors: " +
			joinedErrorMessage)
	}

	return functionInfoInstance, nil
}

func (fr *functionResource) populateGetFunctionReplicaLogsStreamOptions(request *http.Request,
	replicaName string,
	namespace string) (*platform.GetFunctionReplicaLogsStreamOptions, error) {

	getFunctionReplicaLogsStreamOptions := &platform.GetFunctionReplicaLogsStreamOptions{
		Name:      replicaName,
		Namespace: namespace,
		Follow:    fr.GetURLParamBoolOrDefault(request, "follow", true),
	}

	// populate since seconds
	sinceStr := fr.GetURLParamStringOrDefault(request, "since", "")
	if sinceStr != "" {
		since, err := time.ParseDuration(sinceStr)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to parse sinceSeconds")
		}
		sinceSeconds := int64(since.Seconds())
		getFunctionReplicaLogsStreamOptions.SinceSeconds = &sinceSeconds
	} else {
		getFunctionReplicaLogsStreamOptions.SinceSeconds = nil
	}

	// populate since seconds
	tailLines := fr.GetURLParamInt64OrDefault(request, "tailLines", -1)
	if tailLines != -1 {
		getFunctionReplicaLogsStreamOptions.TailLines = &tailLines
	} else {
		getFunctionReplicaLogsStreamOptions.TailLines = nil
	}

	return getFunctionReplicaLogsStreamOptions, nil

}

func (fr *functionResource) getCreationStateUpdatedTimeout(request *http.Request) time.Duration {
	timeoutDuration := 1 * time.Minute

	// get the timeout from the request header
	timeout := request.Header.Get(headers.CreationStateUpdatedTimeout)
	if timeout != "" {

		// parse the timeout
		if parsedTimeoutDuration, err := time.ParseDuration(timeout); err != nil {
			fr.Logger.WarnWith("Failed to parse timeout from header, using default",
				"timeout", timeout,
				"err", err)
		} else {
			timeoutDuration = parsedTimeoutDuration
		}
	}
	return timeoutDuration
}

// register the resource
var functionResourceInstance = &functionResource{
	resource: newResource("api/functions", []restful.ResourceMethod{
		restful.ResourceMethodGetList,
		restful.ResourceMethodGetDetail,
		restful.ResourceMethodCreate,
		restful.ResourceMethodUpdate,
		restful.ResourceMethodPatch,
	}),
}

func init() {
	functionResourceInstance.Resource = functionResourceInstance
	functionResourceInstance.Register(dashboard.DashboardResourceRegistrySingleton)
}
