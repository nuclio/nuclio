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

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/test/suite"

	"github.com/streadway/amqp"
	"github.com/stretchr/testify/suite"
)

const (
	brokerExchangeName = "nuclio.rabbitmq_trigger_test"
	brokerQueueName    = "test_queue"
	brokerPort         = 5672
	triggerName        = "test_rmq"
)

type TestSuite struct {
	processorsuite.TestSuite
	rabbitmqContainerID string
	brokerConn          *amqp.Connection
	brokerChannel       *amqp.Channel
	brokerQueue         amqp.Queue
	brokerPort          int
	brokerExchangeName  string
	brokerQueueName     string
	brokerURL           string
	expectedResponses   []map[string]string
}

func (suite *TestSuite) SetupSuite() {
	var err error

	suite.TestSuite.SetupSuite()

	suite.brokerPort = brokerPort
	suite.brokerExchangeName = brokerExchangeName
	suite.brokerQueueName = brokerQueueName

	baseURL := "localhost"

	// Check if situation is dockerized, if so set url to given NUCLIO_TEST_HOST
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

func (suite *TestSuite) TearDownSuite() {
	suite.TestSuite.TearDownTest()

	// if we weren't successful starting, nothing to do
	if suite.rabbitmqContainerID != "" {
		suite.DockerClient.RemoveContainer(suite.rabbitmqContainerID)
	}
}

func (suite *TestSuite) SetupTest() {
	suite.TestSuite.SetupTest()

	suite.FunctionDir = suite.getFunctionsPath()

	suite.expectedResponses = []map[string]string{
		{"request": "0"},
		{"request": "1"},
		{"request": "2"},
	}
}

func (suite *TestSuite) TearDownTest() {
	suite.TestSuite.TearDownTest()

	// delete broker stuff
	suite.deleteBrokerResources(suite.brokerURL, suite.brokerExchangeName, suite.brokerQueueName)
}

func (suite *TestSuite) TestPostEventPython() {

	// create broker stuff
	suite.createBrokerResources(suite.brokerURL, suite.brokerExchangeName, suite.brokerQueueName, nil)

	suite.Runtime = "python"

	deployOptions := suite.getRmqDeployOptions()
	messages := suite.getTestMessages(3)

	suite.invokeEventRecorder(deployOptions, messages)
}

func (suite *TestSuite) TestPostEventGolang() {

	// create broker stuff
	suite.createBrokerResources(suite.brokerURL, suite.brokerExchangeName, suite.brokerQueueName, nil)

	suite.Runtime = "golang"

	deployOptions := suite.getRmqDeployOptions()
	messages := suite.getTestMessages(3)

	suite.invokeEventRecorder(deployOptions, messages)
}

func (suite *TestSuite) TestPostEventSubscribeToSingleTopic() {

	// create broker stuff
	suite.createBrokerResources(suite.brokerURL, suite.brokerExchangeName, suite.brokerQueueName, []string{"t1"})

	suite.Runtime = "golang"

	deployOptions := suite.getRmqDeployOptions()
	deployOptions.FunctionConfig.Spec.Triggers[triggerName].Attributes["topics"] = []string{"t1"}

	messages := suite.getTestMessages(3)
	for i := range messages {
		messages[i].Topic = fmt.Sprintf("t%d", i)
	}

	suite.expectedResponses = []map[string]string{
		{"request": "1"},
	}

	suite.invokeEventRecorder(deployOptions, messages)
}

func (suite *TestSuite) TestPostEventSubscribeToMultipleTopics() {

	// create broker stuff
	suite.createBrokerResources(suite.brokerURL, suite.brokerExchangeName, suite.brokerQueueName, []string{"t1", "t2"})

	suite.Runtime = "golang"

	deployOptions := suite.getRmqDeployOptions()
	deployOptions.FunctionConfig.Spec.Triggers[triggerName].Attributes["topics"] = []string{"t1", "t2"}

	messages := suite.getTestMessages(5)
	for i := range messages {
		messages[i].Topic = fmt.Sprintf("t%d", i)
	}

	suite.expectedResponses = []map[string]string{
		{"request": "1"},
		{"request": "2"},
	}

	suite.invokeEventRecorder(deployOptions, messages)

}

func (suite *TestSuite) getRmqDeployOptions() *platform.DeployOptions {
	deployOptions := suite.getDeployOptionsForRuntime(suite.Runtime)

	if deployOptions.FunctionConfig.Spec.Triggers == nil {
		deployOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
			triggerName: suite.getRmqTriggerConfig(),
		}
	} else {
		deployOptions.FunctionConfig.Spec.Triggers[triggerName] = suite.getRmqTriggerConfig()
	}

	return deployOptions
}

func (suite *TestSuite) getDeployOptionsForRuntime(runtime string) *platform.DeployOptions {
	var deployOptions *platform.DeployOptions

	switch runtime {
	case "python":
		deployOptions = suite.GetDeployOptions("event_recorder",
			suite.GetFunctionPath(path.Join("event_recorder_python")))
	case "golang":
		functionPath := path.Join("_event_recorder_golang", "event_recorder.go")
		deployOptions = suite.GetDeployOptions("event_recorder",
			suite.GetFunctionPath(functionPath))
	default:
		suite.Failf("Unrecognized runtime name: %s", runtime)
	}
	return deployOptions
}

func (suite *TestSuite) getRmqTriggerConfig() functionconfig.Trigger {
	return functionconfig.Trigger{
		Kind: "rabbit-mq",
		URL:  fmt.Sprintf("amqp://guest:guest@172.17.0.1:%d", brokerPort),
		Attributes: map[string]interface{}{
			"exchangeName": brokerExchangeName,
			"queueName":    brokerQueueName,
			"topics":       []string{"*"},
		},
	}
}

func (suite *TestSuite) getTestMessages(amount int) []TestMessage {
	messages := make([]TestMessage, amount)

	for i := 0; i < amount; i++ {
		newMessage := TestMessage{
			amqp.Publishing{
			},
			"t1",
		}

		newMessage.Message.ContentType = "application/json"
		newMessage.Message.Body = []byte(fmt.Sprintf(`{"request": "%d"}`, i))

		messages[i] = newMessage
	}

	return messages
}

func (suite *TestSuite) invokeEventRecorder(deployOptions *platform.DeployOptions, messages []TestMessage) {
	suite.DeployFunction(deployOptions, func(deployResult *platform.DeployResult) bool {

		// send messages
		for _, message := range messages {

			// publish the message
			err := suite.brokerChannel.Publish(suite.brokerExchangeName,
				message.Topic,
				false,
				false,
				message.Message)

			suite.Require().NoError(err, "Failed to publish to queue")
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
		var receivedEvents []map[string]string

		err = json.Unmarshal(marshalledResponseBody, &receivedEvents)
		suite.Require().NoError(err, "Failed to unmarshal response")

		// must have received 3 events
		suite.Require().Equal(suite.expectedResponses, receivedEvents)

		return true
	})
}

func (suite *TestSuite) createBrokerResources(brokerURL string, brokerExchangeName string, queueName string, topics []string) {
	var err error

	if len(topics) == 0 {
		topics = []string{"*"}
	}

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

func (suite *TestSuite) deleteBrokerResources(brokerURL string, brokerExchangeName string, queueName string) {

	// delete the queue in case it exists
	suite.brokerChannel.QueueDelete(queueName, false, false, false)

	// delete the exchange
	suite.brokerChannel.ExchangeDelete(brokerExchangeName, false, false)
}

func (suite *TestSuite) waitBrokerReady() {
	time.Sleep(5 * time.Second)
}

func (suite *TestSuite) getFunctionsPath() string {
	return path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "trigger", "rabbitmq", "test")
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestSuite))
}

type TestMessage struct {
	Message amqp.Publishing
	Topic   string
}
