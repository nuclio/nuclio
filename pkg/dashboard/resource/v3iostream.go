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
	"strconv"
	"strings"
	"sync"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/dashboard/auth"
	"github.com/nuclio/nuclio/pkg/errgroup"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/opa"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
	v3io "github.com/v3io/v3io-go/pkg/dataplane"
	v3iohttp "github.com/v3io/v3io-go/pkg/dataplane/http"
	"github.com/valyala/fasthttp"
)

type v3ioStreamResource struct {
	*resource
	v3ioHTTPClient *fasthttp.Client
}

type v3ioStreamInfo struct {
	ConsumerGroup string `json:"consumerGroup,omitempty"`
	ContainerName string `json:"containerName,omitempty"`
	StreamPath    string `json:"streamPath,omitempty"`
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
			Pattern:   "/get_shard_lags",
			Method:    http.MethodPost,
			RouteFunc: vsr.getStreamShardLags,
		},
	}, nil
}

func (vsr *v3ioStreamResource) getStreamShardLags(request *http.Request) (*restful.CustomRouteFuncResponse, error) {

	ctx := request.Context()

	err := vsr.validateRequest(request)
	if err != nil {
		return nil, errors.Wrap(err, "Request validation failed")
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

	var consumerGroups []string
	if v3ioStreamInfoInstance.ConsumerGroup == "" {

		// Not supported
		return nil, nuclio.WrapErrBadRequest(errors.Wrap(err, "Request must have a consumerGroup"))

		// TODO: get all consumer groups listening on the stream
		//err = vsr.enrichStreamConsumerGroups(&consumerGroups, &v3ioStreamInfoInstance)
		//if err != nil {
		//	return nil, errors.Wrap(err, "Failed getting stream's consumer groups")
		//}

	} else {
		consumerGroups = append(consumerGroups, v3ioStreamInfoInstance.ConsumerGroup)
	}

	shardLags, err := vsr.getShardLagsMap(ctx, &v3ioStreamInfoInstance, consumerGroups)
	if err != nil {
		return nil, errors.Wrap(err, "Failed getting shard lags")
	}

	return &restful.CustomRouteFuncResponse{
		Resources:  shardLags,
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
		AuthSession: vsr.getCtxSession(ctx),
		PermissionOptions: opa.PermissionOptions{
			MemberIds:           opa.GetUserAndGroupIdsFromAuthSession(vsr.getCtxSession(ctx)),
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

func (vsr *v3ioStreamResource) enrichStreamConsumerGroups(consumerGroups *[]string, streamInfo *v3ioStreamInfo) error {

	// TODO: get all consumer groups that consume from this stream and container

	return nil
}

func (vsr *v3ioStreamResource) getShardLagsMap(ctx context.Context,
	info *v3ioStreamInfo,
	consumerGroups []string) (map[string]restful.Attributes, error) {

	// create v3io context
	v3ioContext, err := v3iohttp.NewContext(vsr.Logger, &v3iohttp.NewContextInput{

		// allow changing http client for testing purposes
		HTTPClient: vsr.v3ioHTTPClient,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed creating v3io context")
	}

	// a data plane access key is required for v3io operations
	accessKey := vsr.getCtxSession(ctx).GetPassword()
	if accessKey == "" {
		return nil, errors.Wrap(err, "A data plane access key is required")
	}

	dataPlaneInput := v3io.DataPlaneInput{
		URL:           vsr.getPlatform().GetConfig().StreamMonitoring.WebapiURL,
		AccessKey:     accessKey,
		ContainerName: info.ContainerName,
	}
	getContainerContentsInput := &v3io.GetContainerContentsInput{
		Path:             info.StreamPath,
		GetAllAttributes: true,
		DirectoriesOnly:  false,
		DataPlaneInput:   dataPlaneInput,
	}

	shardLagsMap := map[string]restful.Attributes{}
	lock := sync.Mutex{}

	// we get the v3io container contents in batches until we go over everything
	// when we find a shard content, get its lag details
	for {
		response, err := v3ioContext.GetContainerContentsSync(getContainerContentsInput)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get container subdirectories")
		}

		getContainerContentsOutput := response.Output.(*v3io.GetContainerContentsOutput)
		response.Release()

		errGroup, _ := errgroup.WithContextSemaphore(ctx,
			vsr.Logger,
			vsr.getPlatform().GetConfig().StreamMonitoring.GetStreamShardsConcurrentRequests)

		// Iterate over subdirectories listed in the response, and look for shards
		for _, content := range getContainerContentsOutput.Contents {
			content := content
			for _, consumerGroup := range consumerGroups {

				errGroup.Go("get-single-shard-lags", func() error {

					// each file name that is just a number is a shard
					filePath := strings.Split(content.Key, "/")
					shardID, err := strconv.Atoi(filePath[len(filePath)-1])
					if err != nil {

						// not a shard, we can skip it
						return nil
					}

					current, committed, err := vsr.getSingleShardLagDetails(v3ioContext,
						info.StreamPath,
						consumerGroup,
						shardID,
						&dataPlaneInput)
					if err != nil {
						return errors.Wrapf(err, "Failed getting shard lag details, shardID: %v", shardID)
					}

					lock.Lock()
					shardLagsMap[consumerGroup][strconv.Itoa(shardID)] = restful.Attributes{
						"lag":       current - committed,
						"current":   current,
						"committed": committed,
					}
					lock.Unlock()

					return nil
				})
			}
		}

		if err := errGroup.Wait(); err != nil {
			return nil, errors.Wrap(err, "Failed getting at least one shard lags")
		}

		// if nextMarker not empty or isTruncated="true" - there are more children (need another fetch to get them)
		if !getContainerContentsOutput.IsTruncated || len(getContainerContentsOutput.NextMarker) == 0 {

			// there is no more content in the container, we can exit the loop
			break
		}
		getContainerContentsInput.Marker = getContainerContentsOutput.NextMarker
	}

	return shardLagsMap, nil
}

func (vsr *v3ioStreamResource) getSingleShardLagDetails(v3ioContext v3io.Context,
	streamPath,
	consumerGroup string,
	shardID int,
	dataPlaneInput *v3io.DataPlaneInput) (int, int, error) {

	response, err := v3ioContext.GetItemSync(&v3io.GetItemInput{
		DataPlaneInput: *dataPlaneInput,
		Path:           fmt.Sprintf("%s/%d", streamPath, shardID),
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

		// the stream is lazily created - it exists but there is still no data in it so nothing is committed yet
		committed = 0
	}

	return current, committed, nil
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
