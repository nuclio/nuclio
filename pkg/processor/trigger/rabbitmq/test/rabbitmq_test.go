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

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/test/suite"
	"github.com/nuclio/nuclio/test/compare"
	"github.com/rs/xid"

	"github.com/streadway/amqp"
	"github.com/stretchr/testify/suite"
)

const (
	triggerName = "test_rmq"
)

type testSuite struct {
	processorsuite.TestSuite
	rabbitmqContainerID string
	brokerConn          *amqp.Connection
	brokerChannel       *amqp.Channel
	brokerQueue         amqp.Queue
	brokerPort          int
	brokerExchangeName  string
	brokerQueueName     string
	brokerURL           string
}

func (suite *testSuite) SetupSuite() {
	var err error

	suite.TestSuite.SetupSuite()

	suite.brokerPort = 5672
	suite.brokerExchangeName = "nuclio.rabbitmq_trigger_test"

	baseURL := "localhost"

	// Check if dockerized, if so set url to given NUCLIO_TEST_HOST
	if os.Getenv("NUCLIO_TEST_HOST") != "" {
		baseURL = os.Getenv("NUCLIO_TEST_HOST")
	}

	// Set suite's brokerURL
	suite.brokerURL = fmt.Sprintf("amqp://%s:%d", baseURL, suite.brokerPort)

	// start rabbit mq
	suite.rabbitmqContainerID, err = suite.DockerClient.RunContainer("rabbitmq:3.6-alpine",
		&dockerclient.RunOptions{
			Ports: map[int]int{suite.brokerPort: suite.brokerPort, 15671: 15671},
		})

	suite.Require().NoError(err, "Failed to start RabbitMQ container")

	suite.waitBrokerReady()
}

func (suite *testSuite) TearDownSuite() {
	suite.TestSuite.TearDownTest()

	// if we weren't successful starting, nothing to do
	if suite.rabbitmqContainerID != "" {
		suite.DockerClient.RemoveContainer(suite.rabbitmqContainerID)
	}
}

func (suite *testSuite) SetupTest() {
	suite.TestSuite.SetupTest()

	// set function dir
	suite.FunctionDir = path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "trigger", "rabbitmq", "test")
}

func (suite *testSuite) TearDownTest() {
	suite.TestSuite.TearDownTest()

	// delete broker stuff
	suite.deleteBrokerResources(suite.brokerURL, suite.brokerExchangeName, suite.brokerQueueName)
}

func (suite *testSuite) TestPreexistingResources() {
	suite.brokerQueueName = "test-queue-" + xid.New().String()

	// Create a queue and bind it to all topics
	triggerConfig := suite.createBrokerResources(suite.brokerURL,
		suite.brokerExchangeName,
		suite.brokerQueueName,
		[]string{"*"})

	createFunctionOptions := suite.getCreateFunctionOptionsWithRmqTrigger(triggerConfig)

	suite.invokeEventRecorder(createFunctionOptions, map[string]int{
		"t1": 3,
		"t2": 3,
		"t3": 3,
	}, nil)
}

func (suite *testSuite) TestResourcesCreatedByFunction() {
	suite.brokerQueueName = "test-queue-" + xid.New().String()

	// Declare an exchange, but don't create a queue
	triggerConfig := suite.createBrokerResources(suite.brokerURL,
		suite.brokerExchangeName,
		"",
		[]string{"t1", "t2", "t3"})

	createFunctionOptions := suite.getCreateFunctionOptionsWithRmqTrigger(triggerConfig)

	suite.invokeEventRecorder(createFunctionOptions,
		map[string]int{
			"t1": 3,
			"t2": 3,
			"t3": 3,
		},
		map[string]int{
			"t4": 3,
			"t5": 3,
		},
	)
}

func (suite *testSuite) getCreateFunctionOptionsWithRmqTrigger(triggerConfig functionconfig.Trigger) *platform.CreateFunctionOptions {
	createFunctionOptions := suite.getCreateFunctionOptionsForRuntime(suite.Runtime)

	if createFunctionOptions.FunctionConfig.Spec.Triggers == nil {
		createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{}
	}

	createFunctionOptions.FunctionConfig.Spec.Triggers[triggerName] = triggerConfig

	return createFunctionOptions
}

func (suite *testSuite) getCreateFunctionOptionsForRuntime(runtime string) *platform.CreateFunctionOptions {
	var createFunctionOptions *platform.CreateFunctionOptions

	switch runtime {
	case "python":
		createFunctionOptions = suite.GetDeployOptions("event_recorder",
			suite.GetFunctionPath(path.Join("event_recorder_python")))
	case "golang":
		functionPath := path.Join("_event_recorder_golang", "event_recorder.go")
		createFunctionOptions = suite.GetDeployOptions("event_recorder",
			suite.GetFunctionPath(functionPath))
	default:
		suite.Failf("Unrecognized runtime name: %s", runtime)
	}

	return createFunctionOptions
}

func (suite *testSuite) getDefaultRmqTriggerConfig() functionconfig.Trigger {
	return functionconfig.Trigger{
		Kind: "rabbit-mq",
		URL:  fmt.Sprintf("amqp://guest:guest@172.17.0.1:%d", suite.brokerPort),
		Attributes: map[string]interface{}{
			"exchangeName": suite.brokerExchangeName,
		},
	}
}

func (suite *testSuite) invokeEventRecorder(createFunctionOptions *platform.CreateFunctionOptions,
	numExpectedMessagesPerTopic map[string]int,
	numNonExpectedMessagesPerTopic map[string]int) {

	// deploy functions
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		var sentEventBodies []string

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

func (suite *testSuite) createBrokerResources(brokerURL string,
	brokerExchangeName string,
	queueName string,
	topics []string) functionconfig.Trigger {

	var err error

	suite.brokerConn, err = amqp.Dial(brokerURL)
	suite.Require().NoError(err, "Failed to dial to broker")

	suite.brokerChannel, err = suite.brokerConn.Channel()
	suite.Require().NoError(err, "Failed to create broker channel")

	// clear stuff before we create stuff
	suite.deleteBrokerResources(suite.brokerURL, suite.brokerExchangeName, suite.brokerQueueName)

	// create the exchange
	suite.brokerChannel.ExchangeDeclare(brokerExchangeName,
		"topic",
		false,
		false,
		false,
		false,
		nil)

	// declare a queue and bind it, if a queue set
	if queueName != "" {

		suite.brokerQueue, err = suite.brokerChannel.QueueDeclare(
			queueName,
			false,
			false,
			false,
			false,
			nil)

		suite.Require().NoError(err, "Failed to declare queue")

		for _, topic := range topics {
			err = suite.brokerChannel.QueueBind(
				suite.brokerQueue.Name,
				topic,
				brokerExchangeName,
				false,
				nil)

			suite.Require().NoError(err, "Failed to bind queue")
		}
	}

	// create a trigger configuration where the queue name is specified
	triggerConfig := suite.getDefaultRmqTriggerConfig()
	triggerConfig.Attributes["queueName"] = queueName
	triggerConfig.Attributes["topics"] = topics

	return triggerConfig
}

func (suite *testSuite) deleteBrokerResources(brokerURL string, brokerExchangeName string, queueName string) {

	// delete the queue in case it exists
	suite.brokerChannel.QueueDelete(queueName, false, false, false)

	// delete the exchange
	suite.brokerChannel.ExchangeDelete(brokerExchangeName, false, false)
}

func (suite *testSuite) waitBrokerReady() {
	err := common.RetryUntilSuccessful(30*time.Second, 1*time.Second, func() bool {

		// try to connect
		conn, err := amqp.Dial(suite.brokerURL)
		if err != nil {
			return false
		}

		conn.Close()
		return true
	})

	suite.Require().NoError(err, "Failed to connect to RabbitMQ in given timeframe")
}

func (suite *testSuite) publishMessageToTopic(topic string, messageIdx int) string {
	amqpMessage := amqp.Publishing{}
	amqpMessage.ContentType = "application/text"
	amqpMessage.Body = []byte(fmt.Sprintf("%s-%d", topic, messageIdx))

	// publish the message
	err := suite.brokerChannel.Publish(suite.brokerExchangeName,
		topic,
		false,
		false,
		amqpMessage)

	suite.Require().NoError(err, "Failed to publish to queue")

	return string(amqpMessage.Body)
}

//
// Tests for Python
//

type pythonTestSuite struct {
	testSuite
}

func (suite *pythonTestSuite) SetupSuite() {
	suite.testSuite.SetupSuite()

	suite.Runtime = "python"
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

	suite.Run(t, new(golangTestSuite))
	suite.Run(t, new(pythonTestSuite))
}
