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
	"io"
	"net/http"
	"strings"

	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/common"
	v3io "github.com/nuclio/nuclio/pkg/common/v3io"
	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/opa"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
)

type v3ioStreamResource struct {
	*resource
}

func (vsr *v3ioStreamResource) ExtendMiddlewares() error {
	vsr.resource.addAuthMiddleware(&auth.Options{

		// we need a data plane session for accessing the v3io stream container
		EnrichDataPlane: true,
	})
	return nil
}

func (vsr *v3ioStreamResource) GetAll(request *http.Request) (map[string]restful.Attributes, error) {

	functions, err := vsr.getFunctions(request)
	if err != nil {
		return nil, errors.Wrap(err, "Failed getting project functions")
	}

	// iterate over functions and look for v3iostreams
	streams, err := v3io.GetStreamsFromFunctions(functions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed getting streams from functions")
	}

	return streams, nil
}

// GetCustomRoutes returns a list of custom routes for the resource
func (vsr *v3ioStreamResource) GetCustomRoutes() ([]restful.CustomRoute, error) {

	return []restful.CustomRoute{
		{
			Pattern:   "/get_shard_lags",
			Method:    http.MethodPost,
			RouteFunc: vsr.getStreamShardLags,
		},
	}, nil
}

func (vsr *v3ioStreamResource) getFunctions(request *http.Request) ([]platform.Function, error) {

	// ensure namespace
	namespace := vsr.getNamespaceFromRequest(request)
	if namespace == "" {
		return nil, nuclio.NewErrBadRequest("Namespace must exist")
	}

	// ensure project name
	projectName := request.Header.Get("x-nuclio-project-name")
	if projectName == "" {
		return nil, nuclio.NewErrBadRequest("Project name must not be empty")
	}

	// get project functions
	ctx := request.Context()
	getFunctionsOptions := &platform.GetFunctionsOptions{
		Name:      "",
		Namespace: namespace,
		Labels: fmt.Sprintf("%s=%s",
			common.NuclioResourceLabelKeyProjectName,
			projectName),
		AuthSession: vsr.getCtxSession(ctx),
		PermissionOptions: opa.PermissionOptions{
			MemberIds:           opa.GetUserAndGroupIdsFromAuthSession(vsr.getCtxSession(ctx)),
			OverrideHeaderValue: request.Header.Get(opa.OverrideHeader),
		},
	}

	return vsr.getPlatform().GetFunctions(ctx, getFunctionsOptions)
}

func (vsr *v3ioStreamResource) getStreamShardLags(request *http.Request) (*restful.CustomRouteFuncResponse, error) {

	ctx := request.Context()

	if err := vsr.validateRequest(request); err != nil {
		return nil, errors.Wrap(err, "Request validation failed")
	}

	// read body
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, nuclio.WrapErrInternalServerError(errors.Wrap(err, "Failed to read body"))
	}

	v3ioStreamInfoInstance := v3io.StreamInfo{}
	if err := json.Unmarshal(body, &v3ioStreamInfoInstance); err != nil {
		return nil, nuclio.WrapErrBadRequest(errors.Wrap(err, "Failed to parse JSON body"))
	}

	if err := vsr.validateRequestBody(&v3ioStreamInfoInstance); err != nil {
		return nil, errors.Wrap(err, "Request body validation failed")
	}

	// TODO: this is a set-up for supporting multiple consumer groups
	var consumerGroups []string
	consumerGroups = append(consumerGroups, v3ioStreamInfoInstance.ConsumerGroup)

	shardLags, err := v3io.GetShardLagsMap(ctx,
		vsr.getCtxSession(ctx).GetPassword(),
		vsr.Logger,
		vsr.getPlatform().GetConfig(),
		&v3ioStreamInfoInstance,
		consumerGroups)
	if err != nil {
		return nil, errors.Wrap(err, "Failed getting shard lags")
	}

	streamKey := v3ioStreamInfoInstance.ContainerName + v3ioStreamInfoInstance.StreamPath
	return &restful.CustomRouteFuncResponse{
		Resources: map[string]restful.Attributes{
			streamKey: shardLags,
		},
		Single:     false,
		Headers:    map[string]string{"Content-Type": "application/json"},
		StatusCode: http.StatusOK,
	}, nil
}

func (vsr *v3ioStreamResource) getNamespaceFromRequest(request *http.Request) string {
	return vsr.getNamespaceOrDefault(request.Header.Get("x-nuclio-project-namespace"))
}

func (vsr *v3ioStreamResource) validateRequest(request *http.Request) error {

	ctx := request.Context()

	// get and validate namespace
	namespace := vsr.getNamespaceFromRequest(request)
	if namespace == "" {
		return nuclio.NewErrBadRequest("Namespace must exist")
	}

	projectName := request.Header.Get("x-nuclio-project-name")
	if projectName == "" {
		return errors.New("Project name must not be empty")
	}

	// getting projects for validating project read permissions
	if _, err := vsr.getPlatform().GetProjects(ctx, &platform.GetProjectsOptions{
		Meta: platform.ProjectMeta{
			Name:      projectName,
			Namespace: namespace,
		},
		AuthSession: vsr.getCtxSession(ctx),
		PermissionOptions: opa.PermissionOptions{
			MemberIds:           opa.GetUserAndGroupIdsFromAuthSession(vsr.getCtxSession(ctx)),
			OverrideHeaderValue: request.Header.Get(opa.OverrideHeader),
		},
	}); err != nil {
		return nuclio.NewErrUnauthorized("Unauthorized to read project")
	}

	return nil
}

func (vsr *v3ioStreamResource) validateRequestBody(v3ioStreamInfo *v3io.StreamInfo) error {

	if v3ioStreamInfo.StreamPath == "" {
		return nuclio.NewErrBadRequest("Stream path must be provided in request body")
	}
	if v3ioStreamInfo.ContainerName == "" {
		return nuclio.NewErrBadRequest("Container name must be provided in request body")
	}
	if v3ioStreamInfo.ConsumerGroup == "" {

		// TODO: get all consumer groups listening on the stream
		return nuclio.NewErrBadRequest("Stream lags for multiple consumer groups is not supported. Please specify consumer group in request body")
	}

	// enrich stream path syntax
	if !strings.HasPrefix(v3ioStreamInfo.StreamPath, "/") {
		v3ioStreamInfo.StreamPath = "/" + v3ioStreamInfo.StreamPath
	}
	return nil
}

// register the resource
var v3ioStreamResourceInstance = &v3ioStreamResource{
	resource: newResource("api/v3io_streams", []restful.ResourceMethod{
		restful.ResourceMethodGetList,
	}),
}

func init() {
	v3ioStreamResourceInstance.Resource = v3ioStreamResourceInstance
	v3ioStreamResourceInstance.Register(dashboard.DashboardResourceRegistrySingleton)
}
