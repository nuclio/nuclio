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
	"fmt"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/test"

	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
	"github.com/v3io/v3io-go/pkg/dataplane"
	v3iohttp "github.com/v3io/v3io-go/pkg/dataplane/http"
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
	suite.url = "https://somewhere:8444"
	suite.accessKey = "some-access-key"
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

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	testSuiteInstance := &testSuite{}
	testSuiteInstance.AbstractBrokerSuite = triggertest.NewAbstractBrokerSuite(testSuiteInstance)
	suite.Run(t, testSuiteInstance)
}
