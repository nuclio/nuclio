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

package test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"testing"
	"time"

	"github.com/Shopify/sarama"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/test/suite"
	"github.com/nuclio/nuclio/test/compare"

	"github.com/stretchr/testify/suite"
)

type testSuite struct {
	processorsuite.TestSuite
	kafkaContainerID string
	broker           *sarama.Broker
	producer         sarama.SyncProducer
}

func (suite *testSuite) SetupSuite() {
	var err error

	suite.TestSuite.SetupSuite()

	suite.Logger.Info("Starting Kafka")

	// start kafka
	suite.kafkaContainerID, err = suite.DockerClient.RunContainer("spotify/kafka",
		&dockerclient.RunOptions{
			Ports: map[int]int{2181: 2181, 9092: 9092},
			Env: map[string]string{"ADVERTISED_HOST": "172.17.0.1", "ADVERTISED_PORT": "9092"},
		})

	suite.Require().NoError(err, "Failed to start Kafka container")

	suite.waitBrokerReady()

	suite.Logger.Info("Creating broker resources")

	// create broker
	suite.broker = sarama.NewBroker("172.17.0.1:9092")

	brokerConfig := sarama.NewConfig()
	brokerConfig.Version = sarama.V0_10_1_0

	// connect to the broker
	err = suite.broker.Open(brokerConfig)
	suite.Require().NoError(err, "Failed to open broker")

	// init a create topic request
	createTopicsRequest := sarama.CreateTopicsRequest{}
	createTopicsRequest.TopicDetails = map[string]*sarama.TopicDetail{
		"test-topic": {
			NumPartitions: 1,
		},
	}

	// create topic
	_, err = suite.broker.CreateTopics(&createTopicsRequest)
	suite.Require().NoError(err, "Failed to create topic")

	// create a sync producer
	suite.producer, err = sarama.NewSyncProducer([]string{"172.17.0.1:9092"}, nil)
	suite.Require().NoError(err, "Failed to create sync producer")
}

func (suite *testSuite) TearDownSuite() {
	suite.TestSuite.TearDownTest()

	// if we weren't successful starting, nothing to do
	if suite.kafkaContainerID != "" {
		suite.DockerClient.RemoveContainer(suite.kafkaContainerID)
	}

	// set function dir
	suite.FunctionDir = path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "trigger", "kafka", "test")
}

func (suite *testSuite) TestReceiveRecords() {
	triggerConfig := functionconfig.Trigger{
		Kind: "kafka",
		URL:  "172.17.0.1:9092",
		Attributes: map[string]interface{}{
			"topic": "testtopic",
			"partitions": []int{0},
		},
	}

	createFunctionOptions := suite.getCreateFunctionOptionsWithTrigger(triggerConfig)

	suite.invokeEventRecorder(createFunctionOptions, map[string]int{
		"testtopic": 3,
	}, nil)
}

func (suite *testSuite) getCreateFunctionOptionsWithTrigger(triggerConfig functionconfig.Trigger) *platform.CreateFunctionOptions {
	createFunctionOptions := suite.GetDeployOptions("event_recorder",
		suite.GetFunctionPath(path.Join("event_recorder_python")))

	if createFunctionOptions.FunctionConfig.Spec.Triggers == nil {
		createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{}
	}

	createFunctionOptions.FunctionConfig.Spec.Triggers["my-kafka"] = triggerConfig

	return createFunctionOptions
}

func (suite *testSuite) invokeEventRecorder(createFunctionOptions *platform.CreateFunctionOptions,
	numExpectedMessagesPerTopic map[string]int,
	numNonExpectedMessagesPerTopic map[string]int) {

	// deploy functions
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		var sentEventBodies []string

		suite.Logger.DebugWith("Producing",
			"numExpectedMessagesPerTopic", numExpectedMessagesPerTopic,
				"numNonExpectedMessagesPerTopic", numNonExpectedMessagesPerTopic)

		// send messages we expect to see arrive @ the function, each to their own topic
		for topic, numMessages := range numExpectedMessagesPerTopic {
			for messageIdx := 0; messageIdx < numMessages; messageIdx++ {

				// send the message
				sentBody := suite.publishMessageToTopic(topic, messageIdx)

				// add body to bodies we expect to see in response
				sentEventBodies = append(sentEventBodies, sentBody)
			}
		}

		// send messages we *don't* expect to see arrive @ the function
		for topic, numMessages := range numNonExpectedMessagesPerTopic {
			for messageIdx := 0; messageIdx < numMessages; messageIdx++ {
				suite.publishMessageToTopic(topic, messageIdx)
			}
		}

		// TODO: retry until successful
		time.Sleep(2 * time.Second)

		suite.Logger.DebugWith("Done producing")

		baseURL := "localhost"

		// Check if situation is dockerized, if so set url to given NUCLIO_TEST_HOST
		if os.Getenv("NUCLIO_TEST_HOST") != "" {
			baseURL = os.Getenv("NUCLIO_TEST_HOST")
		}

		// Set the url for the http request
		url := fmt.Sprintf("http://%s:%d", baseURL, deployResult.Port)

		// read the events from the function
		httpResponse, err := http.Get(url)
		suite.Require().NoError(err, "Failed to read events from function: %s", url)

		marshalledResponseBody, err := ioutil.ReadAll(httpResponse.Body)
		suite.Require().NoError(err, "Failed to read response body")

		// unmarshall the body into a list
		var receivedEventBodies []string

		err = json.Unmarshal(marshalledResponseBody, &receivedEventBodies)
		suite.Require().NoError(err, "Failed to unmarshal response")

		// compare bodies
		suite.Require().True(compare.CompareNoOrder(sentEventBodies, receivedEventBodies))

		return true
	})
}

func (suite *testSuite) waitBrokerReady() {
	time.Sleep(10 * time.Second)
}

func (suite *testSuite) publishMessageToTopic(topic string, messageIdx int) string {
	producerMessage := sarama.ProducerMessage{
		Topic: topic,
		Key: sarama.StringEncoder("key"),
		Value: sarama.StringEncoder("value"),
	}

	suite.Logger.InfoWith("Producing")

	partition, offset, err := suite.producer.SendMessage(&producerMessage)
	suite.Require().NoError(err, "Failed to publish to queue")

	suite.Logger.InfoWith("Produced", "partition", partition, "offset", offset)

	return "value"
}

//
// Tests for Python
//

type pythonTestSuite struct {
	testSuite
}

func (suite *pythonTestSuite) SetupSuite() {
	suite.testSuite.SetupSuite()


}

//
// Tests for Golang
//

type golangTestSuite struct {
	testSuite
}

func (suite *golangTestSuite) SetupSuite() {
	suite.testSuite.SetupSuite()

	suite.Runtime = "golang"
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	// suite.Run(t, new(golangTestSuite))
	suite.Run(t, new(pythonTestSuite))
}
