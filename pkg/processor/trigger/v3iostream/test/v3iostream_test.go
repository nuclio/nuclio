//go:build test_integration && test_iguazio

/*
Copyright 2018 The Nuclio Authors.

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

package test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/test"

	"github.com/nuclio/errors"
	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
	"github.com/v3io/v3io-go/pkg/dataplane"
	v3iohttp "github.com/v3io/v3io-go/pkg/dataplane/http"
	v3ioscg "github.com/v3io/v3io-go/pkg/dataplane/streamconsumergroup"
	v3ioerrors "github.com/v3io/v3io-go/pkg/errors"
	"gopkg.in/yaml.v2"
)

type testSuite struct {
	*triggertest.AbstractBrokerSuite

	// kafka clients
	v3ioContext   v3io.Context
	v3ioContainer v3io.Container
	v3ioSession   v3io.Session
	numWorkers    int
	shardCount    int

	// v3io trigger attributes
	url           string
	accessKey     string
	containerName string
	streamPath    string
	consumerGroup string
}

func (suite *testSuite) SetupSuite() {
	var err error

	// Change below
	suite.url = "https://default-tenant.app.dev62.lab.iguazeng.com:8444" // "https://somewhere:8444"
	suite.accessKey = "f68221c5-2320-4ba7-b52b-b8e7d876eb86"             // "some-access-key"
	// END OF change

	suite.numWorkers = 8
	suite.shardCount = 1

	// v3io trigger attributes
	suite.containerName = "bigdata"
	suite.streamPath = fmt.Sprintf("test-nuclio-%s/", xid.New().String())   // must have `/` at the end
	suite.consumerGroup = fmt.Sprintf("test_nuclio_%s", xid.New().String()) // use `_` not `-`

	// we use an Iguazio system, not a containerzed broker
	suite.AbstractBrokerSuite.SkipStartBrokerContainer = true
	suite.AbstractBrokerSuite.SetupSuite()

	// create broker
	suite.v3ioContext, err = v3iohttp.NewContext(suite.Logger, &v3iohttp.NewContextInput{
		NumWorkers: suite.numWorkers,
	})
	suite.Require().NoError(err, "Failed to create v3io context")

	suite.v3ioSession, err = suite.v3ioContext.NewSession(&v3io.NewSessionInput{
		URL:       suite.url,
		AccessKey: suite.accessKey,
	})
	suite.Require().NoError(err, "Failed to create v3io session")

	suite.v3ioContainer, err = suite.v3ioSession.NewContainer(&v3io.NewContainerInput{
		ContainerName: suite.containerName,
	})
	suite.Require().NoError(err, "Failed to create v3io container")

	err = suite.v3ioContainer.CreateStreamSync(&v3io.CreateStreamInput{
		Path:                 suite.streamPath,
		ShardCount:           suite.shardCount,
		RetentionPeriodHours: 1,
	})
	suite.Require().NoError(err, "Failed to create v3io sync stream")
}

func (suite *testSuite) TearDownSuite() {
	suite.Logger.DebugWith("Deleting stream sync", "streamPath", suite.streamPath)
	err := suite.v3ioContainer.DeleteStreamSync(&v3io.DeleteStreamInput{
		Path: suite.streamPath,
	})
	suite.Require().NoError(err)
	suite.AbstractBrokerSuite.TearDownSuite()
}

func (suite *testSuite) TestAckWindowSize() {
	ackWindowSize := 10
	shardID := 0
	recordedEvents := 0
	createFunctionOptions := suite.GetDeployOptions("event_recorder", suite.FunctionPaths["python"])
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		"test-nuclio-v3io": {
			Kind:     "v3ioStream",
			URL:      suite.url,
			Password: suite.accessKey,
			Attributes: map[string]interface{}{
				"seekTo":        "earliest",
				"containerName": suite.containerName,
				"streamPath":    suite.streamPath,
				"consumerGroup": suite.consumerGroup,
				"ackWindowSize": ackWindowSize,
			},
		},
	}

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		suite.Require().NotNil(deployResult, "Unexpected empty deploy results")

		// send window-size minus one messages
		for messageIdx := 0; messageIdx < ackWindowSize; messageIdx++ {
			recordedEvents += 1
			messageBody := fmt.Sprintf("%s-%d", suite.streamPath, messageIdx)
			err := suite.writingMessageToStream(suite.streamPath, messageBody)
			suite.Require().NoError(err, "Failed to publish message")
		}

		err := common.RetryUntilSuccessful(10*time.Second,
			time.Second,
			func() bool {
				receivedEvents := triggertest.GetEventRecorderReceivedEvents(suite.Suite,
					suite.Logger,
					suite.BrokerHost,
					deployResult.Port)
				return len(receivedEvents) == ackWindowSize
			})
		suite.Require().NoError(err,
			"Exhausted waiting for received event length to be equal to window size")

		// received all events
		receivedEvents := triggertest.GetEventRecorderReceivedEvents(suite.Suite,
			suite.Logger,
			suite.BrokerHost,
			deployResult.Port)
		suite.Require().Len(receivedEvents, ackWindowSize)

		current, committed := suite.getShardLagDetails(shardID)

		// ensure nothing was committed
		suite.Require().Equal(current, ackWindowSize)
		suite.Require().Equal(committed, 0)

		// send another message
		err = suite.writingMessageToStream(suite.streamPath, "trigger-commit")
		suite.Require().NoError(err, "Failed to publish message")
		recordedEvents += 1

		// ensure first commit
		err = common.RetryUntilSuccessful(10*time.Second,
			time.Second,
			func() bool {
				current, committed = suite.getShardLagDetails(shardID)
				return current-committed == ackWindowSize && committed == 1
			})
		suite.Require().NoError(err, "Message did not trigger shard commit")

		// clear recorded events now, because after we start the container
		// it will seekTo: earliest and that will cause the function to reprocess and record all events again
		err = suite.DockerClient.ExecInContainer(deployResult.ContainerID, &dockerclient.ExecOptions{
			Command: "rm /tmp/events.json",
		})
		suite.Require().NoError(err)

		severalMessagesWhileFunctionIsDown := ackWindowSize / 2

		// send several messages on stream (lesser than ack window size) while function container is down
		suite.WithFunctionContainerRestart(deployResult, func() {
			for messageIdx := 0; messageIdx < severalMessagesWhileFunctionIsDown; messageIdx++ {
				recordedEvents += 1
				messageBody := fmt.Sprintf("%s-%d", suite.streamPath, messageIdx)
				err := suite.writingMessageToStream(suite.streamPath, messageBody)
				suite.Require().NoError(err, "Failed to publish message")
			}
		})

		// ensure function got the messages (aka processed)
		err = common.RetryUntilSuccessful(10*time.Second,
			time.Second,
			func() bool {
				receivedEvents = triggertest.GetEventRecorderReceivedEvents(suite.Suite,
					suite.Logger,
					suite.BrokerHost,
					deployResult.Port)

				// first message was committed and hence was not "re processed"
				return len(receivedEvents) == recordedEvents-1

			})
		suite.Require().NoError(err, "Not all messages were committed as expected")

		// yet there is a lag because those messages were still not committed
		current, committed = suite.getShardLagDetails(shardID)
		suite.Require().Equal(current-committed, ackWindowSize)

		// "commit first message" + several messages sent after
		suite.Require().Equal(committed, 1+severalMessagesWhileFunctionIsDown)
		return true
	})
}

func (suite *testSuite) TestReceiveRecords() {
	createFunctionOptions := suite.GetDeployOptions("event_recorder", suite.FunctionPaths["python"])
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		"test-nuclio-v3io": {
			Kind:     "v3ioStream",
			URL:      suite.url,
			Password: suite.accessKey,
			Attributes: map[string]interface{}{
				"seekTo":        "earliest", // avoid race condition with `latest` missed by function
				"containerName": suite.containerName,
				"streamPath":    suite.streamPath,
				"consumerGroup": suite.consumerGroup,
			},
		},
	}

	triggertest.InvokeEventRecorder(&suite.AbstractBrokerSuite.TestSuite,
		suite.BrokerHost,
		createFunctionOptions,
		map[string]triggertest.TopicMessages{
			suite.streamPath: {
				NumMessages: suite.numWorkers,
			},
		},
		nil,
		suite.writingMessageToStream)
}

//func (suite *testSuite) TestShardAssignmentSanity() {
//	var err error
//
//	// create a stream with 9 shards
//	streamPath := fmt.Sprintf("test-nuclio-%s/", xid.New().String())
//	shardCount := 9
//
//	err = suite.v3ioContainer.CreateStreamSync(&v3io.CreateStreamInput{
//		Path:                 streamPath,
//		ShardCount:           shardCount,
//		RetentionPeriodHours: 1,
//	})
//	suite.Require().NoError(err, "Failed to create v3io sync stream")
//
//	// deploy a first function with maxReplicas=3
//	maxReplicas := 3
//	createFunctionOptions := suite.GetDeployOptions("event_recorder", suite.FunctionPaths["python"])
//	createFunctionOptions.FunctionConfig.Meta.Name = "event_recorder_test_1"
//	createFunctionOptions.FunctionConfig.Spec.MaxReplicas = &maxReplicas
//	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
//		"test-nuclio-v3io": {
//			Kind:     "v3ioStream",
//			URL:      suite.url,
//			Password: suite.accessKey,
//			Attributes: map[string]interface{}{
//				"seekTo":        "earliest", // avoid race condition with `latest` missed by function
//				"containerName": suite.containerName,
//				"streamPath":    streamPath,
//				"consumerGroup": suite.consumerGroup,
//			},
//		},
//	}
//
//	suite.Logger.Debug("Creating first function")
//	suite.DeployFunction(createFunctionOptions, func(deployResult1 *platform.CreateFunctionResult) bool {
//		suite.Logger.Debug("After deploying function number 1")
//
//		// check which shards the first function's trigger got
//		v3ioStreamsMap := functionconfig.GetTriggersByKind(deployResult1.UpdatedFunctionConfig.Spec.Triggers, "v3ioStream")
//		suite.Logger.DebugWith("First function stream map", "v3ioStreamsMap", v3ioStreamsMap)
//
//		partitions1 := v3ioStreamsMap["test-nuclio-v3io"].Partitions
//		suite.Logger.DebugWith("First function trigger partitions", "partitions", partitions1)
//
//		// deploy a second function with the same stream trigger
//		suite.Logger.Debug("Creating second function")
//		createFunctionOptions.FunctionConfig.Meta.Name = "event_recorder_test_2"
//		suite.DeployFunction(createFunctionOptions, func(deployResult2 *platform.CreateFunctionResult) bool {
//			suite.Logger.Debug("After deploying function number 2")
//
//			// stop and start the first function's container
//			err = suite.DockerClient.StopContainer(deployResult1.ContainerID)
//			suite.Require().NoError(err)
//
//			// stop and start the first function's container
//			err = suite.DockerClient.StartContainer(deployResult1.ContainerID)
//			suite.Require().NoError(err)
//
//			// check which shards the first and second function's trigger got
//			partitions1 = deployResult1.UpdatedFunctionConfig.Spec.Triggers["test-nuclio-v3io"].Partitions
//			suite.Logger.DebugWith("First function trigger partitions", "partitions", partitions1)
//
//			partitions2 := deployResult2.UpdatedFunctionConfig.Spec.Triggers["test-nuclio-v3io"].Partitions
//			suite.Logger.DebugWith("Second function trigger partitions", "partitions", partitions2)
//
//			// deploy a third function with the same stream trigger
//			suite.Logger.Debug("Creating third function")
//			createFunctionOptions.FunctionConfig.Meta.Name = "event_recorder_test_3"
//			suite.DeployFunction(createFunctionOptions, func(deployResult3 *platform.CreateFunctionResult) bool {
//				suite.Logger.Debug("After deploying function number 3")
//
//				// Make sure every function got 3 shards (member's shard group)
//				suite.Logger.Debug("Describing stream")
//				response, err := suite.v3ioContainer.DescribeStreamSync(&v3io.DescribeStreamInput{
//					Path: streamPath,
//					DataPlaneInput: v3io.DataPlaneInput{
//						Ctx:           context.Background(),
//						URL:           suite.url,
//						ContainerName: suite.containerName,
//						AccessKey:     suite.accessKey,
//					},
//				})
//				suite.Require().NoError(err)
//				defer response.Release()
//				describeStreamOutput := response.Output.(*v3io.DescribeStreamOutput)
//				outputShardCount := describeStreamOutput.ShardCount
//
//				suite.Logger.DebugWith("Describe stream output and shard count",
//					"describeStreamOutput", describeStreamOutput,
//					"outputShardCount", outputShardCount)
//				suite.Require().Equal(shardCount, outputShardCount)
//
//				partitions3 := deployResult3.UpdatedFunctionConfig.Spec.Triggers["test-nuclio-v3io"].Partitions
//				suite.Logger.DebugWith("Third function trigger partitions", "partitions", partitions3)
//
//				return true
//			})
//			return true
//		})
//		return true
//	})
//
//}

func (suite *testSuite) TestShardRetention() {

	var err error

	// create a stream with 4 shards
	streamPath := "dani/in-stream/"
	shardCount := 4
	consumerGroupName := "test-cg"

	err = suite.v3ioContainer.CreateStreamSync(&v3io.CreateStreamInput{
		Path:                 streamPath,
		ShardCount:           shardCount,
		RetentionPeriodHours: 1,
	})
	suite.Require().NoError(err, "Failed to create v3io sync stream")

	// create a function with 2 replicas and a v3iostream - it should only take 2 shards
	createFunctionOptions := &platform.CreateFunctionOptions{
		Logger: suite.Logger,
	}

	// parse function options from yaml
	yamlFilePath := path.Join(suite.GetNuclioSourceDir(), "hack", "env", "golang.yaml")
	yamlFile, err := ioutil.ReadFile(yamlFilePath)
	suite.Require().NoError(err)

	err = yaml.Unmarshal(yamlFile, createFunctionOptions.FunctionConfig)
	suite.Require().NoError(err)

	suite.Logger.Debug("Creating first function")
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		suite.Logger.Debug("Function deployed successfully")

		// read the state file and see the shard group taken by the function
		state, err := suite.getStateFromPersistency(streamPath, consumerGroupName)
		suite.Require().NoError(err)

		suite.Logger.DebugWith("Read state file from persistency", "state", state)

		// manually change the state file - another function is handling the same shard group

		// the function should try to retain its shard group and abort

		// read the state file and see that the function took the 2 other shards

		return true
	})

}

// GetContainerRunInfo returns nothing as Iguazio WebAPI has no Docker container
func (suite *testSuite) GetContainerRunInfo() (string, *dockerclient.RunOptions) {
	return "", nil
}

func (suite *testSuite) writingMessageToStream(streamPath string, body string) error {
	suite.Logger.InfoWith("Publishing message to stream",
		"streamPath", streamPath,
		"body", body)
	response, err := suite.v3ioContainer.PutRecordsSync(&v3io.PutRecordsInput{
		Path: streamPath,
		Records: []*v3io.StreamRecord{
			{
				Data: []byte(body),
			},
		},
	})
	suite.Require().NoError(err)

	suite.Logger.InfoWith("Successfully wrote a message to stream",
		"response", response,
		"streamPath", streamPath)
	response.Release()
	return nil
}

func (suite *testSuite) getShardLagDetails(shardID int) (int, int) {
	response, err := suite.v3ioContainer.GetItemSync(&v3io.GetItemInput{
		Path: fmt.Sprintf("%s/%d", suite.streamPath, shardID),
		AttributeNames: []string{
			"__last_sequence_num",
			fmt.Sprintf("__%s_committed_sequence_number", suite.consumerGroup),
		},
	})
	suite.Require().NoError(err)
	defer response.Release()
	getItemOutput := response.Output.(*v3io.GetItemOutput)

	current, err := getItemOutput.Item.GetFieldInt("__last_sequence_num")
	suite.Require().NoError(err)

	committed, err := getItemOutput.Item.GetFieldInt(fmt.Sprintf("__%s_committed_sequence_number",
		suite.consumerGroup))
	if err != nil {
		suite.Require().Contains(err.Error(), "Not found")
		committed = 0
	}

	return current, committed
}

func (suite *testSuite) getStateFromPersistency(streamPath, consumerGroupName string) (*v3ioscg.State, error) {
	stateContentsAttributeKey := "state"
	response, err := suite.v3ioContainer.GetItemSync(&v3io.GetItemInput{
		Path: suite.getStateFilePath(streamPath, consumerGroupName),
		AttributeNames: []string{
			stateContentsAttributeKey,
		},
	})

	if err != nil {
		errWithStatusCode, errHasStatusCode := err.(v3ioerrors.ErrorWithStatusCode)
		if !errHasStatusCode {
			return nil, errors.Wrap(err, "Got error without status code")
		}

		if errWithStatusCode.StatusCode() != 404 {
			return nil, errors.Wrap(err, "Failed getting state item")
		}

		return nil, v3ioerrors.ErrNotFound
	}

	defer response.Release()

	getItemOutput := response.Output.(*v3io.GetItemOutput)

	stateContents, err := getItemOutput.Item.GetFieldString(stateContentsAttributeKey)
	if err != nil {
		return nil, errors.Wrap(err, "Failed getting state attribute")
	}

	var state v3ioscg.State

	if err := json.Unmarshal([]byte(stateContents), &state); err != nil {
		return nil, errors.Wrapf(err, "Failed unmarshalling state contents: %s", stateContents)
	}

	return &state, nil
}

func (suite *testSuite) getStateFilePath(streamPath, consumerGroupName string) string {
	return path.Join(streamPath, fmt.Sprintf("%s-state.json", consumerGroupName))
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	testSuiteInstance := &testSuite{}
	testSuiteInstance.AbstractBrokerSuite = triggertest.NewAbstractBrokerSuite(testSuiteInstance)
	suite.Run(t, testSuiteInstance)
}
