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

package v3io

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/nuclio/nuclio/pkg/errgroup"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	v3iodataplane "github.com/v3io/v3io-go/pkg/dataplane"
	v3iohttp "github.com/v3io/v3io-go/pkg/dataplane/http"
	v3ioerrors "github.com/v3io/v3io-go/pkg/errors"
)

type StreamInfo struct {
	ConsumerGroup string `json:"consumerGroup,omitempty"`
	ContainerName string `json:"containerName,omitempty"`
	StreamPath    string `json:"streamPath,omitempty"`
}

func GetStreamsFromFunctions(functions []platform.Function) (map[string]restful.Attributes, error) {

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

func GetShardLagsMap(ctx context.Context,
	accessKey string,
	logger logger.Logger,
	platformConfig *platformconfig.Config,
	info *StreamInfo,
	consumerGroups []string) (map[string]interface{}, error) {

	// a data plane access key is required for v3io operations
	if accessKey == "" {
		return nil, errors.New("A data plane access key is required")
	}

	// create v3io context
	v3ioContext, err := v3iohttp.NewContext(logger, &v3iohttp.NewContextInput{})
	if err != nil {
		return nil, errors.Wrap(err, "Failed creating v3io context")
	}

	dataPlaneInput := v3iodataplane.DataPlaneInput{
		URL:           platformConfig.StreamMonitoring.WebapiURL,
		AccessKey:     accessKey,
		ContainerName: info.ContainerName,
	}
	getContainerContentsInput := &v3iodataplane.GetContainerContentsInput{
		Path:             info.StreamPath,
		GetAllAttributes: true,
		DirectoriesOnly:  false,
		DataPlaneInput:   dataPlaneInput,
	}

	shardLagsMap := map[string]interface{}{}
	lock := sync.Mutex{}

	// we get the v3io container contents in batches until we go over everything
	// when we find a shard content, get its lag details
	for {
		response, err := v3ioContext.GetContainerContentsSync(getContainerContentsInput)
		if err != nil {
			errMessage := "Failed to get container subdirectories"
			if v3ioErr, ok := err.(v3ioerrors.ErrorWithStatusCode); ok {

				// Convert to nuclio error
				return nil, errors.Wrap(nuclio.GetByStatusCode(v3ioErr.StatusCode())(v3ioErr.Error()), errMessage)
			}
			return nil, errors.Wrap(err, errMessage)
		}

		getContainerContentsOutput := response.Output.(*v3iodataplane.GetContainerContentsOutput)
		response.Release()

		errGroup, _ := errgroup.WithContextSemaphore(ctx,
			logger,
			platformConfig.StreamMonitoring.V3ioRequestConcurrency)

		// iterate over subdirectories listed in the response, and look for shards
		for _, content := range getContainerContentsOutput.Contents {
			content := content

			// each file name that is just a number is a shard
			filePath := strings.Split(content.Key, "/")
			shardID, err := strconv.Atoi(filePath[len(filePath)-1])
			if err != nil {

				// not a shard, we can skip it
				break
			}

			for _, consumerGroup := range consumerGroups {
				consumerGroup := consumerGroup
				if shardLagsMap[consumerGroup] == nil {
					shardLagsMap[consumerGroup] = map[string]restful.Attributes{}
				}

				errGroup.Go("get-single-shard-lags", func() error {
					current, committed, err := GetSingleShardLagDetails(v3ioContext,
						info.StreamPath,
						consumerGroup,
						shardID,
						&dataPlaneInput)
					if err != nil {
						return errors.Wrapf(err, "Failed getting shard lag details, shardID: %v", shardID)
					}

					lock.Lock()
					shardLagsMap[consumerGroup].(map[string]restful.Attributes)[strconv.Itoa(shardID)] = restful.Attributes{
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

func GetSingleShardLagDetails(v3ioContext v3iodataplane.Context,
	streamPath,
	consumerGroup string,
	shardID int,
	dataPlaneInput *v3iodataplane.DataPlaneInput) (int, int, error) {

	response, err := v3ioContext.GetItemSync(&v3iodataplane.GetItemInput{
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
	getItemOutput := response.Output.(*v3iodataplane.GetItemOutput)

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
