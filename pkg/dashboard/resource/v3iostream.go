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

const (
	ConsumerGroup = "consumerGroup"
	ContainerName = "containerName"
	StreamPath    = "streamPath"
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

func (vsr *v3ioStreamResource) GetByID(request *http.Request, id string) (restful.Attributes, error) {

	// get and validate namespace
	namespace := vsr.getNamespaceFromRequest(request)
	if namespace == "" {
		return nil, nuclio.NewErrBadRequest("Namespace must exist")
	}

	//// get stream params from request
	//consumerGroup, containerName, streamPath, err := vsr.getStreamParamsFromRequest(request)
	//if err != nil {
	//	return nil, errors.Wrap(err, "Failed getting stream params from request")
	//}

	// read body
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		return nil, nuclio.WrapErrInternalServerError(errors.Wrap(err, "Failed to read body"))
	}

	v3ioStreamInfoInstance := v3ioStreamInfo{}
	if err = json.Unmarshal(body, &v3ioStreamInfoInstance); err != nil {
		return nil, nuclio.WrapErrBadRequest(errors.Wrap(err, "Failed to parse JSON body"))
	}

	// -- create v3io go client --
	// TOMER - see how to extract the correct url and access key
	url := "https://webapi.default-tenant.app.dev62.lab.iguazeng.com" // "https://somewhere:8444"
	accessKey := "fec5a247-c0a5-42b7-a7fb-6cd4d5bf36ff"               // "some-access-key"

	// create v3io context
	v3ioContext, err := v3iohttp.NewContext(vsr.Logger, &v3iohttp.NewContextInput{})
	if err != nil {
		return nil, errors.Wrap(err, "Failed creating v3io context")
	}

	// create v3io session
	v3ioSession, err := v3ioContext.NewSession(&v3io.NewSessionInput{
		URL:       url,
		AccessKey: accessKey,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed creating v3io session")
	}

	v3ioContainer, err := v3ioSession.NewContainer(&v3io.NewContainerInput{
		ContainerName: v3ioStreamInfoInstance.ContainerName,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed creating v3io container")
	}

	// for every shard - get shard lag details
	shardLags := map[int]restful.Attributes{}

	shardCount := 3

	for shardID := 0; shardID < shardCount; shardID++ {
		current, committed, err := vsr.getShardLagDetails(v3ioContainer,
			v3ioStreamInfoInstance.ConsumerGroup,
			v3ioStreamInfoInstance.StreamPath,
			shardID)
		if err != nil {
			return nil, errors.Wrap(err, "Failed getting shard lag details")
		}
		shardLags[shardID] = restful.Attributes{
			"lag":       current - committed,
			"current":   current,
			"committed": committed,
		}
	}

	attributes := restful.Attributes{
		"shardLags": shardLags,
	}

	return attributes, nil
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

func (vsr *v3ioStreamResource) getStreamParamsFromRequest(request *http.Request) (string, string, string, error) {
	consumerGroup := vsr.GetURLParamStringOrDefault(request, ConsumerGroup, "")
	if consumerGroup == "" {
		return "", "", "", nuclio.NewErrBadRequest("Consumer group name must not be empty")
	}
	containerName := vsr.GetURLParamStringOrDefault(request, ContainerName, "")
	if consumerGroup == "" {
		return "", "", "", nuclio.NewErrBadRequest("Container name must not be empty")
	}
	streamPath := vsr.GetURLParamStringOrDefault(request, StreamPath, "")
	if consumerGroup == "" {
		return "", "", "", nuclio.NewErrBadRequest("Stream path must not be empty")
	}

	return consumerGroup, containerName, streamPath, nil
}

func (vsr *v3ioStreamResource) getShardLagDetails(v3ioContainer v3io.Container, consumerGroup, streamPath string, shardID int) (int, int, error) {
	response, err := v3ioContainer.GetItemSync(&v3io.GetItemInput{
		Path: fmt.Sprintf("%s/%d", streamPath, shardID),
		AttributeNames: []string{
			"__last_sequence_num",
			fmt.Sprintf("__%s_committed_sequence_number", consumerGroup),
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
		consumerGroup))
	if err != nil {
		if !strings.Contains(err.Error(), "Not found") {
			return 0, 0, err
		}
		committed = 0
	}

	return current, committed, nil
}

// register the resource
var v3ioStreamResourceInstance = &v3ioStreamResource{
	resource: newResource("api/v3io_streams", []restful.ResourceMethod{
		restful.ResourceMethodGetList,
		restful.ResourceMethodGetDetail,
	}),
}

func init() {
	v3ioStreamResourceInstance.Resource = v3ioStreamResourceInstance
	v3ioStreamResourceInstance.Register(dashboard.DashboardResourceRegistrySingleton)
}
