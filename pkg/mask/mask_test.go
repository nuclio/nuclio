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
	"encoding/json"
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
	dummyFunctionConfig := &functionconfig.Config{
		Spec: functionconfig.Spec{
			Build: functionconfig.Build{
				CodeEntryAttributes: map[string]interface{}{

					// should be masked
					"password": "abcd",
				},

				// should not be masked
				Image: "some-image:latest",
				Commands: []string{
					"some-command-1",
					"some-command-2",
				},
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
	maskedFunctionConfig, secretMap, err := ScrubSensitiveDataInFunctionConfig(dummyFunctionConfig, nil)
	suite.Require().NoError(err)

	suite.logger.DebugWith("Masked function config", "functionConfig", maskedFunctionConfig, "secretMap", secretMap)

	suite.Require().Equal(dummyFunctionConfig.Spec.Build.Image, maskedFunctionConfig.Spec.Build.Image)
	suite.Require().NotEqual(dummyFunctionConfig.Spec.Build.CodeEntryAttributes["password"],
		maskedFunctionConfig.Spec.Build.CodeEntryAttributes["password"])
	suite.Require().NotEqual(dummyFunctionConfig.Spec.Triggers["secret-trigger"].Password,
		maskedFunctionConfig.Spec.Triggers["secret-trigger"].Password)
	suite.Require().NotEqual(dummyFunctionConfig.Spec.Triggers["secret-trigger"].Attributes["password"],
		maskedFunctionConfig.Spec.Triggers["secret-trigger"].Attributes["password"])
	suite.Require().Equal(dummyFunctionConfig.Spec.Triggers["non-secret-trigger"].Attributes["not-a-password"],
		maskedFunctionConfig.Spec.Triggers["non-secret-trigger"].Attributes["not-a-password"])
	suite.Require().NotEqual(dummyFunctionConfig.Spec.Volumes[0].Volume.VolumeSource.FlexVolume.Options["accesskey"],
		maskedFunctionConfig.Spec.Volumes[0].Volume.VolumeSource.FlexVolume.Options["accesskey"])

	restoredFunctionConfig, err := RestoreSensitiveDataInFunctionConfig(maskedFunctionConfig, secretMap)
	suite.Require().NoError(err)

	suite.logger.DebugWith("Restored function config", "functionConfig", restoredFunctionConfig)
	suite.Require().Equal(dummyFunctionConfig, restoredFunctionConfig)
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

	suite.Require().NotEqual(existingSecrets, secretMap)
	suite.Require().NotEqual(functionConfig.Spec.Triggers["secret-trigger"].Password,
		maskedFunctionConfig.Spec.Triggers["secret-trigger"].Password)
	suite.Require().NotEqual(functionConfig.Spec.Triggers["secret-trigger"].Attributes["password"],
		maskedFunctionConfig.Spec.Triggers["secret-trigger"].Attributes["password"])
	suite.Require().Equal(functionConfig.Spec.Build.CodeEntryAttributes["password"],
		maskedFunctionConfig.Spec.Build.CodeEntryAttributes["password"])

	// test error case
	maskedFunctionConfig, secretMap, err = ScrubSensitiveDataInFunctionConfig(functionConfig, nil)
	suite.Require().Error(err)
	suite.logger.DebugWith("Masked function config", "maskedFunctionConfig", maskedFunctionConfig, "secretMap", secretMap, "err", err.Error())
}

func (suite *MaskTestSuite) TestMaskSecrets() {
	secretMap := map[string]string{
		"secret1": "value1",
		"secret2": "value2",
	}

	marshaledSecretMap, err := json.Marshal(secretMap)
	suite.Require().NoError(err)
	suite.logger.DebugWith("Marshalled secret map", "marshaledSecretMap", string(marshaledSecretMap))
	var restoredSecretMap map[string]string
	err = json.Unmarshal(marshaledSecretMap, &restoredSecretMap)
	suite.Require().NoError(err)

	suite.logger.DebugWith("Unmarshalled secret map", "restoredSecretMap", restoredSecretMap)
	suite.Require().Equal(secretMap, restoredSecretMap)

}

func TestMaskTestSuite(t *testing.T) {
	suite.Run(t, new(MaskTestSuite))
}
