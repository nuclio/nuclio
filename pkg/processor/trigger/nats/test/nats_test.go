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
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/test/suite"

	"github.com/streadway/amqp"
	"github.com/stretchr/testify/suite"
)

const (
	brokerExchangeName = "nuclio.nats_trigger_test"
	brokerQueueName    = "test_queue"
	brokerPort         = 5672
)

type TestSuite struct {
	processorsuite.TestSuite
	natsContainerID    string
	brokerConn         *amqp.Connection
	brokerChannel      *amqp.Channel
	brokerQueue        amqp.Queue
	brokerPort         int
	brokerExchangeName string
	brokerQueueName    string
	brokerURL          string
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

	// start nats
	suite.natsContainerID, err = suite.DockerClient.RunContainer("nats:latest",
		&dockerclient.RunOptions{
			Ports: map[int]int{4222: 4222, 8222: 8222},
		})

	suite.Require().NoError(err, "Failed to start NATS container")

	suite.waitBrokerReady()
}

func (suite *TestSuite) TearDownSuite() {
	suite.TestSuite.TearDownTest()

	// if we weren't successful starting, nothing to do
	if suite.natsContainerID != "" {
		suite.DockerClient.RemoveContainer(suite.natsContainerID)
	}
}

func (suite *TestSuite) SetupTest() {
	suite.TestSuite.SetupTest()

	suite.FunctionDir = suite.getFunctionsPath()

	// create broker stuff
	suite.createBrokerResources(suite.brokerURL, suite.brokerExchangeName, suite.brokerQueueName)
}

func (suite *TestSuite) TearDownTest() {
	suite.TestSuite.TearDownTest()

	// delete broker stuff
	suite.deleteBrokerResources(suite.brokerURL, suite.brokerExchangeName, suite.brokerQueueName)
}

func (suite *TestSuite) TestPostEventPython() {
	suite.invokeEventRecorder("event_recorder_python", "python")
}

func (suite *TestSuite) TestPostEventGolang() {
	suite.invokeEventRecorder(path.Join("_event_recorder_golang", "event_recorder.go"), "golang")
}

func (suite *TestSuite) invokeEventRecorder(functionPath string, runtimeType string) {
	suite.Runtime = runtimeType

	deployOptions := suite.GetDeployOptions("event_recorder",
		suite.GetFunctionPath(functionPath))

	suite.DeployFunction(deployOptions, func(deployResult *platform.DeployResult) bool {
		message := amqp.Publishing{}

		// send 3 messages
		for requestIdx := 0; requestIdx < 3; requestIdx++ {

			message.ContentType = "application/json"
			message.Body = []byte(fmt.Sprintf(`{"request": "%d"}`, requestIdx))

			// publish the message
			err := suite.brokerChannel.Publish(suite.brokerExchangeName,
				"t1",
				false,
				false,
				message)

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
		suite.Require().Equal([]map[string]string{
			{"request": "0"},
			{"request": "1"},
			{"request": "2"},
		}, receivedEvents)

		return true
	})
}

func (suite *TestSuite) createBrokerResources(brokerURL string, brokerExchangeName string, queueName string) {
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

	suite.brokerQueue, err = suite.brokerChannel.QueueDeclare(
		queueName,
		false,
		false,
		false,
		false,
		nil)

	suite.Require().NoError(err, "Failed to declare queue")

	err = suite.brokerChannel.QueueBind(
		suite.brokerQueue.Name,
		"*",
		brokerExchangeName,
		false,
		nil)

	suite.Require().NoError(err, "Failed to bind queue")
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
	return path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "trigger", "nats", "test")
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestSuite))
}
