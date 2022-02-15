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
	"path"
	"reflect"
	"strings"
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
)

type testSuite struct {
	*triggertest.AbstractBrokerSuite

	// kafka clients
	v3ioContext   v3io.Context
	v3ioContainer v3io.Container
	v3ioSession   v3io.Session
	numWorkers    int

	// v3io trigger attributes
	url           string
	accessKey     string
	containerName string
	streamPath    string
	consumerGroup string

	stateContentsAttributeKey string
}

func (suite *testSuite) SetupSuite() {
	var err error

	// Change below
	suite.url = "https://webapi.default-tenant.app.dev62.lab.iguazeng.com" // "https://somewhere:8444"
	suite.accessKey = "9aa84507-49ff-416f-bbe9-6a43161edb2f"               // "some-access-key"
	// END OF change

	suite.numWorkers = 8
	suite.stateContentsAttributeKey = "state"

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

	//err = suite.v3ioContainer.CreateStreamSync(&v3io.CreateStreamInput{
	//	Path:                 suite.streamPath,
	//	ShardCount:           suite.shardCount,
	//	RetentionPeriodHours: 1,
	//})
	//suite.Require().NoError(err, "Failed to create v3io sync stream")
}

func (suite *testSuite) TearDownSuite() {
	suite.Logger.DebugWith("Deleting stream sync", "streamPath", suite.streamPath)
	//err := suite.v3ioContainer.DeleteStreamSync(&v3io.DeleteStreamInput{
	//	Path: suite.streamPath,
	//})
	//suite.Require().NoError(err)
	suite.AbstractBrokerSuite.TearDownSuite()
}

func (suite *testSuite) TestAckWindowSize() {
	ackWindowSize := 10
	shardID := 0
	recordedEvents := 0
	shardCount := 1

	err := suite.v3ioContainer.CreateStreamSync(&v3io.CreateStreamInput{
		Path:                 suite.streamPath,
		ShardCount:           shardCount,
		RetentionPeriodHours: 1,
	})
	suite.Require().NoError(err, "Failed to create v3io sync stream")

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

	shardCount := 1

	err := suite.v3ioContainer.CreateStreamSync(&v3io.CreateStreamInput{
		Path:                 suite.streamPath,
		ShardCount:           shardCount,
		RetentionPeriodHours: 1,
	})
	suite.Require().NoError(err, "Failed to create v3io sync stream")

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

func (suite *testSuite) TestShardRetention() {

	var err error
	hijackMemberName := "another-replica-id"
	shardCount := 4
	//streamPath := "test-stream/path/"
	numReplicas := 2

	// create a stream with 4 shards
	err = suite.v3ioContainer.CreateStreamSync(&v3io.CreateStreamInput{
		Path:                 suite.streamPath,
		ShardCount:           shardCount,
		RetentionPeriodHours: 1,
	})
	suite.Require().NoError(err, "Failed to create v3io sync stream")

	// create a function with 2 replicas and a v3iostream - it should only take 2 shards
	createFunctionOptions := suite.GetDeployOptions("event_recorder", suite.FunctionPaths["python"])
	createFunctionOptions.FunctionConfig.Spec.Replicas = &numReplicas
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		"test-nuclio-v3io": {
			Kind:       "v3ioStream",
			URL:        suite.url,
			Password:   suite.accessKey,
			MaxWorkers: 1,
			Attributes: map[string]interface{}{
				"seekTo":        "earliest",
				"containerName": suite.containerName,
				"streamPath":    suite.streamPath,
				"consumerGroup": suite.consumerGroup,
			},
		},
	}

	// deploy function
	suite.Logger.Debug("Deploying function")
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		suite.Logger.Debug("Function deployed successfully")

		var state *v3ioscg.State

		// read the state file and see the shard group taken by the function
		err = common.RetryUntilSuccessful(10*time.Second, 1*time.Second, func() bool {

			state, err = suite.getStateFromPersistency(suite.streamPath, suite.consumerGroup)
			if err != nil {
				suite.Logger.WarnWith("Failed getting state from persistency", "err", err.Error())
				return false
			}

			// wait until the function grabs a shard group
			if len(state.SessionStates) == 0 {
				return false
			}

			return true
		})
		suite.Require().NoError(err)

		suite.Logger.DebugWith("Read state file from persistency", "state", state)

		// there should only be 1 session state
		suite.Require().Equal(1, len(state.SessionStates))

		// keep the originally assigned shards
		originalShards := state.SessionStates[0].Shards
		suite.Require().NotEmpty(originalShards)

		// write to shard #0
		shardID := 0
		messageBody := fmt.Sprintf("%s-%d", suite.streamPath, shardID)
		err = suite.writeMessageToStreamShard(suite.streamPath, messageBody, &shardID)
		suite.Require().NoError(err, "Failed to publish message")

		time.Sleep(2 * time.Second)

		// make sure there is no lag
		current, committed := suite.getShardLagDetails(shardID)
		suite.Logger.DebugWith("Got shard lag details",
			"shardID", shardID,
			"current", current,
			"committed", committed)
		suite.Require().Equal(current, committed)

		// manually change the state file - another function hijacks the same shard group
		suite.Logger.Debug("Modifying state file")
		state.SessionStates[0].MemberID = hijackMemberName
		err = suite.setStateInPersistency(suite.streamPath, suite.consumerGroup, state)
		suite.Require().NoError(err)

		// update heart beat regularly in a goroutine
		stopChan := make(chan struct{}, 1)
		defer func() {
			stopChan <- struct{}{}
		}()

		go func() {
			for {
				select {
				case <-time.After(1 * time.Second):
					err := suite.updateHeartBeat(suite.streamPath, suite.consumerGroup, hijackMemberName)
					suite.Require().NoError(err)

				case <-stopChan:
					return
				}
			}
		}()

		// the function should try to retain its shard group and abort
		time.Sleep(time.Second) // not sure if needed

		// read the state file again and see that the function took the other shard group
		err = common.RetryUntilSuccessful(10*time.Second, 1*time.Second, func() bool {

			state, err = suite.getStateFromPersistency(suite.streamPath, suite.consumerGroup)
			if err != nil {
				suite.Logger.WarnWith("Failed getting state from persistency", "err", err.Error())
				return false
			}

			// wait until the function grabs the other shard group
			if len(state.SessionStates) == 1 {
				return false
			}
			return true
		})

		suite.Logger.DebugWith("Read state file from persistency #2", "state", state)
		suite.Require().Equal(2, len(state.SessionStates))

		var updatedShards []int
		for _, sessionState := range state.SessionStates {
			if sessionState.MemberID != "another-replica-id" {
				updatedShards = sessionState.Shards
			}
		}
		suite.Require().NotEmpty(updatedShards)

		suite.Require().False(reflect.DeepEqual(updatedShards, originalShards))

		// after hijack - write to shard #0 again
		err = suite.writeMessageToStreamShard(suite.streamPath, messageBody, &shardID)
		suite.Require().NoError(err, "Failed to publish message")

		time.Sleep(2 * time.Second)

		// expect a lag of 1 for shard #0
		current, committed = suite.getShardLagDetails(shardID) // TOMER - {"current": 2, "committed": 2} - not as expected
		suite.Logger.DebugWith("Got shard lag details",
			"shardID", shardID,
			"current", current,
			"committed", committed)
		//suite.Require().Equal(1, current-committed)

		// after hijack - write to shard #2
		shardID = 2
		messageBody = fmt.Sprintf("%s-%d", suite.streamPath, shardID)
		err = suite.writeMessageToStreamShard(suite.streamPath, messageBody, &shardID)
		suite.Require().NoError(err, "Failed to publish message")

		time.Sleep(2 * time.Second)

		// expect no lag and committed > 0 for shard #2
		current, committed = suite.getShardLagDetails(shardID) // TOMER - {"current": 1, "committed": 0} - not as expected
		suite.Logger.DebugWith("Got shard lag details",
			"shardID", shardID,
			"current", current,
			"committed", committed)
		suite.Require().Equal(current, committed)
		suite.Require().Greater(committed, 0)

		return true
	})
}

func (suite *testSuite) TestManualShardHijacking() {

	suite.Logger.Debug("Hello!")
	var (
		err   error
		state *v3ioscg.State
	)

	streamPath := "/test-stream-6/"
	//shardCount := 4
	consumerGroup := "cg0"
	memberName := "another-replica-id"

	suite.v3ioContainer, err = suite.v3ioSession.NewContainer(&v3io.NewContainerInput{
		ContainerName: "users",
	})
	suite.Require().NoError(err, "Failed to create v3io container")

	err = common.RetryUntilSuccessful(10*time.Second, 1*time.Second, func() bool {

		state, err = suite.getStateFromPersistency(streamPath, consumerGroup)
		if err != nil {
			suite.Logger.WarnWith("Failed getting state from persistency", "err", err.Error())
			return false
		}

		// wait until the function grabs the other shard group
		//if len(state.SessionStates) == 1 {
		//	return false
		//}
		return true
	})

	suite.Require().NoError(err)

	suite.Logger.DebugWith("Read state file from persistency", "state", state)

	// change memberID - to nothing
	suite.Logger.Debug("Modifying state file")
	for i, session := range state.SessionStates {
		if !strings.Contains(session.MemberID, "stream-test") {
			state.SessionStates[i].MemberID = memberName
		}
	}
	//state.SessionStates[0].MemberID = memberName
	err = suite.setStateInPersistency(streamPath, consumerGroup, state)
	suite.Require().NoError(err)

	// update heart beat regularly in a goroutine
	stopChan := make(chan struct{}, 1)
	defer func() {
		stopChan <- struct{}{}
	}()

	go func() {
		for {
			select {
			case <-time.After(1 * time.Second):
				suite.Logger.DebugWith("Updating heartbeat")
				err := suite.updateHeartBeat(streamPath, consumerGroup, memberName)
				if err != nil {
					suite.Logger.WarnWith("Failed to update heartbeat", "err", err.Error())
				}

			case <-stopChan:
				return
			}
		}
	}()

	// make sure the original replica took the other shards
	err = common.RetryUntilSuccessful(10*time.Second, 1*time.Second, func() bool {

		state, err = suite.getStateFromPersistency(streamPath, consumerGroup)
		if err != nil {
			suite.Logger.WarnWith("Failed getting state from persistency", "err", err.Error())
			return false
		}

		// wait until the function grabs the other shard group
		if len(state.SessionStates) == 2 {
			return false
		}
		return true
	})

	suite.Logger.DebugWith("Read state file from persistency Again", "state", state)

	time.Sleep(time.Minute * 120)
	suite.consumerGroup = consumerGroup
	suite.streamPath = streamPath

	shardID := 0
	current, committed := suite.getShardLagDetails(shardID)
	suite.Logger.DebugWith("Got shard lag details",
		"shardID", shardID,
		"current", current,
		"committed", committed)

	shardID = 17
	current, committed = suite.getShardLagDetails(shardID)
	suite.Logger.DebugWith("Got shard lag details",
		"shardID", shardID,
		"current", current,
		"committed", committed)

}

func (suite *testSuite) updateHeartBeat(streamPath, consumerGroup, memberName string) error {
	var (
		err   error
		state *v3ioscg.State
	)

	err = common.RetryUntilSuccessful(10*time.Second, 1*time.Second, func() bool {

		state, err = suite.getStateFromPersistency(streamPath, consumerGroup)
		if err != nil {
			suite.Logger.WarnWith("Failed getting state from persistency", "err", err.Error())
			return false
		}

		// wait until the function grabs the other shard group
		if len(state.SessionStates) == 1 {
			return false
		}
		return true
	})
	if err != nil {
		return err
	}

	// find our session by member ID
	var sessionState *v3ioscg.SessionState

	for _, sessState := range state.SessionStates {
		if sessState.MemberID == memberName {
			sessionState = sessState
		}
	}

	// session already exists - just set the last heartbeat
	if sessionState != nil {
		sessionState.LastHeartbeat = time.Now()
	}

	return suite.setStateInPersistency(streamPath, consumerGroup, state)
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

func (suite *testSuite) writeMessageToStreamShard(streamPath string, body string, shardID *int) error {
	suite.Logger.InfoWith("Publishing message to stream shard",
		"streamPath", streamPath,
		"body", body,
		"shardID", shardID)
	response, err := suite.v3ioContainer.PutRecordsSync(&v3io.PutRecordsInput{
		Path: streamPath,
		Records: []*v3io.StreamRecord{
			{
				ShardID: shardID,
				Data:    []byte(body),
			},
		},
	})
	suite.Require().NoError(err)

	suite.Logger.InfoWith("Successfully wrote a message to stream",
		"response", response,
		"streamPath", streamPath,
		"shardID", shardID)
	response.Release()
	return nil
}

func (suite *testSuite) getShardLagDetails(shardID int) (int, int) {
	response, err := suite.v3ioContainer.GetItemSync(&v3io.GetItemInput{
		Path: fmt.Sprintf("%s%d", suite.streamPath, shardID),
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

	response, err := suite.v3ioContainer.GetItemSync(&v3io.GetItemInput{
		Path: suite.getStateFilePath(streamPath, consumerGroupName),
		AttributeNames: []string{
			suite.stateContentsAttributeKey,
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

	stateContents, err := getItemOutput.Item.GetFieldString(suite.stateContentsAttributeKey)
	if err != nil {
		return nil, errors.Wrap(err, "Failed getting state attribute")
	}

	var state v3ioscg.State

	if err := json.Unmarshal([]byte(stateContents), &state); err != nil {
		return nil, errors.Wrapf(err, "Failed unmarshalling state contents: %s", stateContents)
	}

	return &state, nil
}

func (suite *testSuite) setStateInPersistency(streamPath, consumerGroupName string, state *v3ioscg.State) error {
	stateContents, err := json.Marshal(state)
	if err != nil {
		return errors.Wrap(err, "Failed marshaling state file contents")
	}

	if _, err := suite.v3ioContainer.UpdateItemSync(&v3io.UpdateItemInput{
		Path: suite.getStateFilePath(streamPath, consumerGroupName),
		Attributes: map[string]interface{}{
			suite.stateContentsAttributeKey: string(stateContents),
		},
	}); err != nil {
		return errors.Wrap(err, "Failed setting state in persistency")
	}

	return nil
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
