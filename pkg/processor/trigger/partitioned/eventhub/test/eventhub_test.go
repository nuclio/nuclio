//go:build test_integration && test_iguazio

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
	"context"
	"os"
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger/test"
	"github.com/nuclio/nuclio/pkg/processor/util/eventhub"

	"github.com/Azure/go-amqp"
	"github.com/stretchr/testify/suite"
)

type testSuite struct {
	*triggertest.AbstractBrokerSuite
	eventhubSender       *amqp.Sender
	namespace            string
	sharedAccessKeyName  string
	sharedAccessKeyValue string
	eventhubName         string
}

func newTestSuite() *testSuite {
	newTestSuite := &testSuite{
		namespace:            os.Getenv("NUCLIO_EVENTHUB_TEST_NAMESPACE"),
		sharedAccessKeyName:  os.Getenv("NUCLIO_EVENTHUB_TEST_SHARED_ACCESS_KEY_NAME"),
		sharedAccessKeyValue: os.Getenv("NUCLIO_EVENTHUB_TEST_SHARED_ACCESS_KEY_VALUE"),
		eventhubName:         os.Getenv("NUCLIO_EVENTHUB_TEST_EVENTHUB_NAME"),
	}

	newTestSuite.AbstractBrokerSuite = triggertest.NewAbstractBrokerSuite(newTestSuite)

	return newTestSuite
}

func (suite *testSuite) SetupSuite() {
	suite.AbstractBrokerSuite.SetupSuite()

	session, err := eventhubutil.CreateSession(suite.namespace, suite.sharedAccessKeyName, suite.sharedAccessKeyValue)
	suite.Require().NoError(err)

	// Create a sender
	address := amqp.LinkTargetAddress(suite.eventhubName)
	suite.eventhubSender, err = session.NewSender(address)
	suite.Require().NoError(err)
}

func (suite *testSuite) TestReceiveRecords() {
	createFunctionOptions := suite.GetDeployOptions("event_recorder", suite.FunctionPaths["python"])
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{}
	createFunctionOptions.FunctionConfig.Spec.Triggers["my-eventhub"] = functionconfig.Trigger{
		Kind: "eventhub",
		Attributes: map[string]interface{}{
			"namespace":            suite.namespace,
			"sharedAccessKeyName":  suite.sharedAccessKeyName,
			"sharedAccessKeyValue": suite.sharedAccessKeyValue,
			"eventhubName":         suite.eventhubName,
			"partitions":           []int{0, 1},
		},
	}

	triggertest.InvokeEventRecorder(&suite.AbstractBrokerSuite.TestSuite,
		suite.BrokerHost,
		createFunctionOptions,
		map[string]triggertest.TopicMessages{"": {NumMessages: 3}},
		nil,
		suite.publishMessageToTopic)
}

func (suite *testSuite) publishMessageToTopic(topic string, body string) error {
	message := amqp.Message{
		Data: [][]byte{[]byte(body)},
	}

	return suite.eventhubSender.Send(context.Background(), &message)
}

func TestIntegrationSuite(t *testing.T) {

	if testing.Short() {
		return
	}

	// requires an Iguazio system
	suite.Run(t, newTestSuite())
}
