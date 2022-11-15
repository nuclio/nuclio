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

package mask

import (
	"context"
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/api/core/v1"
)

type MaskTestSuite struct {
	suite.Suite
	logger logger.Logger
	ctx    context.Context
}

func (suite *MaskTestSuite) SetupTest() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	suite.ctx = context.Background()
}

func (suite *MaskTestSuite) TestMaskBasics() {
	functionConfig := &functionconfig.Config{
		Spec: functionconfig.Spec{
			Build: functionconfig.Build{
				CodeEntryAttributes: map[string]interface{}{

					// should be masked
					"password": "abcd",
				},

				// should not be masked
				Image: "some-image:latest",
			},
			Triggers: map[string]functionconfig.Trigger{
				"secret-trigger": {
					Attributes: map[string]interface{}{
						"password": "1234",
					},
					Password: "4567",
				},
				"non-secret-trigger": {
					Attributes: map[string]interface{}{
						"not-a-password": "4321",
					},
				},
			},

			// check nested fields
			Volumes: []functionconfig.Volume{
				{
					Volume: v1.Volume{
						VolumeSource: v1.VolumeSource{
							FlexVolume: &v1.FlexVolumeSource{
								Options: map[string]string{
									"accesskey": "some-access-key",
								},
							},
						},
					},
				},
			},
		},
	}

	// mask the function config
	maskedFunctionConfig, secretMap, err := ScrubSensitiveDataInFunctionConfig(functionConfig, nil)
	suite.Require().NoError(err)

	suite.logger.DebugWith("Masked function config", "functionConfig", maskedFunctionConfig, "secretMap", secretMap)

	suite.Require().NotEmpty(secretMap)
	suite.Require().NotEqual(functionConfig.Spec.Build.CodeEntryAttributes["password"],
		maskedFunctionConfig.Spec.Build.CodeEntryAttributes["password"])
	suite.Require().Equal(functionConfig.Spec.Build.Image, maskedFunctionConfig.Spec.Build.Image)
	suite.Require().NotEqual(functionConfig.Spec.Triggers["secret-trigger"].Password,
		maskedFunctionConfig.Spec.Triggers["secret-trigger"].Password)
	suite.Require().NotEqual(functionConfig.Spec.Triggers["secret-trigger"].Attributes["password"],
		maskedFunctionConfig.Spec.Triggers["secret-trigger"].Attributes["password"])
	suite.Require().Equal(functionConfig.Spec.Triggers["non-secret-trigger"].Attributes["not-a-password"],
		maskedFunctionConfig.Spec.Triggers["non-secret-trigger"].Attributes["not-a-password"])
	suite.Require().NotEqual(functionConfig.Spec.Volumes[0].Volume.VolumeSource.FlexVolume.Options["accesskey"],
		maskedFunctionConfig.Spec.Volumes[0].Volume.VolumeSource.FlexVolume.Options["accesskey"])

	restoredFunctionConfig, err := RestoreSensitiveDataInFunctionConfig(maskedFunctionConfig, secretMap)
	suite.Require().NoError(err)

	suite.logger.DebugWith("Restored function config", "functionConfig", restoredFunctionConfig)
	suite.Require().Equal(functionConfig, restoredFunctionConfig)
}

func (suite *MaskTestSuite) TestScrubWithExistingSecrets() {
	existingSecrets := map[string]string{
		"$ref:/Spec/Build/CodeEntryAttributes/password": "abcd",
	}

	functionConfig := &functionconfig.Config{
		Spec: functionconfig.Spec{
			Build: functionconfig.Build{
				CodeEntryAttributes: map[string]interface{}{

					// should be masked
					"password": "$ref:/Spec/Build/CodeEntryAttributes/password",
				},

				// should not be masked
				Image: "some-image:latest",
			},
			Triggers: map[string]functionconfig.Trigger{
				"secret-trigger": {
					Attributes: map[string]interface{}{
						"password": "1234",
					},
					Password: "4567",
				},
			},
		},
	}

	// mask the function config
	maskedFunctionConfig, secretMap, err := ScrubSensitiveDataInFunctionConfig(functionConfig, existingSecrets)
	suite.Require().NoError(err)
	suite.logger.DebugWith("Masked function config", "maskedFunctionConfig", maskedFunctionConfig, "secretMap", secretMap)

	suite.Require().Less(len(existingSecrets), len(secretMap))
	suite.Require().NotEqual(functionConfig.Spec.Triggers["secret-trigger"].Password,
		maskedFunctionConfig.Spec.Triggers["secret-trigger"].Password)
	suite.Require().NotEqual(functionConfig.Spec.Triggers["secret-trigger"].Attributes["password"],
		maskedFunctionConfig.Spec.Triggers["secret-trigger"].Attributes["password"])
	suite.Require().Contains(secretMap, maskedFunctionConfig.Spec.Triggers["secret-trigger"].Attributes["password"])
	suite.Require().Equal(functionConfig.Spec.Build.CodeEntryAttributes["password"],
		maskedFunctionConfig.Spec.Build.CodeEntryAttributes["password"])

	// test error cases:
	// existing secret map is nil
	_, _, err = ScrubSensitiveDataInFunctionConfig(functionConfig, nil)
	suite.Require().Error(err)

	// existing secret map doesn't contain the secret
	_, _, err = ScrubSensitiveDataInFunctionConfig(functionConfig, map[string]string{
		"$ref:/Spec/Something/Else/password": "abcd",
	})
	suite.Require().Error(err)
}

func TestMaskTestSuite(t *testing.T) {
	suite.Run(t, new(MaskTestSuite))
}
