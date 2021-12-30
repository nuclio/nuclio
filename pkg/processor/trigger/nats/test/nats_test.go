//go:build test_integration && test_local

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
	"github.com/nuclio/nuclio/pkg/processor/trigger/test"

	"github.com/nats-io/go-nats"
	"github.com/stretchr/testify/suite"
)

type testSuite struct {
	*triggertest.AbstractBrokerSuite
	natsConn  *nats.Conn
	natsPort  int
	topicName string
}

func newTestSuite() *testSuite {
	newTestSuite := &testSuite{
		natsPort:  4222,
		topicName: "my.topic",
	}

	newTestSuite.AbstractBrokerSuite = triggertest.NewAbstractBrokerSuite(newTestSuite)

	return newTestSuite
}

// GetContainerRunInfo returns information about the broker container
func (suite *testSuite) GetContainerRunInfo() (string, *dockerclient.RunOptions) {
	return "nats:1.0.6-linux", &dockerclient.RunOptions{
		Ports: map[int]int{suite.natsPort: suite.natsPort, 8222: 8222},
	}
}

func (suite *testSuite) TestPostEvent() {
	err := suite.createNatsConnection()
	suite.Require().NoError(err, "Failed to create NATS connection")

	// invoke the event recorder
	triggertest.InvokeEventRecorder(&suite.AbstractBrokerSuite.TestSuite,
		suite.BrokerHost,
		suite.getDeployOptions(),
		map[string]triggertest.TopicMessages{
			suite.topicName: {NumMessages: 3},
		},
		nil,
		suite.publishMessageToTopic)
}

func (suite *testSuite) getDeployOptions() *platform.CreateFunctionOptions {
	createFunctionOptions := suite.GetDeployOptions("event_recorder", "")

	createFunctionOptions.FunctionConfig.Spec.Runtime = "python"
	createFunctionOptions.FunctionConfig.Meta.Name = "nats-test"
	createFunctionOptions.FunctionConfig.Spec.Build.Path = suite.FunctionPaths["python"]
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{}
	createFunctionOptions.FunctionConfig.Spec.Triggers["nats"] = functionconfig.Trigger{
		Kind: "nats",
		URL:  fmt.Sprintf("nats://172.17.0.1:%d", suite.natsPort),
		Attributes: map[string]interface{}{
			"topic": suite.topicName,
		},
	}

	return createFunctionOptions
}

func (suite *testSuite) publishMessageToTopic(topic string, body string) error {
	return suite.natsConn.Publish(topic, []byte(body))
}

func (suite *testSuite) createNatsConnection() error {

	// Try to perform connection to Nats
	return common.RetryUntilSuccessful(15*time.Second, 1*time.Second, func() bool {
		suite.natsConn, _ = nats.Connect(fmt.Sprintf("nats://%s:%d", suite.GetTestHost(), suite.natsPort))

		// If we're connected to the Nats get up from the function
		return suite.natsConn.IsConnected()
	})
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, newTestSuite())
}
