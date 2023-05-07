//go:build test_unit

/*
Copyright 2018 The Nuclio Authors.

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
	"os"
	"path"
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/util/partitionworker"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	suite.Suite
	trigger kafka
	logger  logger.Logger
}

func (suite *TestSuite) SetupSuite() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	suite.trigger = kafka{
		AbstractTrigger: trigger.AbstractTrigger{
			Logger: suite.logger,
		},
		configuration: &Configuration{},
	}
}

func (suite *TestSuite) TestPopulateValuesFromMountedSecrets() {

	// mock a mounted secret by creating a files in a temp dir
	// and setting the configuration fields to point to it

	// create temp dir
	tempDir, err := os.MkdirTemp("", "test")
	suite.Require().NoError(err)
	defer os.RemoveAll(tempDir)

	sensitiveConfigFields := []struct {
		fileName    string
		value       string
		configField *string
	}{
		{"accessKey", "test-access-key", &suite.trigger.configuration.AccessKey},
		{"AccessCert", "test-access-certificate", &suite.trigger.configuration.AccessCertificate},
		{"caCert", "test-ca-certificate", &suite.trigger.configuration.CACert},
		{"SASLPassword", "test-sasl-password", &suite.trigger.configuration.SASL.Password},
		{"SASLClientSecret", "test-sasl-client-secret", &suite.trigger.configuration.SASL.OAuth.ClientSecret},
	}

	for _, field := range sensitiveConfigFields {

		// create files
		filePath := path.Join(tempDir, field.fileName)
		err = os.WriteFile(filePath, []byte(field.value), 0644)
		suite.Require().NoError(err)

		// set config value
		*field.configField = field.fileName
	}

	// set configuration fields to point to the temp dir
	suite.trigger.configuration.SecretPath = tempDir

	err = suite.trigger.configuration.populateValuesFromMountedSecrets(suite.logger)
	suite.Require().NoError(err)

	for _, field := range sensitiveConfigFields {
		suite.Require().Equal(field.value, *field.configField)
	}
}

func (suite *TestSuite) TestExplicitAckModeWithWorkerAllocationModes() {
	for _, testCase := range []struct {
		name                 string
		explicitAckMode      functionconfig.ExplicitAckMode
		workerAllocationMode partitionworker.AllocationMode
		expectedFailure      bool
	}{
		{
			name:                 "Disable-Static",
			explicitAckMode:      functionconfig.ExplicitAckModeDisable,
			workerAllocationMode: partitionworker.AllocationModeStatic,
			expectedFailure:      false,
		},
		{
			name:                 "Disable-Pool",
			explicitAckMode:      functionconfig.ExplicitAckModeDisable,
			workerAllocationMode: partitionworker.AllocationModePool,
			expectedFailure:      false,
		},
		{
			name:                 "Enable-Static",
			explicitAckMode:      functionconfig.ExplicitAckModeEnable,
			workerAllocationMode: partitionworker.AllocationModeStatic,
			expectedFailure:      false,
		},
		{
			name:                 "Enable-Pool",
			explicitAckMode:      functionconfig.ExplicitAckModeEnable,
			workerAllocationMode: partitionworker.AllocationModePool,
			expectedFailure:      true,
		},
		{
			name:                 "ExplicitOnly-Static",
			explicitAckMode:      functionconfig.ExplicitAckModeExplicitOnly,
			workerAllocationMode: partitionworker.AllocationModeStatic,
			expectedFailure:      false,
		},
		{
			name:                 "ExplicitOnly-Pool",
			explicitAckMode:      functionconfig.ExplicitAckModeEnable,
			workerAllocationMode: partitionworker.AllocationModePool,
			expectedFailure:      true,
		},
	} {
		suite.Run(testCase.name, func() {
			_, err := NewConfiguration(testCase.name,
				&functionconfig.Trigger{
					// populate some dummy values
					Attributes: map[string]interface{}{
						"topics": []string{
							"some-topic",
						},
						"consumerGroup": "some-cg",
						"initialOffset": "earliest",
						"brokers": []string{
							"some-broker",
						},
						"workerAllocationMode": string(testCase.workerAllocationMode),
					},
				},
				&runtime.Configuration{
					Configuration: &processor.Configuration{
						Config: functionconfig.Config{
							Meta: functionconfig.Meta{
								Annotations: map[string]string{
									"nuclio.io/kafka-explicit-ack-mode": string(testCase.explicitAckMode),
								},
							},
						},
					},
				},
				suite.logger)
			if testCase.expectedFailure {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
			}
		})
	}
}

func TestKafkaSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}
