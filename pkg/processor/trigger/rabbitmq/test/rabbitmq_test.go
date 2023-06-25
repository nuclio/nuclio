//go:build test_integration && test_local

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

package test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/test"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
)

type testSuite struct {
	*triggertest.AbstractBrokerSuite
	brokerConn             *amqp.Connection
	brokerChannel          *amqp.Channel
	brokerQueue            amqp.Queue
	brokerPort             int
	brokerExchangeName     string
	brokerQueueName        string
	brokerURL              string
	containerizedBrokerURL string
}

func (suite *testSuite) SetupSuite() {
	suite.brokerURL = fmt.Sprintf("amqp://%s:%d", suite.GetTestHost(), suite.brokerPort)
	suite.containerizedBrokerURL = fmt.Sprintf("amqp://guest:guest@172.17.0.1:%d", suite.brokerPort)
	suite.AbstractBrokerSuite.SetupSuite()
}

func (suite *testSuite) TearDownTest() {
	suite.TestSuite.TearDownTest()

	// delete broker stuff
	suite.deleteBrokerResources(suite.brokerURL, suite.brokerExchangeName, suite.brokerQueueName)
}

// GetContainerRunInfo returns information about the broker container
func (suite *testSuite) GetContainerRunInfo() (string, *dockerclient.RunOptions) {
	return "rabbitmq:3-management", &dockerclient.RunOptions{
		Ports: map[int]int{suite.brokerPort: suite.brokerPort, 15671: 15671},
	}
}

// WaitForBroker waits until the broker is ready
func (suite *testSuite) WaitForBroker() error {
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

	return nil
}

func (suite *testSuite) TestReconnect() {
	// create a trigger configuration where the queue name is specified
	triggerConfig := functionconfig.Trigger{
		Kind: "rabbit-mq",
		URL:  suite.containerizedBrokerURL,
		Attributes: map[string]interface{}{
			"exchangeName": suite.brokerExchangeName,
			"queueName":    suite.brokerQueueName,
		},
	}
	suite.createBrokerResources([]string{"t1", "t2", "t3"})

	// invoke the event recorder
	triggertest.InvokeEventRecorder(&suite.AbstractBrokerSuite.TestSuite,
		suite.BrokerHost,
		suite.getCreateFunctionOptionsWithRmqTrigger(triggerConfig),
		map[string]triggertest.TopicMessages{
			"t1": {NumMessages: 3},
			"t2": {NumMessages: 3},
			"t3": {NumMessages: 3},
		},
		nil,
		func(topic string, body string) error {

			// publish few messages (basically publish all t1)
			// simulate network failure (by stopping the broker container)
			// start the broker container
			// publish few more messages
			if topic == "t2" && body == "t2-1" {

				// close test to broker connections
				suite.Require().NoError(suite.brokerChannel.Close())
				suite.Require().NoError(suite.brokerConn.Close())

				// close broker internal connections
				suite.closeAllBrokerConnections()

				// re-initialize broker connection
				suite.initializeBrokerConnection()

				// give the function some time to reconnect
				time.Sleep(45 * time.Second)
			}

			// publish
			return suite.publishMessageToTopic(topic, body)
		})
}

func (suite *testSuite) TestPreexistingResources() {

	// create a trigger configuration where the queue name is specified
	triggerConfig := functionconfig.Trigger{
		Kind: "rabbit-mq",
		URL:  suite.containerizedBrokerURL,
		Attributes: map[string]interface{}{
			"exchangeName": suite.brokerExchangeName,
			"queueName":    suite.brokerQueueName,

			// no topics passed means to listen on topics bound pre function deploy
			"topics": []string{},
		},
	}

	suite.createBrokerResources([]string{"t1", "t2", "t3"})

	// invoke the event recorder
	triggertest.InvokeEventRecorder(&suite.AbstractBrokerSuite.TestSuite,
		suite.BrokerHost,
		suite.getCreateFunctionOptionsWithRmqTrigger(triggerConfig),
		map[string]triggertest.TopicMessages{
			"t1": {NumMessages: 3},
			"t2": {NumMessages: 3},
			"t3": {NumMessages: 3},
		},
		nil,
		suite.publishMessageToTopic)
}

func (suite *testSuite) TestResourcesCreatedByFunction() {

	// Declare an exchange, but don't create a queue
	triggerConfig := functionconfig.Trigger{
		Kind: "rabbit-mq",
		URL:  fmt.Sprintf("amqp://guest:guest@172.17.0.1:%d", suite.brokerPort),
		Attributes: map[string]interface{}{
			"exchangeName": suite.brokerExchangeName,
			"queueName":    suite.brokerQueueName,
			"topics":       []string{"t1", "t2", "t3"},
		},
	}

	// invoke the event recorder
	triggertest.InvokeEventRecorder(&suite.AbstractBrokerSuite.TestSuite,
		suite.BrokerHost,
		suite.getCreateFunctionOptionsWithRmqTrigger(triggerConfig),
		map[string]triggertest.TopicMessages{
			"t1": {NumMessages: 3},
			"t2": {NumMessages: 3},
			"t3": {NumMessages: 3},
		},
		map[string]triggertest.TopicMessages{
			"t4": {NumMessages: 3},
			"t5": {NumMessages: 3},
		},
		suite.publishMessageToTopic)
}

func (suite *testSuite) getCreateFunctionOptionsWithRmqTrigger(triggerConfig functionconfig.Trigger) *platform.CreateFunctionOptions {
	createFunctionOptions := suite.GetDeployOptions("event_recorder", "")
	createFunctionOptions.FunctionConfig.Spec.Runtime = "python"
	createFunctionOptions.FunctionConfig.Meta.Name = "rmq-trigger-test"
	createFunctionOptions.FunctionConfig.Spec.Build.Path = suite.FunctionPaths["python"]
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{}
	createFunctionOptions.FunctionConfig.Spec.Triggers["test_rmq"] = triggerConfig
	return createFunctionOptions
}

func (suite *testSuite) createBrokerResources(topics []string) {

	var err error

	// initialize required connection to the broker
	suite.initializeBrokerConnection()

	// clear stuff before we create stuff
	suite.deleteBrokerResources(suite.brokerURL, suite.brokerExchangeName, suite.brokerQueueName)

	// create the exchange
	err = suite.brokerChannel.ExchangeDeclare(suite.brokerExchangeName,
		"topic",
		false,
		false,
		false,
		false,
		nil)
	suite.Require().NoError(err)

	// declare a queue and bind it, if a queue set
	if suite.brokerQueueName != "" {

		suite.brokerQueue, err = suite.brokerChannel.QueueDeclare(
			suite.brokerQueueName,
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
				suite.brokerExchangeName,
				false,
				nil)

			suite.Require().NoError(err, "Failed to bind queue")
		}
	}
}

func (suite *testSuite) deleteBrokerResources(brokerURL string, brokerExchangeName string, queueName string) {

	// delete the queue in case it exists
	suite.brokerChannel.QueueDelete(queueName, false, false, false) // nolint: errcheck

	// delete the exchange
	suite.brokerChannel.ExchangeDelete(brokerExchangeName, false, false) // nolint: errcheck
}

func (suite *testSuite) publishMessageToTopic(topic string, body string) error {
	amqpMessage := amqp.Publishing{
		ContentType: "application/text",
		Body:        []byte(body),
	}

	// publish the message
	return suite.brokerChannel.PublishWithContext(context.TODO(),
		suite.brokerExchangeName,
		topic,
		false,
		false,
		amqpMessage)
}

func (suite *testSuite) initializeBrokerConnection() {
	var err error
	suite.brokerConn, err = amqp.Dial(suite.brokerURL)
	suite.Require().NoError(err, "Failed to dial to broker")

	suite.brokerChannel, err = suite.brokerConn.Channel()
	suite.Require().NoError(err, "Failed to create broker channel")
}

func (suite *testSuite) closeAllBrokerConnections() {

	var stdout string
	// stdout will be something like "[{"name":"192.168.101.3:57931 -> 172.17.0.2:5672"}]"
	err := suite.DockerClient.ExecInContainer(suite.BrokerContainerID,
		&dockerclient.ExecOptions{
			Command: `rabbitmqadmin list connections name --format raw_json`,
			Stdout:  &stdout,
		})
	suite.Require().NoError(err)

	// unmarshal the json
	var connections []struct {
		Name string `json:"name"`
	}
	err = json.Unmarshal([]byte(stdout), &connections)
	suite.Require().NoError(err)
	for _, connection := range connections {
		stdout = ""
		err = suite.DockerClient.ExecInContainer(suite.BrokerContainerID,
			&dockerclient.ExecOptions{
				Command: fmt.Sprintf(`rabbitmqadmin close connection name='%s'`, connection.Name),
				Stdout:  &stdout,
			})
		suite.Require().NoError(err)
	}
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	newTestSuite := &testSuite{
		brokerPort:         5672,
		brokerExchangeName: "nuclio.rabbitmq_trigger_test",
		brokerQueueName:    "test-queue-" + xid.New().String(),
	}
	newTestSuite.AbstractBrokerSuite = triggertest.NewAbstractBrokerSuite(newTestSuite)
	suite.Run(t, newTestSuite)
}
