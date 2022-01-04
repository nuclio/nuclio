//go:build test_unit

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

package rabbitmq

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/test/suite"

	"github.com/stretchr/testify/suite"
)

const (
	exchangeName = "nuclio.rabbitmq_trigger_test"
	queueName    = "test_queue"
)

type TestSuite struct {
	processorsuite.TestSuite
	trigger rabbitMq
}

func (suite *TestSuite) SetupSuite() {
	suite.TestSuite.SetupSuite()
}

func (suite *TestSuite) TearDownSuite() {
	suite.TestSuite.TearDownTest()
}

func (suite *TestSuite) SetupTest() {
	suite.trigger = rabbitMq{}
	suite.trigger.Logger = suite.Logger.GetChild("rabbitMQ")

	suite.trigger.configuration = &Configuration{
		QueueName:    queueName,
		ExchangeName: exchangeName,
		Topics:       []string{"t1", "t2"},
	}
}

func (suite *TestSuite) TestSetEmptyParametersNoChange() {
	suite.trigger.setEmptyParameters()

	suite.EqualValues(suite.trigger.configuration.QueueName, queueName)
	suite.EqualValues(suite.trigger.configuration.Topics, []string{"t1", "t2"})
}

func (suite *TestSuite) TestSetEmptyParametersSetsEmptyQueueName() {
	suite.trigger.configuration.RuntimeConfiguration = &runtime.Configuration{
		Configuration: &processor.Configuration{},
	}

	// set namespace and name, as they contribute to function name
	suite.trigger.configuration.RuntimeConfiguration.Meta.Namespace = "mynamespace"
	suite.trigger.configuration.RuntimeConfiguration.Meta.Name = "myname"

	suite.trigger.configuration.QueueName = ""
	suite.trigger.setEmptyParameters()

	suite.EqualValues("nuclio-mynamespace-myname", suite.trigger.configuration.QueueName)
}

func (suite *TestSuite) TestSetEmptyParametersMakesNoChange() {
	suite.trigger.configuration.Topics = []string{}

	suite.trigger.setEmptyParameters()

	suite.EqualValues(suite.trigger.configuration.Topics, []string{})
}

func TestRabbitMQSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}
