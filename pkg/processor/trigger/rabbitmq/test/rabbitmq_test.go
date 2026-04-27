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
	"io"
	"net/http"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/rabbitmq"
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

// TestEvent represents the structure of each event returned by your Nuclio function.
// Adjust the fields according to your actual event structure.
type TestEvent struct {
	ID      string                 `json:"id"`
	Body    string                 `json:"body"`
	Headers map[string]interface{} `json:"headers"`
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
	return "rabbitmq:4-management", &dockerclient.RunOptions{
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
	triggertest.InvokeEventRecorder(&suite.TestSuite,
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
		}, nil)
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
	triggertest.InvokeEventRecorder(&suite.TestSuite,
		suite.BrokerHost,
		suite.getCreateFunctionOptionsWithRmqTrigger(triggerConfig),
		map[string]triggertest.TopicMessages{
			"t1": {NumMessages: 3},
			"t2": {NumMessages: 3},
			"t3": {NumMessages: 3},
		},
		nil,
		suite.publishMessageToTopic,
		nil)
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
	triggertest.InvokeEventRecorder(&suite.TestSuite,
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
		suite.publishMessageToTopic,
		nil)
}

// TestNonExistentQueueFailsToStart verifies that when a user provides a queue name that does not exist
// (e.g. exchange exists but queue was never created), the trigger fails at startup with
// a clear error instead of starting consumption and flooding logs with "delivery not initialized" ack errors.
func (suite *testSuite) TestNonExistentQueueFailsToStart() {
	exchangeName := "nuclio.rabbitmq_nonexistent_test"
	nonExistentQueueName := "non-existent-queue-" + xid.New().String()

	suite.initializeBrokerConnection()
	defer suite.deleteBrokerResources(suite.brokerURL, exchangeName, nonExistentQueueName)

	suite.createExchange(exchangeName, "fanout", true)

	triggerConfig := functionconfig.Trigger{
		Kind: "rabbit-mq",
		URL:  suite.containerizedBrokerURL,
		Attributes: map[string]interface{}{
			"exchangeName": exchangeName,
			"queueName":    nonExistentQueueName,
			"topics":       []string{},
		},
	}

	createFunctionOptions := suite.getCreateFunctionOptionsWithRmqTrigger(triggerConfig)
	createFunctionOptions.FunctionConfig.Meta.Name = "rmq-nonexistent-queue-test"

	_, deployErr := suite.DeployFunctionExpectError(createFunctionOptions, func(result *platform.CreateFunctionResult) bool {
		containerID := suite.resolveContainerID(result, createFunctionOptions)
		suite.Require().NotEmpty(containerID, "Expected a container to be created for the function")

		err := common.RetryUntilSuccessful(30*time.Second, 1*time.Second, func() bool {
			containerLogs, getLogsErr := suite.DockerClient.GetContainerLogs(containerID)
			if getLogsErr != nil {
				return false
			}
			// Match exact chain: queue missing -> broker resources fail -> trigger fails to start
			return strings.Contains(containerLogs, "Queue does not exist") &&
				strings.Contains(containerLogs, "Failed to start trigger") &&
				!strings.Contains(containerLogs, "delivery not initialized")
		})
		suite.Require().NoError(err,
			"Expected logs: 'Queue does not exist', 'Failed to start trigger', and no 'delivery not initialized'")

		// Processor exits when trigger fails to start; container should eventually stop
		err = common.RetryUntilSuccessful(15*time.Second, 1*time.Second, func() bool {
			containers, getContainersErr := suite.DockerClient.GetContainers(&dockerclient.GetContainerOptions{
				ID:      containerID,
				Stopped: true,
			})
			if getContainersErr != nil || len(containers) == 0 {
				return false
			}
			return containers[0].State != nil && containers[0].State.Status == "exited"
		})
		suite.Require().NoError(err, "Expected function container to exit after trigger failed to start")

		containers, getContainersErr := suite.DockerClient.GetContainers(&dockerclient.GetContainerOptions{
			ID:      containerID,
			Stopped: true,
		})
		suite.Require().NoError(getContainersErr)
		suite.Require().Len(containers, 1)
		suite.Require().NotNil(containers[0].State, "Container state should be available")
		suite.Require().NotZero(containers[0].State.ExitCode,
			"Expected non-zero exit code when trigger fails to start")

		return true
	})
	suite.Require().Error(deployErr)
}

func (suite *testSuite) TestNackAndRequeue() {
	expectedRequeued := []string{
		"success-5",
		"success-6",
		"success-7",
		"success-8",
		"success-9",
		"nack-once-0",
		"nack-once-1",
		"nack-once-2",
		"nack-once-3",
		"nack-once-4",
	}

	expectedNotRequeued := []string{
		"success-5",
		"success-6",
		"success-7",
		"success-8",
		"success-9",
	}

	testCases := []struct {
		name           string
		onError        string
		requeueOnError bool
		expectedEvents []string
	}{
		{
			name:           "NackWithRequeueOnError",
			onError:        string(rabbitmq.OnProcessErrorNack),
			requeueOnError: true,
			expectedEvents: expectedRequeued,
		},
		{
			name:           "NackWithoutRequeueOnError",
			onError:        string(rabbitmq.OnProcessErrorNack),
			requeueOnError: false,
			expectedEvents: expectedNotRequeued,
		},
		{
			name:           "AckOnError",
			onError:        string(rabbitmq.OnProcessErrorAck),
			expectedEvents: expectedNotRequeued,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			functionName := fmt.Sprintf("nack-requeue-%s", strings.ToLower(tc.name))
			functionPath := path.Join(
				suite.GetTestFunctionsDir(),
				"python",
				"rabbitmq",
				"ack-test.py",
			)
			topicName := "t1"
			suite.createBrokerResources([]string{topicName})

			triggerConfig := functionconfig.Trigger{
				Kind: "rabbit-mq",
				URL:  fmt.Sprintf("amqp://guest:guest@172.17.0.1:%d", suite.brokerPort),
				Attributes: map[string]interface{}{
					"exchangeName":   suite.brokerExchangeName,
					"queueName":      suite.brokerQueueName,
					"topics":         []string{"t1"},
					"onError":        tc.onError,
					"requeueOnError": tc.requeueOnError,
				},
			}

			createFunctionOptions := suite.GetDeployOptions(functionName, functionPath)
			createFunctionOptions.FunctionConfig.Spec.Build.Commands = []string{"pip install nuclio-sdk"}
			createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
				"my-rabbit-mq": triggerConfig,
				"my-http": {
					Kind:       "http",
					Attributes: map[string]interface{}{},
				},
			}

			suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
				suite.Require().NotNil(deployResult, "Unexpected empty deploy results")

				for index := 0; index < 10; index++ {
					message := fmt.Sprintf("success-%d", index)
					if index < 5 {
						message = fmt.Sprintf("nack-once-%d", index)
					}
					err := suite.publishMessageToTopic(topicName, message)
					suite.Require().NoError(err, "Failed to publish message to topic")
				}

				var events []TestEvent
				err := common.RetryUntilSuccessful(30*time.Second, 3*time.Second, func() bool {
					url := fmt.Sprintf("http://%s:%d", suite.GetTestHost(), deployResult.Port)
					resp, err := http.Get(url)
					if err != nil {
						return false
					}
					defer resp.Body.Close()
					bodyBytes, err := io.ReadAll(resp.Body)
					if err != nil {
						return false
					}
					if err := json.Unmarshal(bodyBytes, &events); err != nil {
						return false
					}
					return len(events) >= len(tc.expectedEvents)
				})
				suite.Require().NoError(err, "Failed to wait for all events to arrive")

				// Validate order and count
				suite.Require().Equal(len(tc.expectedEvents), len(events),
					"Unexpected number of events for %s", tc.name)

				for i, expected := range tc.expectedEvents {
					suite.Require().Equal(expected, events[i].Body,
						"Unexpected message order at index %d: expected %s, got %s", i, expected, events[i].Body)
				}

				return true
			})
		})
	}
}

// resolveContainerID returns container ID from deploy result or by looking up the function container by name (e.g. when result is nil on failure).
func (suite *testSuite) resolveContainerID(result *platform.CreateFunctionResult, createFunctionOptions *platform.CreateFunctionOptions) string {
	if result != nil && result.ContainerID != "" {
		return result.ContainerID
	}
	containerName := fmt.Sprintf("nuclio-%s-%s",
		createFunctionOptions.FunctionConfig.Meta.Namespace,
		createFunctionOptions.FunctionConfig.Meta.Name)
	containers, err := suite.DockerClient.GetContainers(&dockerclient.GetContainerOptions{
		Name:    containerName,
		Stopped: true,
	})
	if err != nil || len(containers) == 0 {
		return ""
	}
	return containers[0].ID
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

	suite.createExchange(suite.brokerExchangeName, "topic", false)

	// declare a queue and bind it, if a queue set
	if suite.brokerQueueName != "" {

		suite.brokerQueue, err = suite.brokerChannel.QueueDeclare(
			suite.brokerQueueName,
			true, // durable — required by RabbitMQ 4+ (transient non-exclusive queues removed)
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

// createExchange declares an exchange
func (suite *testSuite) createExchange(exchangeName string, exchangeType string, durable bool) {
	err := suite.brokerChannel.ExchangeDeclare(exchangeName,
		exchangeType,
		durable,
		false,
		false,
		false,
		nil)
	suite.Require().NoError(err, "Failed to declare exchange %q", exchangeName)
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
	// default user
	user := "guest"
	var stdout string
	err := suite.DockerClient.ExecInContainer(suite.BrokerContainerID,
		&dockerclient.ExecOptions{
			Command: fmt.Sprintf(`rabbitmqadmin close user_connections --username '%s'`, user),
			Stdout:  &stdout,
		})

	suite.Require().NoError(err)
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
