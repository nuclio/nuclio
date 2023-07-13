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

package v3iostream

import (
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
	trigger v3iostream
	logger  logger.Logger
}

func (suite *TestSuite) SetupSuite() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	suite.trigger = v3iostream{
		AbstractTrigger: trigger.AbstractTrigger{
			Logger: suite.logger,
		},
		configuration: &Configuration{},
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
						"containerName":        "my-container",
						"streamPath":           "/my-stream",
						"consumerGroup":        "some-cg",
						"password":             "some-password",
						"workerAllocationMode": string(testCase.workerAllocationMode),
					},
				},
				&runtime.Configuration{
					Configuration: &processor.Configuration{
						Config: functionconfig.Config{
							Meta: functionconfig.Meta{
								Annotations: map[string]string{
									"nuclio.io/v3iostream-explicit-ack-mode": string(testCase.explicitAckMode),
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
