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
	brokerPort int
	mqttClient mqttclient.Client
}

func newTestSuite() *testSuite {
	newTestSuite := &testSuite{
		brokerPort: 1883,
	}

	newTestSuite.AbstractBrokerSuite = triggertest.NewAbstractBrokerSuite(newTestSuite)

	return newTestSuite
}

func (suite *testSuite) TearDownTest() {
	suite.TestSuite.TearDownTest()
}

// GetContainerRunInfo returns information about the broker container
func (suite *testSuite) GetContainerRunInfo() (string, *dockerclient.RunOptions) {
	return "eclipse-mosquitto", &dockerclient.RunOptions{
		Ports: map[int]int{
			suite.brokerPort: suite.brokerPort,
			9001:             9001,
		},
	}
}

// WaitForBroker waits until the broker is ready
func (suite *testSuite) WaitForBroker() error {
	err := common.RetryUntilSuccessful(30*time.Second, 1*time.Second, func() bool {
		brokerURL := fmt.Sprintf("tcp://%s:%d", suite.BrokerHost, suite.brokerPort)

		// create client
		suite.mqttClient = mqttclient.NewClient(mqttclient.NewClientOptions().AddBroker(brokerURL))

		// try to connect
		if token := suite.mqttClient.Connect(); token.Wait() && token.Error() != nil {
			return false
		}

		return true
	})

	suite.Require().NoError(err, "Failed to connect to MQTT broker in given timeframe")

	return nil
}

func (suite *testSuite) TestMulitpleTopics() {
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
			"a1/b1/c1":    {3},
			"a1/b1":       {3},
			"a2/b2/c3/c4": {3},
		},
		nil,
		suite.publishMessageToTopic)
}

func (suite *testSuite) getCreateFunctionOptionsWithMQTTTrigger(triggerConfig functionconfig.Trigger) *platform.CreateFunctionOptions {
	createFunctionOptions := suite.GetDeployOptions("event_recorder", "")

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
		URL:  fmt.Sprintf("tcp://172.17.0.1:%d", suite.brokerPort),
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

	suite.Run(t, newTestSuite())
}
