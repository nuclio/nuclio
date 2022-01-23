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
	"strconv"
	"strings"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/opa"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
	v3io "github.com/v3io/v3io-go/pkg/dataplane"
	v3iohttp "github.com/v3io/v3io-go/pkg/dataplane/http"
)

type v3ioStreamResource struct {
	*resource
}

type v3ioStreamInfo struct {
	ConsumerGroup string `json:"consumerGroup,omitempty"`
	ContainerName string `json:"containerName,omitempty"`
	StreamPath    string `json:"streamPath,omitempty"`
}

func (vsr *v3ioStreamResource) ExtendMiddlewares() error {
	vsr.resource.addAuthMiddleware()
	return nil
}

func (vsr *v3ioStreamResource) GetAll(request *http.Request) (map[string]restful.Attributes, error) {

	functions, err := vsr.getFunctions(request)
	if err != nil {
		return nil, errors.Wrap(err, "Failed getting project functions")
	}

	// iterate over functions and look for v3iostreams
	streams, err := vsr.getStreamsFromFunctions(functions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed getting streams from functions")
	}

	return streams, nil
}

// GetCustomRoutes returns a list of custom routes for the resource
func (vsr *v3ioStreamResource) GetCustomRoutes() ([]restful.CustomRoute, error) {

	return []restful.CustomRoute{
		{
			Pattern:   "/get-shard-lags",
			Method:    http.MethodPost,
			RouteFunc: vsr.getStreamShardLags,
		},
	}, nil
}

func (vsr *v3ioStreamResource) getStreamShardLags(request *http.Request) (*restful.CustomRouteFuncResponse, error) {

	ctx := request.Context()

	// get and validate namespace
	namespace := vsr.getNamespaceFromRequest(request)
	if namespace == "" {
		return nil, nuclio.NewErrBadRequest("Namespace must exist")
	}

	projectName := request.Header.Get("x-nuclio-project-name")
	if projectName == "" {
		return nil, errors.New("Project name must not be empty")
	}

	functionName := request.Header.Get("x-nuclio-function-name")
	if functionName == "" {
		return nil, errors.New("Function name must not be empty")
	}

	// getting projects for validating project read permissions
	if _, err := vsr.getPlatform().GetProjects(ctx, &platform.GetProjectsOptions{
		Meta: platform.ProjectMeta{
			Name:      projectName,
			Namespace: namespace,
		},
		AuthSession: vsr.getCtxSession(request),
		PermissionOptions: opa.PermissionOptions{
			MemberIds:           opa.GetUserAndGroupIdsFromAuthSession(vsr.getCtxSession(request)),
			OverrideHeaderValue: request.Header.Get(opa.OverrideHeader),
		},
	}); err != nil {
		return nil, nuclio.NewErrUnauthorized("Unauthorized to read project")
	}

	// read body
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		return nil, nuclio.WrapErrInternalServerError(errors.Wrap(err, "Failed to read body"))
	}

	v3ioStreamInfoInstance := v3ioStreamInfo{}
	if err = json.Unmarshal(body, &v3ioStreamInfoInstance); err != nil {
		return nil, nuclio.WrapErrBadRequest(errors.Wrap(err, "Failed to parse JSON body"))
	}

	shardLags, err := vsr.getShardLagsMap(v3ioStreamInfoInstance)
	if err != nil {
		return nil, errors.Wrap(err, "Failed getting shard lags")
	}

	return &restful.CustomRouteFuncResponse{
		Resources: map[string]restful.Attributes{
			"meta": {
				"projectName":  projectName,
				"functionName": functionName,
			},
			"streamShardLags": map[string]interface{}{
				"shardLags": shardLags,
			},
		},
		Single:     false,
		Headers:    map[string]string{"Content-Type": "application/json"},
		StatusCode: http.StatusOK,
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
		AuthSession: vsr.getCtxSession(request),
		PermissionOptions: opa.PermissionOptions{
			MemberIds:           opa.GetUserAndGroupIdsFromAuthSession(vsr.getCtxSession(request)),
			OverrideHeaderValue: request.Header.Get(opa.OverrideHeader),
		},
	}

	return vsr.getPlatform().GetFunctions(ctx, getFunctionsOptions)
}

func (vsr *v3ioStreamResource) getStreamsFromFunctions(functions []platform.Function) (map[string]restful.Attributes, error) {

	streamsMap := map[string]restful.Attributes{}

	for _, function := range functions {
		v3ioStreamsMap := functionconfig.GetTriggersByKind(function.GetConfig().Spec.Triggers, "v3ioStream")
		for streamName, stream := range v3ioStreamsMap {

			// add stream to map, with a key in the format: "function-name@stream-name"
			keyName := fmt.Sprintf("%s@%s", function.GetConfig().Meta.Name, streamName)
			streamsMap[keyName] = restful.Attributes{
				"consumerGroup": stream.Attributes["consumerGroup"],
				"containerName": stream.Attributes["containerName"],
				"streamPath":    stream.Attributes["streamPath"],
			}
		}
	}

	return streamsMap, nil
}

func (vsr *v3ioStreamResource) getNamespaceFromRequest(request *http.Request) string {
	return vsr.getNamespaceOrDefault(request.Header.Get("x-nuclio-project-namespace"))
}

func (vsr *v3ioStreamResource) getSingleShardLagDetails(v3ioContext v3io.Context, info v3ioStreamInfo, shardID int, dataPlaneInput v3io.DataPlaneInput) (int, int, error) {
	response, err := v3ioContext.GetItemSync(&v3io.GetItemInput{
		DataPlaneInput: dataPlaneInput,
		Path:           fmt.Sprintf("%s/%d", info.StreamPath, shardID),
		AttributeNames: []string{
			"__last_sequence_num",
			fmt.Sprintf("__%s_committed_sequence_number", info.ConsumerGroup),
		},
	})
	if err != nil {
		return 0, 0, err
	}
	defer response.Release()
	getItemOutput := response.Output.(*v3io.GetItemOutput)

	current, err := getItemOutput.Item.GetFieldInt("__last_sequence_num")
	if err != nil {
		return 0, 0, err
	}

	committed, err := getItemOutput.Item.GetFieldInt(fmt.Sprintf("__%s_committed_sequence_number",
		info.ConsumerGroup))
	if err != nil {
		if !strings.Contains(err.Error(), "Not found") {
			return 0, 0, err
		}
		committed = 0
	}

	return current, committed, nil
}

func (vsr *v3ioStreamResource) getShardLagsMap(info v3ioStreamInfo) (map[string]restful.Attributes, error) {

	// create v3io context
	v3ioContext, err := v3iohttp.NewContext(vsr.Logger, &v3iohttp.NewContextInput{})
	if err != nil {
		return nil, errors.Wrap(err, "Failed creating v3io context")
	}

	// For testing purposes, use the webapi url from your machine
	//url := "https://webapi.default-tenant.app.dev62.lab.iguazeng.com" // "https://somewhere:8444"
	//accessKey := "f68221c5-2320-4ba7-b52b-b8e7d876eb86"               // "some-access-key"

	dataPlaneInput := v3io.DataPlaneInput{
		URL:           vsr.getPlatform().GetConfig().Stream.WebapiURL,
		AccessKey:     vsr.getPlatform().GetConfig().Stream.AccessKey,
		ContainerName: info.ContainerName,
	}
	getContainerContentsInput := &v3io.GetContainerContentsInput{
		Path:             info.StreamPath,
		GetAllAttributes: true,
		DirectoriesOnly:  false,
		DataPlaneInput:   dataPlaneInput,
	}

	shardLags := map[string]restful.Attributes{}

	for {
		response, err := v3ioContext.GetContainerContentsSync(getContainerContentsInput)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get container subdirectories")
		}

		getContainerContentsOutput := response.Output.(*v3io.GetContainerContentsOutput)
		response.Release()

		for _, content := range getContainerContentsOutput.Contents {
			content := content

			// each file name that is just a number is a shard
			filePath := strings.Split(content.Key, "/")
			if shardID, err := strconv.Atoi(filePath[len(filePath)-1]); err == nil {
				current, committed, err := vsr.getSingleShardLagDetails(v3ioContext,
					info,
					shardID,
					dataPlaneInput)
				if err != nil {
					return nil, errors.Wrap(err, "Failed getting shard lag details")
				}
				shardLags[strconv.Itoa(shardID)] = restful.Attributes{
					"lag":       current - committed,
					"current":   current,
					"committed": committed,
				}
			}
		}

		// if nextMarker not empty or isTruncated="true" - there are more children (need another fetch to get them)
		if !getContainerContentsOutput.IsTruncated || len(getContainerContentsOutput.NextMarker) == 0 {
			break
		}
		getContainerContentsInput.Marker = getContainerContentsOutput.NextMarker

	}

	return shardLags, nil
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
