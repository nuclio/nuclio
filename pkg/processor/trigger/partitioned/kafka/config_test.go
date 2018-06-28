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

package kafka

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/processor/trigger/partitioned"

	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type kafkaConfigTestSuite struct {
	suite.Suite
}

func (suite *kafkaConfigTestSuite) TestConfigurationOverrides() {

	kafkaTrigger := suite.newTrigger()

	user := "daffy"
	password := "ducker"
	bufferSize := 99

	attributes := map[string]interface{}{
		"driver": map[string]interface{}{
			"Net": map[string]interface{}{
				"SASL": map[string]interface{}{
					"User":     user,
					"Password": password,
					"Enable":   true,
				},
			},
			"ChannelBufferSize": bufferSize,
		},
	}

	kafkaConfig, err := kafkaTrigger.newKafkaConfig(attributes)
	suite.Require().NoErrorf(err, "Can't create kafka configuration from %+v", attributes)

	suite.Require().Equal(user, kafkaConfig.Net.SASL.User, "User mismatch")
	suite.Require().Equal(password, kafkaConfig.Net.SASL.Password, "Password mismatch")
	suite.Require().True(kafkaConfig.Net.SASL.Enable, "SASL not enabled")
	suite.Require().Equal(kafkaConfig.ChannelBufferSize, bufferSize, "ChannelBufferSize mismatch")
}

func (suite *kafkaConfigTestSuite) TestDefaultConfiguration() {
	kafkaTrigger := suite.newTrigger()
	kafkaConfig, err := kafkaTrigger.newKafkaConfig(nil)
	suite.Require().NoError(err, "Can't create kafka configuration from nil")

	// We can't complare to sarama.NewConfig() since two calls to this function
	// will return difference objects
	// TODO: Find the fields that are different and ignore them
	suite.Require().Equal("", kafkaConfig.Net.SASL.User, "Non empty user")
}

func (suite *kafkaConfigTestSuite) newTrigger() *kafka {
	logger, err := nucliozap.NewNuclioZapTest("kafa-config")
	suite.Require().NoError(err, "Can't create logger")

	kafkaTrigger := &kafka{
		AbstractStream: &partitioned.AbstractStream{},
	}
	kafkaTrigger.Logger = logger

	return kafkaTrigger
}

func TestConfiguration(t *testing.T) {
	suite.Run(t, &kafkaConfigTestSuite{})
}
