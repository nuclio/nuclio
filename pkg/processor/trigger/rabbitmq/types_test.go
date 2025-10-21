//go:build test_unit

/*
Copyright 2025 The Nuclio Authors.

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

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type ConfigurationTestSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *ConfigurationTestSuite) SetupSuite() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
}

func (suite *ConfigurationTestSuite) TestParseACKConfig() {
	testCases := []struct {
		name           string
		onError        OnProcessError
		requeueOnError bool
	}{
		{"Ack_True", OnProcessErrorAck, true},
		{"Ack_False", OnProcessErrorAck, false},
		{"Nack_True", OnProcessErrorNack, true},
		{"Nack_False", OnProcessErrorNack, false},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			triggerConfig := &functionconfig.Trigger{
				Attributes: map[string]interface{}{
					"exchangeName":      "my-exchange",
					"queueName":         "my-queue",
					"reconnectDuration": "10m",
					"reconnectInterval": "20s",
					"onError":           string(tc.onError),
					"requeueOnError":    tc.requeueOnError,
				},
			}

			cfg, err := NewConfiguration("test-id", triggerConfig, &runtime.Configuration{})
			suite.Require().NoError(err, "Expected configuration parsing to succeed")

			suite.Equal(tc.onError, cfg.OnError)
			suite.Equal(tc.requeueOnError, cfg.RequeueOnError)
		})
	}
}

func TestConfigurationSuite(t *testing.T) {
	suite.Run(t, new(ConfigurationTestSuite))
}
