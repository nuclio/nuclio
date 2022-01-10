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

	"github.com/nuclio/nuclio/pkg/common"
	nucliocontext "github.com/nuclio/nuclio/pkg/context"
	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/dashboard/auth"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
)

type apiGatewayResource struct {
	*resource
}

type apiGatewayInfo struct {
	Meta   *platform.APIGatewayMeta   `json:"metadata,omitempty"`
	Spec   *platform.APIGatewaySpec   `json:"spec,omitempty"`
	Status *platform.APIGatewayStatus `json:"status,omitempty"`
}

func (agr *apiGatewayResource) ExtendMiddlewares() error {
	agr.resource.addAuthMiddleware()
	return nil
}

// GetAll returns all api gateways
func (agr *apiGatewayResource) GetAll(request *http.Request) (map[string]restful.Attributes, error) {
	ctx := request.Context()

	// get namespace
	namespace := agr.getNamespaceFromRequest(request)
	if namespace == "" {
		return nil, nuclio.NewErrBadRequest("Namespace must exist")
	}

	exportFunction := agr.GetURLParamBoolOrDefault(request, restful.ParamExport, false)
	projectName := request.Header.Get("x-nuclio-project-name")

	// filter by project name (when it's specified)
	getAPIGatewaysOptions := platform.GetAPIGatewaysOptions{
		AuthSession: agr.getCtxSession(request),
		Namespace:   namespace,
	}
	if projectName != "" {
		getAPIGatewaysOptions.Labels = fmt.Sprintf("%s=%s",
			common.NuclioResourceLabelKeyProjectName,
			projectName)
	}

	return agr.GetAllByNamespace(ctx, &getAPIGatewaysOptions, exportFunction)
}

// GetAllByNamespace returns all api-gateways by namespace
func (agr *apiGatewayResource) GetAllByNamespace(ctx context.Context,
	getAPIGatewayOptions *platform.GetAPIGatewaysOptions,
	exportFunction bool) (map[string]restful.Attributes, error) {
	response := map[string]restful.Attributes{}

	apiGateways, err := agr.getPlatform().GetAPIGateways(ctx, getAPIGatewayOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get api gateways")
	}

	for _, apiGateway := range apiGateways {
		if exportFunction {
			response[apiGateway.GetConfig().Meta.Name] = agr.export(ctx, apiGateway)
		} else {

			// create a map of attributes keyed by the api-gateway id (name)
			response[apiGateway.GetConfig().Meta.Name] = agr.apiGatewayToAttributes(apiGateway)
		}
	}

	return response, nil
}

// GetByID returns a specific api gateway by id
func (agr *apiGatewayResource) GetByID(request *http.Request, id string) (restful.Attributes, error) {
	ctx := request.Context()

	// get namespace
	namespace := agr.getNamespaceFromRequest(request)
	if namespace == "" {
		return nil, nuclio.NewErrBadRequest("Namespace must exist")
	}

	apiGateways, err := agr.getPlatform().GetAPIGateways(ctx, &platform.GetAPIGatewaysOptions{
		Name:        id,
		Namespace:   namespace,
		AuthSession: agr.getCtxSession(request),
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get api gateways")
	}

	if len(apiGateways) == 0 {
		return nil, nuclio.NewErrNotFound("Api-Gateway not found")
	}
	apiGateway := apiGateways[0]

	exportFunction := agr.GetURLParamBoolOrDefault(request, restful.ParamExport, false)
	if exportFunction {
		return agr.export(ctx, apiGateway), nil
	}

	return agr.apiGatewayToAttributes(apiGateway), nil
}

// Create an api gateway
// returns (id, attributes, error)
func (agr *apiGatewayResource) Create(request *http.Request) (string, restful.Attributes, error) {
	ctx := request.Context()
	apiGatewayInfo, err := agr.getAPIGatewayInfoFromRequest(request)
	if err != nil {
		agr.Logger.WarnWithCtx(ctx, "Failed to get api gateway config and status from body", "err", err)
		return "", nil, err
	}

	return agr.createAPIGateway(request, apiGatewayInfo)
}

func (agr *apiGatewayResource) updateAPIGateway(request *http.Request) (*restful.CustomRouteFuncResponse, error) {
	ctx, cancelCtx := context.WithCancel(nucliocontext.NewDetached(request.Context()))
	defer cancelCtx()

	// inject auth session to new context
	ctx = context.WithValue(ctx, auth.AuthSessionContextKey, agr.getCtxSession(request))

	// get api gateway config and status from body
	apiGatewayInfo, err := agr.getAPIGatewayInfoFromRequest(request)
	if err != nil {
		agr.Logger.WarnWithCtx(ctx, "Failed to get api gateway config and status from body", "err", err)

		return &restful.CustomRouteFuncResponse{
			Single:     true,
			StatusCode: http.StatusBadRequest,
		}, err
	}

	apiGatewayConfig := &platform.APIGatewayConfig{
		Meta:   *apiGatewayInfo.Meta,
		Spec:   *apiGatewayInfo.Spec,
		Status: *apiGatewayInfo.Status,
	}

	if err = agr.getPlatform().UpdateAPIGateway(ctx, &platform.UpdateAPIGatewayOptions{
		APIGatewayConfig:           apiGatewayConfig,
		AuthSession:                agr.getCtxSession(request),
		ValidateFunctionsExistence: agr.headerValueIsTrue(request, "x-nuclio-agw-validate-functions-existence"),
	}); err != nil {
		agr.Logger.WarnWithCtx(ctx, "Failed to update api gateway", "err", err)
	}

	// return the stuff
	return &restful.CustomRouteFuncResponse{
		ResourceType: "apiGateway",
		Single:       true,
		StatusCode:   common.ResolveErrorStatusCodeOrDefault(err, http.StatusNoContent),
	}, err
}

// GetCustomRoutes returns a list of custom routes for the resource
func (agr *apiGatewayResource) GetCustomRoutes() ([]restful.CustomRoute, error) {

	// since delete and update by default assume /resource/{id} and we want to get the id/namespace from the body
	// we need to register custom routes
	return []restful.CustomRoute{
		{
			Pattern:   "/",
			Method:    http.MethodPut,
			RouteFunc: agr.updateAPIGateway,
		},
		{
			Pattern:   "/",
			Method:    http.MethodDelete,
			RouteFunc: agr.deleteAPIGateway,
		},
	}, nil
}

func (agr *apiGatewayResource) export(ctx context.Context, apiGateway platform.APIGateway) restful.Attributes {
	apiGatewayConfig := apiGateway.GetConfig()

	agr.Logger.DebugWithCtx(ctx, "Preparing api-gateway for export", "apiGatewayName", apiGatewayConfig.Meta.Name)
	apiGatewayConfig.PrepareAPIGatewayForExport(false)

	agr.Logger.DebugWithCtx(ctx, "Exporting api-gateway", "functionName", apiGatewayConfig.Meta.Name)

	attributes := restful.Attributes{
		"metadata": apiGatewayConfig.Meta,
		"spec":     apiGatewayConfig.Spec,
	}

	return attributes
}

// returns (id, attributes, error)
func (agr *apiGatewayResource) createAPIGateway(request *http.Request,
	apiGatewayInfoInstance *apiGatewayInfo) (string, restful.Attributes, error) {

	ctx, cancelCtx := context.WithCancel(nucliocontext.NewDetached(request.Context()))
	defer cancelCtx()

	// inject auth session to new context
	ctx = context.WithValue(ctx, auth.AuthSessionContextKey, agr.getCtxSession(request))

	// create an api gateway config
	apiGatewayConfig := platform.APIGatewayConfig{
		Meta: *apiGatewayInfoInstance.Meta,
		Spec: *apiGatewayInfoInstance.Spec,
	}

	if apiGatewayInfoInstance.Status != nil {
		apiGatewayConfig.Status = *apiGatewayInfoInstance.Status
	}

	// create an api gateway
	newAPIGateway, err := platform.NewAbstractAPIGateway(agr.Logger, agr.getPlatform(), apiGatewayConfig)
	if err != nil {
		return "", nil, nuclio.WrapErrInternalServerError(err)
	}

	// just deploy. the status is async through polling
	agr.Logger.DebugWithCtx(ctx, "Creating api gateway", "newAPIGateway", newAPIGateway)
	if err = agr.getPlatform().CreateAPIGateway(ctx, &platform.CreateAPIGatewayOptions{
		AuthSession:                ctx.Value(auth.AuthSessionContextKey).(auth.Session),
		APIGatewayConfig:           newAPIGateway.GetConfig(),
		ValidateFunctionsExistence: agr.headerValueIsTrue(request, "x-nuclio-agw-validate-functions-existence"),
	}); err != nil {
		if strings.Contains(errors.Cause(err).Error(), "already exists") {
			err = nuclio.WrapErrConflict(err)
		}

		return "", nil, err
	}

	// set attributes
	attributes := agr.apiGatewayToAttributes(newAPIGateway)
	agr.Logger.DebugWithCtx(ctx, "Successfully created api gateway", "attributes", attributes)

	return apiGatewayConfig.Meta.Name, attributes, nil
}

func (agr *apiGatewayResource) deleteAPIGateway(request *http.Request) (*restful.CustomRouteFuncResponse, error) {
	ctx := request.Context()

	// get api gateway config and status from body
	apiGatewayInfo, err := agr.getAPIGatewayInfoFromRequest(request)
	if err != nil {
		agr.Logger.WarnWithCtx(ctx, "Failed to get api gateway config and status from body", "err", err)

		return &restful.CustomRouteFuncResponse{
			Single:     true,
			StatusCode: http.StatusBadRequest,
		}, err
	}

	deleteAPIGatewayOptions := platform.DeleteAPIGatewayOptions{
		AuthSession: agr.getCtxSession(request),
	}
	deleteAPIGatewayOptions.Meta = *apiGatewayInfo.Meta

	if err = agr.getPlatform().DeleteAPIGateway(ctx, &deleteAPIGatewayOptions); err != nil {

		return &restful.CustomRouteFuncResponse{
			Single:     true,
			StatusCode: common.ResolveErrorStatusCodeOrDefault(err, http.StatusInternalServerError),
		}, err
	}

	return &restful.CustomRouteFuncResponse{
		ResourceType: "apiGateway",
		Single:       true,
		StatusCode:   http.StatusNoContent,
	}, err
}

func (agr *apiGatewayResource) apiGatewayToAttributes(apiGateway platform.APIGateway) restful.Attributes {
	attributes := restful.Attributes{
		"metadata": apiGateway.GetConfig().Meta,
		"spec":     apiGateway.GetConfig().Spec,
		"status":   apiGateway.GetConfig().Status,
	}

	return attributes
}

func (agr *apiGatewayResource) getNamespaceFromRequest(request *http.Request) string {
	return agr.getNamespaceOrDefault(request.Header.Get("x-nuclio-api-gateway-namespace"))
}

func (agr *apiGatewayResource) getAPIGatewayInfoFromRequest(request *http.Request) (*apiGatewayInfo, error) {

	// read body
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		return nil, nuclio.WrapErrInternalServerError(errors.Wrap(err, "Failed to read body"))
	}

	apiGatewayInfoInstance := apiGatewayInfo{}
	if err = json.Unmarshal(body, &apiGatewayInfoInstance); err != nil {
		return nil, nuclio.WrapErrBadRequest(errors.Wrap(err, "Failed to parse JSON body"))
	}

	// enrichment
	agr.enrichAPIGatewayInfo(&apiGatewayInfoInstance, request.Header.Get("x-nuclio-project-name"))

	return &apiGatewayInfoInstance, nil
}

func (agr *apiGatewayResource) enrichAPIGatewayInfo(apiGatewayInfoInstance *apiGatewayInfo, projectName string) {

	// ensure meta exists
	if apiGatewayInfoInstance.Meta == nil {
		apiGatewayInfoInstance.Meta = &platform.APIGatewayMeta{}
	}

	// enrich project name when specified
	if projectName != "" {
		if apiGatewayInfoInstance.Meta.Labels == nil {
			apiGatewayInfoInstance.Meta.Labels = map[string]string{}
		}

		apiGatewayInfoInstance.Meta.Labels[common.NuclioResourceLabelKeyProjectName] = projectName
	}

	// override namespace if applicable
	apiGatewayInfoInstance.Meta.Namespace = agr.getNamespaceOrDefault(apiGatewayInfoInstance.Meta.Namespace)

	// ensure spec exists
	if apiGatewayInfoInstance.Spec == nil {
		apiGatewayInfoInstance.Spec = &platform.APIGatewaySpec{}
	}

	// status is optional, ensure it exists
	if apiGatewayInfoInstance.Status == nil {
		apiGatewayInfoInstance.Status = &platform.APIGatewayStatus{}
	}
}

// register the resource
var apiGatewayResourceInstance = &apiGatewayResource{
	resource: newResource("api/api_gateways", []restful.ResourceMethod{
		restful.ResourceMethodGetList,
		restful.ResourceMethodGetDetail,
		restful.ResourceMethodCreate,
		restful.ResourceMethodUpdate,
	}),
}

func init() {
	apiGatewayResourceInstance.Resource = apiGatewayResourceInstance
	apiGatewayResourceInstance.Register(dashboard.DashboardResourceRegistrySingleton)
}
