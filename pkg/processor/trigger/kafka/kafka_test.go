//go:build test_unit

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

package kafka

import (
	"crypto/tls"
	"os"
	"path"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/util/partitionworker"

	"github.com/Shopify/sarama"
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

func (suite *TestSuite) TestWorkerTimeoutConfiguration() {
	for _, testCase := range []struct {
		name                   string
		timeout                string
		expectedRuntimeTimeout time.Duration
		expectedFailure        bool
	}{
		{
			name:                   "Timeout specified",
			timeout:                "2s",
			expectedRuntimeTimeout: 2 * time.Second,
			expectedFailure:        false,
		},
		{
			name:                   "Timeout not specified",
			timeout:                "",
			expectedRuntimeTimeout: 10 * time.Second,
			expectedFailure:        false,
		},
		{
			name:            "Wrong timeout value",
			timeout:         "timeout",
			expectedFailure: true,
		},
	} {
		triggerInstance := &functionconfig.Trigger{
			WorkerTerminationTimeout: testCase.timeout,
			Attributes: map[string]interface{}{
				"topics": []string{
					"some-topic",
				},
				"consumerGroup":            "some-cg",
				"initialOffset":            "earliest",
				"workerTerminationTimeout": testCase.timeout,
				"brokers": []string{
					"some-broker",
				},
			},
		}
		suite.Run(testCase.name, func() {
			configuration, err := NewConfiguration(testCase.name,
				triggerInstance,
				&runtime.Configuration{
					Configuration: &processor.Configuration{
						Config: functionconfig.Config{},
					},
				},
				suite.logger)
			if testCase.expectedFailure {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
				suite.Require().Equal(testCase.expectedRuntimeTimeout, configuration.RuntimeConfiguration.WorkerTerminationTimeout, "Bad timeout value")
			}
		})
	}
}

func (suite *TestSuite) TestSASLHandshakeConfiguration() {
	trueValue := true
	falseValue := false

	testCases := []struct {
		name              string
		handshake         *bool
		expectedHandshake bool
	}{
		{
			name:              "Default handshake (nil) - expect true",
			handshake:         nil,
			expectedHandshake: true,
		},
		{
			name:              "Explicit handshake true",
			handshake:         &trueValue,
			expectedHandshake: true,
		},
		{
			name:              "Explicit handshake false",
			handshake:         &falseValue,
			expectedHandshake: false,
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			triggerInstance := &functionconfig.Trigger{
				Attributes: map[string]interface{}{
					"topics":        []string{"some-topic"},
					"consumerGroup": "kafkaGroup1",
					"initialOffset": "earliest",
					"brokers":       []string{"some-broker"},
					"sasl": map[string]interface{}{
						"enable":    true,
						"user":      "my_user",
						"password":  "my_password",
						"mechanism": "PLAIN",
					},
					"tls": map[string]interface{}{
						"enable":             true,
						"insecureSkipVerify": true,
						"minimumVersion":     "1.2",
					},
					"version": "1.0.0",
				},
			}

			// Set handshake if provided
			if testCase.handshake != nil {
				triggerInstance.Attributes["sasl"].(map[string]interface{})["handshake"] = *testCase.handshake
			}

			// Create and validate configuration
			configuration, err := NewConfiguration("new-configuration", triggerInstance, &runtime.Configuration{
				Configuration: &processor.Configuration{
					Config: functionconfig.Config{
						Meta: functionconfig.Meta{},
					},
				},
			}, suite.logger)
			suite.Require().NoError(err)

			trigger, err := newTrigger(suite.logger, nil, configuration, nil)
			suite.Require().NoError(err)
			kafkaTrigger := trigger.(*kafka)

			// Assert SASL configuration
			suite.Require().Equal(true, kafkaTrigger.kafkaConfig.Net.SASL.Enable)
			suite.Require().Equal("my_user", kafkaTrigger.kafkaConfig.Net.SASL.User)
			suite.Require().Equal("my_password", kafkaTrigger.kafkaConfig.Net.SASL.Password)
			suite.Require().Equal(sarama.SASLMechanism(sarama.SASLTypePlaintext), kafkaTrigger.kafkaConfig.Net.SASL.Mechanism)

			// Assert TLS configuration
			suite.Require().Equal(true, kafkaTrigger.kafkaConfig.Net.TLS.Enable)
			suite.Require().Equal(true, kafkaTrigger.kafkaConfig.Net.TLS.Config.InsecureSkipVerify)
			suite.Require().Equal(uint16(tls.VersionTLS12), kafkaTrigger.kafkaConfig.Net.TLS.Config.MinVersion)

			// Assert SASL handshake
			suite.Require().Equal(testCase.expectedHandshake, kafkaTrigger.kafkaConfig.Net.SASL.Handshake)
		})
	}
}

func (suite *TestSuite) TestWaitExplicitAckDuringRebalanceTimeoutConfiguration() {
	for _, testCase := range []struct {
		name                                          string
		timeoutConfig                                 string
		timeoutAnnotation                             string
		expectedWaitExplicitAckDuringRebalanceTimeout time.Duration
	}{
		{
			name:          "Timeout specified only in config",
			timeoutConfig: "2s",
			expectedWaitExplicitAckDuringRebalanceTimeout: 2 * time.Second,
		},
		{
			name: "Timeout not specified",
			expectedWaitExplicitAckDuringRebalanceTimeout: 100 * time.Millisecond,
		},
		{
			name:              "Timeout specified only in annotations",
			timeoutConfig:     "",
			timeoutAnnotation: "2s",
			expectedWaitExplicitAckDuringRebalanceTimeout: 2 * time.Second,
		},
		{
			name:              "Timeout specified in both config and annotations",
			timeoutConfig:     "1s",
			timeoutAnnotation: "2s",
			expectedWaitExplicitAckDuringRebalanceTimeout: 2 * time.Second,
		},
	} {
		triggerInstance := &functionconfig.Trigger{
			WaitExplicitAckDuringRebalanceTimeout: testCase.timeoutConfig,
			Attributes: map[string]interface{}{
				"topics": []string{
					"some-topic",
				},
				"consumerGroup": "some-cg",
				"initialOffset": "earliest",
				"brokers": []string{
					"some-broker",
				},
			},
		}

		suite.Run(testCase.name, func() {
			annotations := make(map[string]string)
			if testCase.timeoutAnnotation != "" {
				annotations["nuclio.io/wait-explicit-ack-during-rebalance-timeout"] = testCase.timeoutAnnotation
			}
			configuration, err := NewConfiguration(testCase.name,
				triggerInstance,
				&runtime.Configuration{
					Configuration: &processor.Configuration{
						Config: functionconfig.Config{Meta: functionconfig.Meta{Annotations: annotations}},
					},
				},
				suite.logger)
			suite.Require().NoError(err)
			suite.Require().Equal(testCase.expectedWaitExplicitAckDuringRebalanceTimeout, configuration.waitExplicitAckDuringRebalanceTimeout, "Bad timeout value")
		})
	}
}

func TestKafkaSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}
