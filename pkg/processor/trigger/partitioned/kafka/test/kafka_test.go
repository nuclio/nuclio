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

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger/test"

	"github.com/stretchr/testify/suite"
)

type testSuite struct {
	*KafkaTestSuite
}

func (suite *testSuite) TestReceiveRecords() {
	createFunctionOptions := suite.GetDeployOptions("event_recorder", suite.FunctionPaths["python"])
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{}
	createFunctionOptions.FunctionConfig.Spec.Triggers["my-kafka"] = functionconfig.Trigger{
		Kind: "kafka",
		URL:  fmt.Sprintf("%s:9092", suite.BrokerHost),
		Attributes: map[string]interface{}{
			"topic":      suite.Topic,
			"partitions": []int{0},
		},
	}

	triggertest.InvokeEventRecorder(&suite.AbstractBrokerSuite.TestSuite,
		suite.BrokerHost,
		createFunctionOptions,
		map[string]triggertest.TopicMessages{suite.Topic: {3}},
		nil,
		suite.PublishMessageToTopic)
}

// TestIntegrationSuite runs the suite
func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suiteInstance := &testSuite{}
	suiteInstance.KafkaTestSuite = NewKafkaTestSuite("test-topic", 1, suiteInstance)
	suite.Run(t, suiteInstance)
}
