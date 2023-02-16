//go:build test_integration && test_local

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
	"io"
	"net/http"
	"path"
	"strconv"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/test"
	"github.com/nuclio/nuclio/pkg/processor/util/partitionworker"

	"github.com/Shopify/sarama"
	"github.com/stretchr/testify/suite"
)

type testSuite struct {
	*triggertest.AbstractBrokerSuite

	httpClient *http.Client

	// kafka clients
	broker   *sarama.Broker
	producer sarama.SyncProducer

	// messaging
	topic         string
	consumerGroup string
	initialOffset string
	NumPartitions int32

	// kafka cluster
	brokerPort             int
	brokerURL              string
	brokerContainerName    string
	zooKeeperContainerName string

	// for cleanup
	zooKeeperContainerID string
}

func (suite *testSuite) SetupSuite() {
	var err error

	// create http client
	suite.httpClient = &http.Client{
		Timeout: 10 * time.Second,
	}

	// messaging
	suite.topic = "myTopic"
	suite.consumerGroup = "myConsumerGroup"
	suite.initialOffset = "earliest"
	suite.NumPartitions = 4

	// kafka cluster
	suite.brokerPort = 9092
	suite.brokerContainerName = "nuclio-kafka-broker"
	suite.zooKeeperContainerName = "nuclio-kafka-zookeeper"
	suite.brokerURL = fmt.Sprintf("%s:%d", suite.BrokerHost, suite.brokerPort)

	// start broker and zookeeper containers explicitly
	suite.AbstractBrokerSuite.SkipStartBrokerContainer = true
	suite.AbstractBrokerSuite.BrokerContainerNetworkName = "nuclio-kafka-test"
	suite.AbstractBrokerSuite.SetupSuite()

	// start zoo keeper container
	suite.zooKeeperContainerID = suite.RunContainer(suite.getKafkaZooKeeperContainerRunInfo())

	// start broker container
	suite.StartBrokerContainer(suite.GetContainerRunInfo())

	suite.Logger.InfoWith("Creating broker resources",
		"brokerHost", suite.BrokerHost)

	// create broker
	suite.broker = sarama.NewBroker(suite.brokerURL)

	brokerConfig := sarama.NewConfig()
	brokerConfig.Version = sarama.V0_11_0_2

	// change partitioner , so we can specify which partition to send on
	brokerConfig.Producer.Partitioner = sarama.NewManualPartitioner

	// connect to the broker
	err = suite.broker.Open(brokerConfig)
	suite.Require().NoError(err, "Failed to open broker")

	// create topic
	createTopicsResponse, err := suite.broker.CreateTopics(&sarama.CreateTopicsRequest{
		TopicDetails: map[string]*sarama.TopicDetail{
			suite.topic: {
				NumPartitions:     suite.NumPartitions,
				ReplicationFactor: 1,
			},
		},
	})
	suite.Require().NoError(err, "Failed to create topic")

	suite.Logger.InfoWith("Created topic",
		"topic", suite.topic,
		"createTopicResponse", createTopicsResponse)

	// create a sync producer
	suite.producer, err = sarama.NewSyncProducer([]string{suite.brokerURL}, nil)
	suite.Require().NoError(err, "Failed to create sync producer")
}

func (suite *testSuite) TearDownSuite() {
	if suite.zooKeeperContainerID != "" {
		err := suite.DockerClient.RemoveContainer(suite.zooKeeperContainerID)
		suite.NoError(err)
	}

	suite.AbstractBrokerSuite.TearDownSuite()
}

func (suite *testSuite) TestReceiveRecords() {
	functionName := "event_recorder"
	createFunctionOptions := suite.GetDeployOptions(functionName, suite.FunctionPaths["python"])
	createFunctionOptions.FunctionConfig.Spec.Platform = functionconfig.Platform{
		Attributes: map[string]interface{}{
			"network": suite.BrokerContainerNetworkName,
		},
	}

	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		"my-kafka": {
			Kind: "kafka-cluster",
			URL:  fmt.Sprintf("%s:9090", suite.brokerContainerName),
			Attributes: map[string]interface{}{
				"topics":        []string{suite.topic},
				"consumerGroup": functionName,
				"initialOffset": suite.initialOffset,
			},
			WorkerTerminationTimeout: "5s",
		},
	}

	triggertest.InvokeEventRecorder(&suite.AbstractBrokerSuite.TestSuite,
		suite.BrokerHost,
		createFunctionOptions,
		map[string]triggertest.TopicMessages{
			suite.topic: {
				NumMessages: int(suite.NumPartitions),
			},
		},
		nil,
		suite.publishMessageToTopic)
}

func (suite *testSuite) TestExplicitAck() {

	topic := "myNewTopic"
	shardID := int32(1)
	functionName := "explicitacker"
	functionPath := path.Join(suite.GetTestFunctionsDir(),
		"python",
		"kafka-explicit-ack",
		"explicitacker.py")

	// create new topic
	createTopicsResponse, err := suite.broker.CreateTopics(&sarama.CreateTopicsRequest{
		TopicDetails: map[string]*sarama.TopicDetail{
			topic: {
				NumPartitions:     suite.NumPartitions,
				ReplicationFactor: 1,
			},
		},
	})
	suite.Require().NoError(err, "Failed to create topic")

	suite.Logger.InfoWith("Created topic",
		"topic", topic,
		"createTopicResponse", createTopicsResponse)

	// create explicit ack function

	createFunctionOptions := suite.GetDeployOptions(functionName, functionPath)
	createFunctionOptions.FunctionConfig.Spec.Build.Commands = []string{"pip install nuclio-sdk"}
	createFunctionOptions.FunctionConfig.Spec.Platform = functionconfig.Platform{
		Attributes: map[string]interface{}{
			"network": suite.BrokerContainerNetworkName,
		},
	}

	// configure kafka trigger with explicit ack enabled
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		"my-kafka": {
			Kind: "kafka-cluster",
			URL:  fmt.Sprintf("%s:9090", suite.brokerContainerName),
			Attributes: map[string]interface{}{
				"topics":        []string{topic},
				"consumerGroup": functionName,
				"initialOffset": suite.initialOffset,
			},
			WorkerTerminationTimeout: "5s",
			ExplicitAckMode:          functionconfig.ExplicitAckModeEnable,
		},
		"my-http": {
			Kind:       "http",
			Attributes: map[string]interface{}{},
		},
	}

	// set worker allocation mode to static
	createFunctionOptions.FunctionConfig.Meta.Annotations = map[string]string{
		"nuclio.io/kafka-worker-allocation-mode": string(partitionworker.AllocationModeStatic),
	}

	// deploy function
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		suite.Require().NotNil(deployResult, "Unexpected empty deploy results")

		var err error

		// publish 10 messages to the topic
		for i := 0; i < 10; i++ {
			err = suite.publishMessageToTopicOnSpecificShard(topic, fmt.Sprintf("message-%d", i), shardID)
			suite.Require().NoError(err, "Failed to publish message")
		}

		// ensure queue size is 10
		suite.Logger.Debug("Getting current queue size")
		queueSize := suite.waitForFunctionQueueSize(deployResult.Port, 10, 5*time.Second)
		suite.Require().Equal(queueSize, 10, "Queue size is not 10")

		// ensure commit offset is 0
		suite.Logger.Debug("Getting commit offset before processing")
		commitOffset := suite.getLastCommitOffset(deployResult.Port)
		suite.Require().Equal(commitOffset, 0, "Commit offset is not 0")

		// send http request "start processing"
		suite.Logger.Debug("Sending start processing request")
		body := map[string]string{
			"resource": "start_processing",
		}

		marshalledBody, err := json.Marshal(body)
		suite.Require().NoError(err, "Failed to marshal body")
		response, err := suite.SendHTTPRequest(&triggertest.Request{
			Method: http.MethodPost,
			Port:   deployResult.Port,
			Body:   string(marshalledBody),
		})
		suite.Require().NoError(err, "Failed to send request")
		suite.Require().Equal(http.StatusOK, response.StatusCode)

		time.Sleep(2 * time.Second)

		// ensure queue size is 0 (or < 10)
		suite.Logger.Debug("Getting queue size after processing")
		queueSize = suite.waitForFunctionQueueSize(deployResult.Port, 0, 5*time.Second)
		suite.Require().Equal(queueSize, 0, "Queue size is not 0")

		// ensure commit offset is 9 (10 in zero-indexed offsets)
		suite.Logger.Debug("Getting commit offset after processing")
		commitOffset = suite.getLastCommitOffset(deployResult.Port)
		suite.Require().Equal(commitOffset, 9, "Commit offset is not 10")

		return true
	})
}

//func (suite *testSuite) TestEventRecorderRebalance() {
//
//	topic := "someTopic"
//
//	// create topic
//	createTopicsResponse, err := suite.broker.CreateTopics(&sarama.CreateTopicsRequest{
//		TopicDetails: map[string]*sarama.TopicDetail{
//			topic: {
//				NumPartitions:     suite.NumPartitions,
//				ReplicationFactor: 1,
//			},
//		},
//	})
//	suite.Require().NoError(err, "Failed to create topic")
//	suite.Logger.InfoWith("Created topic",
//		"topic", suite.topic,
//		"createTopicResponse", createTopicsResponse)
//
//	createFunctionOptions := suite.GetDeployOptions("event_recorder-1", suite.FunctionPaths["python"])
//	createFunctionOptions.FunctionConfig.Spec.Platform = functionconfig.Platform{
//		Attributes: map[string]interface{}{
//			"network": suite.BrokerContainerNetworkName,
//		},
//	}
//
//	initialOffset := "latest"
//
//	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
//		"my-kafka": {
//			Kind: "kafka-cluster",
//			URL:  fmt.Sprintf("%s:9090", suite.brokerContainerName),
//			Attributes: map[string]interface{}{
//				"topics":        []string{topic},
//				"consumerGroup": suite.consumerGroup,
//				"initialOffset": initialOffset,
//			},
//		},
//	}
//
//	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
//
//		var sentBodies []string
//
//		suite.Logger.DebugWith("Created first function, producing messages to topic",
//			"topic", topic)
//
//		// write messages on 4 shards
//		for partitionIdx := int32(0); partitionIdx < suite.NumPartitions; partitionIdx++ {
//			messageBody := fmt.Sprintf("%s-%d", "messagingCycleA", partitionIdx)
//
//			// send the message
//			err := suite.publishMessageToTopicOnSpecificShard(topic, messageBody, partitionIdx)
//			suite.Require().NoError(err, "Failed to publish message")
//
//			// add body to bodies we expect to see in response
//			sentBodies = append(sentBodies, messageBody)
//		}
//
//		// make sure they are all read
//		var receivedBodies []string
//		err := common.RetryUntilSuccessful(15*time.Second,
//			2*time.Second,
//			func() bool {
//				receivedBodies = suite.resolveReceivedEventBodies(deployResult)
//				return len(receivedBodies) >= int(suite.NumPartitions)
//			})
//		if err != nil {
//
//			// get container logs
//			dockerLogs, getLogsErr := suite.DockerClient.GetContainerLogs(deployResult.ContainerID)
//			suite.Require().NoError(getLogsErr, "Failed to get container logs")
//			suite.Logger.ErrorWith("At least one message was not received properly by the function",
//				"containerID", deployResult.ContainerID, "logs", dockerLogs)
//		}
//		suite.Require().NoError(err, "Failed to get events")
//		suite.Logger.DebugWith("Done producing")
//
//		suite.Logger.DebugWith("Received events from functions",
//			"event-recorder-1-events", receivedBodies)
//
//		sort.Strings(sentBodies)
//		sort.Strings(receivedBodies)
//
//		// compare bodies
//		suite.Require().Equal(sentBodies, receivedBodies)
//
//		// create another function that consumes from the same topic and consumer group
//		newCreateFunctionOptions := suite.GetDeployOptions("event_recorder-2", suite.FunctionPaths["python"])
//		newCreateFunctionOptions.FunctionConfig.Spec.Platform = functionconfig.Platform{
//			Attributes: map[string]interface{}{
//				"network": suite.BrokerContainerNetworkName,
//			},
//		}
//
//		newCreateFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
//			"my-kafka": {
//				Kind: "kafka-cluster",
//				URL:  fmt.Sprintf("%s:9090", suite.brokerContainerName),
//				Attributes: map[string]interface{}{
//					"topics":        []string{topic},
//					"consumerGroup": suite.consumerGroup,
//					"initialOffset": initialOffset,
//				},
//			},
//		}
//
//		suite.DeployFunction(newCreateFunctionOptions, func(newDeployResult *platform.CreateFunctionResult) bool {
//
//			suite.Logger.DebugWith("Created second function, producing messages to topic",
//				"topic", topic)
//
//			// write messages to all 4 shards
//			for partitionIdx := int32(0); partitionIdx < suite.NumPartitions; partitionIdx++ {
//				messageBody := fmt.Sprintf("%s-%d", "messagingCycleB", partitionIdx)
//
//				// send the message
//				err := suite.publishMessageToTopicOnSpecificShard(topic, messageBody, partitionIdx)
//				suite.Require().NoError(err, "Failed to publish message")
//
//				// add body to bodies we expect to see in response
//				sentBodies = append(sentBodies, messageBody)
//			}
//
//			// make sure they are all read
//			var receivedBodies1, receivedBodies2 []string
//			err := common.RetryUntilSuccessful(15*time.Second,
//				2*time.Second,
//				func() bool {
//					receivedBodies1 = suite.resolveReceivedEventBodies(deployResult)
//					receivedBodies2 = suite.resolveReceivedEventBodies(newDeployResult)
//
//					// make sure that new events were received in both functions
//					return len(receivedBodies1) > len(receivedBodies) && len(receivedBodies2) >= int(suite.NumPartitions)/2
//
//				})
//			if err != nil {
//
//				// get container logs
//				dockerLogs1, getLogsErr := suite.DockerClient.GetContainerLogs(deployResult.ContainerID)
//				suite.Require().NoError(getLogsErr, "Failed to get container logs")
//
//				dockerLogs2, getLogsErr := suite.DockerClient.GetContainerLogs(newDeployResult.ContainerID)
//				suite.Require().NoError(getLogsErr, "Failed to get container logs")
//
//				suite.Logger.ErrorWith("At least one message was not received properly by the functions",
//					"containerID1", deployResult.ContainerID,
//					"function1DockerLogs", dockerLogs1,
//					"containerID2", newDeployResult.ContainerID,
//					"function2DockerLogs", dockerLogs2)
//			}
//			suite.Require().NoError(err, "Failed to get events")
//			suite.Logger.DebugWith("Done producing")
//
//			// validate functions read form different shards - a rebalance occurred!
//			suite.Logger.DebugWith("Received events from functions",
//				"event-recorder-1-events", receivedBodies1,
//				"event-recorder-2-events", receivedBodies2)
//
//			for _, body := range receivedBodies1 {
//				suite.Require().False(common.StringInSlice(body, receivedBodies2))
//			}
//
//			for _, body := range receivedBodies2 {
//				suite.Require().False(common.StringInSlice(body, receivedBodies1))
//			}
//
//			return true
//		})
//
//		return true
//	})
//}

// GetContainerRunInfo returns information about the broker container
func (suite *testSuite) GetContainerRunInfo() (string, *dockerclient.RunOptions) {
	return "wurstmeister/kafka", &dockerclient.RunOptions{
		ContainerName: suite.brokerContainerName,
		Network:       suite.BrokerContainerNetworkName,
		Remove:        true,
		Ports: map[int]int{

			// broker
			suite.brokerPort: suite.brokerPort,
		},
		Env: map[string]string{
			"KAFKA_ZOOKEEPER_CONNECT":              fmt.Sprintf("%s:2181", suite.zooKeeperContainerName),
			"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP": "INTERNAL:PLAINTEXT,EXTERNAL:PLAINTEXT",
			"KAFKA_INTER_BROKER_LISTENER_NAME":     "INTERNAL",
			"KAFKA_LISTENERS": fmt.Sprintf("INTERNAL://:9090,EXTERNAL://:%d",
				suite.brokerPort),
			"KAFKA_ADVERTISED_LISTENERS": fmt.Sprintf(
				"INTERNAL://%s:9090,EXTERNAL://%s:%d",
				suite.brokerContainerName,
				suite.BrokerHost,
				suite.brokerPort,
			),
		},
	}
}

func (suite *testSuite) getKafkaZooKeeperContainerRunInfo() (string, *dockerclient.RunOptions) {
	return "wurstmeister/zookeeper", &dockerclient.RunOptions{
		ContainerName: suite.zooKeeperContainerName,
		Network:       suite.BrokerContainerNetworkName,
		Remove:        true,
		Ports: map[int]int{
			dockerclient.RunOptionsNoPort: 2181,
		},
	}
}

func (suite *testSuite) publishMessageToTopic(topic string, body string) error {
	return suite.publishMessageToTopicOnSpecificShard(topic, body, 0)
}

func (suite *testSuite) publishMessageToTopicOnSpecificShard(topic string, body string, partitionID int32) error {
	producerMessage := sarama.ProducerMessage{
		Topic:     topic,
		Key:       sarama.StringEncoder(fmt.Sprintf("key-%d", partitionID)),
		Value:     sarama.StringEncoder(body),
		Partition: partitionID,
	}

	suite.Logger.InfoWith("Producing", "topic", topic, "body", body)

	partition, offset, err := suite.producer.SendMessage(&producerMessage)
	suite.Require().NoError(err, "Failed to publish to queue")

	suite.Logger.InfoWith("Produced", "partition", partition, "offset", offset)

	return nil
}

func (suite *testSuite) resolveReceivedEventBodies(deployResult *platform.CreateFunctionResult) []string {

	receivedEvents, err := triggertest.GetEventRecorderReceivedEvents(suite.Logger, suite.BrokerHost, deployResult.Port)
	suite.Require().NoError(err)
	var receivedBodies []string

	// compare only bodies due to a deficiency in CompareNoOrder
	for _, receivedEvent := range receivedEvents {

		// some brokers need data to be able to read the stream. these write "ignore", so we ignore that
		if receivedEvent.Body != "ignore" {
			receivedBodies = append(receivedBodies, receivedEvent.Body)
		}
	}

	return receivedBodies
}

func (suite *testSuite) getLastCommitOffset(port int) int {
	body := map[string]string{
		"resource": "last_committed_offset",
	}

	marshalledBody, err := json.Marshal(body)
	suite.Require().NoError(err, "Failed to marshal body")

	httpRequest := &triggertest.Request{
		Method: http.MethodGet,
		Port:   port,
		Body:   string(marshalledBody),
	}
	response, err := suite.SendHTTPRequest(httpRequest)
	suite.Require().NoError(err, "Failed to send request")
	suite.Require().Equal(http.StatusOK, response.StatusCode)

	responseBodyBytes, err := io.ReadAll(response.Body)
	suite.Require().NoError(err, "Failed to read response body")
	suite.Logger.DebugWith("Got response", "response", response, "responseBody", string(responseBodyBytes))

	responseBody := map[string]interface{}{}
	err = json.Unmarshal(responseBodyBytes, &responseBody)
	suite.Require().NoError(err, "Failed to unmarshal response body")

	lastCommittedOffset, exists := responseBody["last_committed_offset"]
	suite.Require().True(exists, "Failed to find last committed offset")

	lastCommittedOffsetString, ok := lastCommittedOffset.(string)
	suite.Require().True(ok, "Failed to convert last committed offset to string")

	lastCommittedOffsetInt, err := strconv.Atoi(lastCommittedOffsetString)
	suite.Require().NoError(err, "Failed to convert last committed offset to int")

	return lastCommittedOffsetInt
}

func (suite *testSuite) getQueueSize(port int) int {
	body := map[string]string{
		"resource": "queue_size",
	}

	marshalledBody, err := json.Marshal(body)
	suite.Require().NoError(err, "Failed to marshal body")

	httpRequest := &triggertest.Request{
		Method: http.MethodGet,
		Port:   port,
		Body:   string(marshalledBody),
	}
	response, err := suite.SendHTTPRequest(httpRequest)
	suite.Require().NoError(err, "Failed to send request")
	suite.Require().Equal(http.StatusOK, response.StatusCode)

	responseBodyBytes, err := io.ReadAll(response.Body)
	suite.Require().NoError(err, "Failed to read response body")
	suite.Logger.DebugWith("Got response", "response", response, "responseBody", string(responseBodyBytes))

	responseBody := map[string]interface{}{}
	err = json.Unmarshal(responseBodyBytes, &responseBody)
	suite.Require().NoError(err, "Failed to unmarshal response body")

	queueSize, exists := responseBody["queue_size"]
	suite.Require().True(exists, "Failed to find queue size")

	queueSizeFloat, ok := queueSize.(float64)
	suite.Require().True(ok, "Failed to convert queue size to int")

	return int(queueSizeFloat)
}

func (suite *testSuite) waitForFunctionQueueSize(port, expectedQueueSize int, timeout time.Duration) int {
	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()

	for {
		select {
		case <-timeoutTimer.C:
			suite.Fail("Timeout waiting for queue size")
		default:
			queueSize := suite.getQueueSize(port)
			if queueSize == expectedQueueSize {
				return queueSize
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	testSuiteInstance := &testSuite{}
	testSuiteInstance.AbstractBrokerSuite = triggertest.NewAbstractBrokerSuite(testSuiteInstance)
	suite.Run(t, testSuiteInstance)
}
