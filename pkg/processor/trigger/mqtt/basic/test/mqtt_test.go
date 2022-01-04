//go:build test_integration && test_local && test_broken

// NOTE: Currently broken
// It seems that the mqtt eclipse container refuses to take incoming requests when running
// from GitHub Action worker, while working just fine when running locally - macOS.
// Container logs:
/*
	1630833404: mosquitto version 2.0.12 starting
	1630833404: Config loaded from /mosquitto/config/mosquitto.conf.
	1630833404: Opening ipv4 listen socket on port 1883.
	1630833404: Opening ipv6 listen socket on port 1883.
	1630833404: mosquitto version 1.6.15 running
	1630833405: New connection from 172.20.0.1 on port 1883.
	1630833405: Sending CONNACK to 172.20.0.1 (0, 2)
	1630833405: Client <unknown> disconnected due to protocol error.
	1630833406: New connection from 172.20.0.1 on port 1883.
	1630833406: Sending CONNACK to 172.20.0.1 (0, 2)
	... and so on
*/ // nolint: misspell

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
	"fmt"
	"path"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/mqtt"
	"github.com/nuclio/nuclio/pkg/processor/trigger/test"

	mqttclient "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/suite"
)

type testSuite struct {
	*triggertest.AbstractBrokerSuite
	brokerPort    int
	brokerURL     string
	containerName string
	mqttClient    mqttclient.Client
}

func (suite *testSuite) SetupSuite() {
	suite.brokerPort = 1883
	suite.containerName = "mqtt-mosquitto"                     // nolint: misspell
	suite.BrokerContainerNetworkName = "nuclio-mosquitto-test" // nolint: misspell

	suite.brokerURL = fmt.Sprintf("tcp://%s:%d", suite.BrokerHost, suite.brokerPort)

	// create client
	suite.mqttClient = mqttclient.NewClient(mqttclient.NewClientOptions().AddBroker(suite.brokerURL))
	suite.AbstractBrokerSuite.SetupSuite()
}

// GetContainerRunInfo returns information about the broker container
func (suite *testSuite) GetContainerRunInfo() (string, *dockerclient.RunOptions) {
	return "eclipse-mosquitto", &dockerclient.RunOptions{ // nolint: misspell
		ContainerName: suite.containerName,
		Network:       suite.BrokerContainerNetworkName,
		Remove:        true,
		Volumes: map[string]string{
			path.Join(suite.GetNuclioHostSourceDir(),
				"test",
				"mqtt",
				"artifacts",
				"mosquitto.conf"): "/mosquitto/config/mosquitto.conf", // nolint: misspell
		},
		Ports: map[int]int{
			suite.brokerPort: suite.brokerPort,
		},
	}
}

// WaitForBroker waits until the broker is ready
func (suite *testSuite) WaitForBroker() error {

	// retry to connect
	err := common.RetryUntilSuccessful(30*time.Second, 1*time.Second, func() bool {
		if token := suite.mqttClient.Connect(); token.Wait() && token.Error() != nil {
			return false
		}
		return true
	})

	// get broker logs in case connect has failed, we want the logs to be logged
	containerLogs, containerLogsErr := suite.DockerClient.GetContainerLogs(suite.containerName)
	suite.Logger.DebugWith("Fetched broker container logs", "logs", containerLogs)
	suite.Require().NoError(containerLogsErr, "Failed to get broker container logs")

	suite.Require().NoError(err, "Failed to connect to MQTT broker in given timeframe")
	return nil
}

func (suite *testSuite) TestMultipleTopics() {
	triggerConfiguration := suite.getTriggerConfiguration([]mqtt.Subscription{
		{Topic: "a1/b1/c1", QOS: 0},
		{Topic: "a1/b1", QOS: 1},
		{Topic: "a2/b2/c3/c4", QOS: 2},
	})

	// invoke the event recorder
	triggertest.InvokeEventRecorder(&suite.AbstractBrokerSuite.TestSuite,
		suite.BrokerHost,
		suite.getCreateFunctionOptionsWithMQTTTrigger(triggerConfiguration),
		map[string]triggertest.TopicMessages{
			"a1/b1/c1":    {NumMessages: 3},
			"a1/b1":       {NumMessages: 3},
			"a2/b2/c3/c4": {NumMessages: 3},
		},
		nil,
		suite.publishMessageToTopic)
}

func (suite *testSuite) getCreateFunctionOptionsWithMQTTTrigger(triggerConfig functionconfig.Trigger) *platform.CreateFunctionOptions {
	createFunctionOptions := suite.GetDeployOptions("event_recorder", "")
	createFunctionOptions.FunctionConfig.Spec.Platform = functionconfig.Platform{
		Attributes: map[string]interface{}{
			"network": suite.BrokerContainerNetworkName,
		},
	}
	createFunctionOptions.FunctionConfig.Spec.Runtime = "python"
	createFunctionOptions.FunctionConfig.Meta.Name = "event-recorder"
	createFunctionOptions.FunctionConfig.Spec.Build.Path = suite.FunctionPaths["python"]
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{}
	createFunctionOptions.FunctionConfig.Spec.Triggers["test_mqtt"] = triggerConfig
	createFunctionOptions.FunctionConfig.Spec.ReadinessTimeoutSeconds = 10

	return createFunctionOptions
}

func (suite *testSuite) getTriggerConfiguration(subscriptions []mqtt.Subscription) functionconfig.Trigger {
	return functionconfig.Trigger{
		Kind: "mqtt",
		URL:  fmt.Sprintf("tcp://%s:%d", suite.containerName, suite.brokerPort),
		Attributes: map[string]interface{}{
			"subscriptions": subscriptions,
		},
	}
}

func (suite *testSuite) publishMessageToTopic(topic string, body string) error {
	token := suite.mqttClient.Publish(topic,
		byte(0),
		false,
		body)

	token.Wait()

	return token.Error()
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	newTestSuite := &testSuite{}
	newTestSuite.AbstractBrokerSuite = triggertest.NewAbstractBrokerSuite(newTestSuite)
	suite.Run(t, newTestSuite)
}
