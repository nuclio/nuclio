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

package functionconfig

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/nuclio/nuclio/pkg/common/headers"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

type ScrubberTestSuite struct {
	suite.Suite
	logger       logger.Logger
	ctx          context.Context
	k8sClientSet *k8sfake.Clientset
	scrubber     *Scrubber
}

func (suite *ScrubberTestSuite) SetupTest() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	suite.ctx = context.Background()
	suite.k8sClientSet = k8sfake.NewSimpleClientset()
	suite.scrubber = NewScrubber(suite.getSensitiveFieldsPathsRegex(), suite.k8sClientSet)
}

func (suite *ScrubberTestSuite) TestScrubBasics() {
	functionConfig := &Config{
		Spec: Spec{
			Build: Build{
				CodeEntryAttributes: map[string]interface{}{

					// should be scrubbed
					"password":          "abcd",
					"s3SecretAccessKey": "some-s3-secret",
					"s3SessionToken":    "some-s3-session-token",
					"headers": map[string]interface{}{
						"Authorization":        "token 1234abcd5678",
						headers.V3IOSessionKey: "some-session-key",
					},

					// should not be scrubbed
					"workDir": "/path/to/test-func",
				},

				// should not be scrubbed
				Image: "some-image:latest",
			},
			Triggers: map[string]Trigger{
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
			Volumes: []Volume{
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

	// scrub the function config
	scrubbedInterface, secretMap, err := suite.scrubber.Scrub(functionConfig, nil, suite.getSensitiveFieldsPathsRegex())
	scrubbedFunctionConfig := GetFunctionConfigFromInterface(scrubbedInterface)
	suite.Require().NoError(err)

	suite.logger.DebugWith("Scrubbed function config", "functionConfig", scrubbedFunctionConfig, "secretMap", secretMap)

	suite.Require().NotEmpty(secretMap)

	// validate code entry attributes
	for _, attribute := range []string{"password", "s3SecretAccessKey", "s3SessionToken"} {
		suite.Require().NotEqual(functionConfig.Spec.Build.CodeEntryAttributes[attribute],
			scrubbedFunctionConfig.Spec.Build.CodeEntryAttributes[attribute])
		suite.Require().Contains(scrubbedFunctionConfig.Spec.Build.CodeEntryAttributes[attribute], ReferencePrefix)
	}

	suite.Require().Equal(functionConfig.Spec.Build.Image, scrubbedFunctionConfig.Spec.Build.Image)
	suite.Require().NotEqual(functionConfig.Spec.Triggers["secret-trigger"].Password,
		scrubbedFunctionConfig.Spec.Triggers["secret-trigger"].Password)
	suite.Require().NotEqual(functionConfig.Spec.Triggers["secret-trigger"].Attributes["password"],
		scrubbedFunctionConfig.Spec.Triggers["secret-trigger"].Attributes["password"])
	suite.Require().Equal(functionConfig.Spec.Triggers["non-secret-trigger"].Attributes["not-a-password"],
		scrubbedFunctionConfig.Spec.Triggers["non-secret-trigger"].Attributes["not-a-password"])
	suite.Require().NotEqual(functionConfig.Spec.Volumes[0].Volume.VolumeSource.FlexVolume.Options["accesskey"],
		scrubbedFunctionConfig.Spec.Volumes[0].Volume.VolumeSource.FlexVolume.Options["accesskey"])

	restoredFunctionConfig, err := suite.scrubber.Restore(scrubbedFunctionConfig, secretMap)
	suite.Require().NoError(err)

	suite.logger.DebugWith("Restored function config", "functionConfig", restoredFunctionConfig)
	suite.Require().Equal(functionConfig, restoredFunctionConfig)
}

func (suite *ScrubberTestSuite) TestScrubWithExistingSecrets() {
	existingSecrets := map[string]string{
		"$ref:/spec/build/codeentryattributes/password": "abcd",
	}

	functionConfig := &Config{
		Spec: Spec{
			Build: Build{
				CodeEntryAttributes: map[string]interface{}{

					// should be scrubbed
					"password": "$ref:/Spec/Build/CodeEntryAttributes/password",
				},

				// should not be scrubbed
				Image: "some-image:latest",
			},
			Triggers: map[string]Trigger{
				"secret-trigger": {
					Attributes: map[string]interface{}{
						"password": "1234",
					},
					Password: "4567",
				},
			},
		},
	}

	// scrub the function config
	scrubbedInterface, secretMap, err := suite.scrubber.Scrub(functionConfig,
		existingSecrets,
		suite.getSensitiveFieldsPathsRegex())
	scrubbedFunctionConfig := scrubbedInterface.(*Config)
	suite.Require().NoError(err)
	suite.logger.DebugWith("Scrubbed function config", "scrubbedFunctionConfig", scrubbedFunctionConfig, "secretMap", secretMap)

	suite.Require().Less(len(existingSecrets), len(secretMap))
	suite.Require().NotEqual(functionConfig.Spec.Triggers["secret-trigger"].Password,
		scrubbedFunctionConfig.Spec.Triggers["secret-trigger"].Password)
	suite.Require().NotEqual(functionConfig.Spec.Triggers["secret-trigger"].Attributes["password"],
		scrubbedFunctionConfig.Spec.Triggers["secret-trigger"].Attributes["password"])
	suite.Require().Contains(secretMap, scrubbedFunctionConfig.Spec.Triggers["secret-trigger"].Attributes["password"])
	suite.Require().Equal(functionConfig.Spec.Build.CodeEntryAttributes["password"],
		scrubbedFunctionConfig.Spec.Build.CodeEntryAttributes["password"])

	// test error cases:
	// existing secret map is nil
	_, _, err = suite.scrubber.Scrub(functionConfig, nil, suite.getSensitiveFieldsPathsRegex())
	suite.Require().Error(err)

	// existing secret map doesn't contain the secret
	_, _, err = suite.scrubber.Scrub(functionConfig, map[string]string{
		"$ref:/Spec/Something/Else/password": "abcd",
	}, suite.getSensitiveFieldsPathsRegex())
	suite.Require().Error(err)
}

func (suite *ScrubberTestSuite) TestEncodeAndDecodeSecretKeys() {
	fieldPath := "Spec/Build/CodeEntryAttributes/password"
	encodedFieldPath := suite.scrubber.EncodeSecretKey(fieldPath)
	suite.logger.DebugWith("Encoded field path", "fieldPath", fieldPath, "encodedFieldPath", encodedFieldPath)

	decodedFieldPath, err := suite.scrubber.DecodeSecretKey(encodedFieldPath)
	suite.Require().NoError(err)
	suite.Require().Equal(fieldPath, decodedFieldPath)
}

func (suite *ScrubberTestSuite) TestEncodeSecretsMap() {

	secretMap := map[string]string{
		"$ref:Spec/Build/CodeEntryAttributes/password":          "abcd",
		"$ref:Spec/Triggers/secret-trigger/Password":            "4567",
		"$ref:Spec/Triggers/secret-trigger/Attributes/password": "1234",
	}

	encodedSecretMap, err := suite.scrubber.EncodeSecretsMap(secretMap)
	suite.Require().NoError(err)
	suite.logger.DebugWith("Encoded secret map", "secretMap", secretMap, "encodedSecretMap", encodedSecretMap)

	suite.Require().Contains(encodedSecretMap, "content")
	suite.Require().NotEmpty(len(encodedSecretMap["content"]))
	for encodedKey, value := range encodedSecretMap {
		if encodedKey == SecretContentKey {
			continue
		}
		decodedKey, err := suite.scrubber.DecodeSecretKey(encodedKey)
		suite.Require().NoError(err)
		suite.Require().Equal(secretMap[decodedKey], value)
	}
}

func (suite *ScrubberTestSuite) TestDecodeSecretsMapContent() {

	functionConfig := &Config{
		Spec: Spec{
			Triggers: map[string]Trigger{
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
		},
	}

	// scrub the function config
	scrubbedFunctionConfig, secretMap, err := suite.scrubber.Scrub(functionConfig, nil, suite.getSensitiveFieldsPathsRegex())
	suite.Require().NoError(err)

	// encode the secret map
	encodedSecretMap, err := suite.scrubber.EncodeSecretsMap(secretMap)
	suite.Require().NoError(err)

	// get the encoded secret map content
	encodedSecretMapContent := encodedSecretMap[SecretContentKey]
	suite.Require().NotEmpty(encodedSecretMapContent)

	decodedSecretMap, err := suite.scrubber.DecodeSecretsMapContent(encodedSecretMapContent)
	suite.Require().NoError(err)

	// restore the function config
	restoredFunctionConfig, err := suite.scrubber.Restore(scrubbedFunctionConfig, decodedSecretMap)
	suite.Require().NoError(err)

	suite.logger.DebugWith("Restored function config", "functionConfig", restoredFunctionConfig)

	// verify that the restored function config is equal to the original function config
	suite.Require().Equal(functionConfig, restoredFunctionConfig)
}

func (suite *ScrubberTestSuite) TestHasScrubbedConfig() {

	for _, testCase := range []struct {
		name           string
		functionConfig *Config
		expectedResult bool
	}{
		{
			name: "ScrubbedConfig",
			functionConfig: &Config{
				Spec: Spec{
					Build: Build{
						CodeEntryAttributes: map[string]interface{}{

							"password": "$ref:/Spec/Build/CodeEntryAttributes/password",
						},
						Image: "some-image:latest",
					},
					Triggers: map[string]Trigger{
						"secret-trigger": {
							Attributes: map[string]interface{}{
								"password": "$ref:Spec/Triggers/secret-trigger/Attributes/password",
							},
							Password: "$ref:Spec/Triggers/secret-trigger/Password",
						},
						"non-secret-trigger": {
							Attributes: map[string]interface{}{
								"not-a-password": "4321",
							},
						},
					},
				},
			},
			expectedResult: true,
		},
		{
			name: "NotScrubbedConfig",
			functionConfig: &Config{
				Spec: Spec{
					Build: Build{
						CodeEntryAttributes: map[string]interface{}{

							"password": "1234",
						},
						Image: "some-image:latest",
					},
					Triggers: map[string]Trigger{
						"secret-trigger": {
							Attributes: map[string]interface{}{
								"password": "5678",
							},
							Password: "abcd",
						},
						"non-secret-trigger": {
							Attributes: map[string]interface{}{
								"not-a-password": "4321",
							},
						},
					},
				},
			},
			expectedResult: false,
		},
	} {
		suite.Run(testCase.name, func() {

			// check that the function config has scrubbed fields
			hasScrubbedConfig, err := suite.scrubber.HasScrubbedConfig(testCase.functionConfig, suite.getSensitiveFieldsPathsRegex())
			suite.Require().NoError(err)

			if testCase.expectedResult {
				suite.Require().True(hasScrubbedConfig)
			} else {
				suite.Require().False(hasScrubbedConfig)
			}
		})
	}
}

func (suite *ScrubberTestSuite) TestGenerateFunctionSecretName() {

	for _, testCase := range []struct {
		name                 string
		functionName         string
		volumeName           string
		expectedResultPrefix string
	}{
		// Function secret names
		{
			name:                 "FunctionSecret-Sanity",
			functionName:         "my-function",
			expectedResultPrefix: "nuclio-my-function",
		},
		{
			name:                 "FunctionSecret-FunctionNameWithTrailingDashes",
			functionName:         "my-function-_",
			expectedResultPrefix: "nuclio-my-function",
		},
		{
			name:                 "FunctionSecret-LongFunctionName",
			functionName:         "my-function-with-a-very-long-name-which-is-more-than-63-characters-long",
			expectedResultPrefix: "nuclio-my-function-with-a-very-long-name-which-is-more", // nolint: misspell
		},

		// Flex volume secret names
		{
			name:                 "VolumeSecret-Sanity",
			functionName:         "my-function",
			volumeName:           "my-volume",
			expectedResultPrefix: "nuclio-flexvolume-my-function-my-volume",
		},
		{
			name:                 "VolumeSecret-VolumeNameWithTrailingDashes",
			functionName:         "my-function",
			volumeName:           "my-volume----",
			expectedResultPrefix: "nuclio-flexvolume-my-function-my-volume",
		},
		{
			name:                 "VolumeSecret-LongFunctionName",
			functionName:         "my-function-with-a-very-long-name-which-is-more-than-63-characters-long",
			volumeName:           "my-volume",
			expectedResultPrefix: "nuclio-flexvolume-my-volume",
		},
		{
			name:                 "VolumeSecret-LongVolumeName",
			functionName:         "my-function",
			volumeName:           "my-volume-name-which-is-more-than-63-characters-long",
			expectedResultPrefix: "nuclio-flexvolume-my-volume-name-which-is-more-than-63",
		},
	} {
		suite.Run(testCase.name, func() {
			var secretName string
			if testCase.volumeName == "" {
				secretName = suite.scrubber.GenerateObjectSecretName(testCase.functionName)
			} else {
				secretName = suite.scrubber.GenerateFlexVolumeSecretName(testCase.functionName, testCase.volumeName)
			}
			suite.logger.DebugWith("Generated secret name", "secretName", secretName)
			suite.Require().True(strings.HasPrefix(secretName, testCase.expectedResultPrefix))
		})
	}
}

func (suite *ScrubberTestSuite) TestRestoreConfigWithResources() {

	config := &Config{
		Spec: Spec{
			Resources: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("200m"),
					v1.ResourceMemory: resource.MustParse("200Mi"),
				},
				Requests: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("100m"),
					v1.ResourceMemory: resource.MustParse("100Mi"),
				},
			},
		},
	}

	secretMap := map[string]string{}

	// restore the config
	restoredConfigInterface, err := suite.scrubber.Restore(config, secretMap)
	restoredFunctionConfig := GetFunctionConfigFromInterface(restoredConfigInterface)
	suite.Require().NoError(err)

	// check that the restored config has the same resources
	suite.Require().Equal(config.Spec.Resources, restoredFunctionConfig.Spec.Resources)
}

// getSensitiveFieldsRegex returns a list of regexes for sensitive fields paths
// this is implemented here to avoid a circular dependency between platformconfig and functionconfig
func (suite *ScrubberTestSuite) getSensitiveFieldsPathsRegex() []*regexp.Regexp {
	var regexpList []*regexp.Regexp
	for _, sensitiveFieldPath := range []string{

		// Path nested in a map
		"^/Spec/Build/CodeEntryAttributes/password$",
		"^/spec/build/codeentryattributes/password$",
		"^/spec/build/codeentryattributes/s3secretaccesskey$",
		"^/spec/build/codeentryattributes/s3sessiontoken$",
		"^/spec/build/codeentryattributes/headers/authorization$",
		"^/spec/build/codeentryattributes/headers/x-v3io-session-key$",
		// Path nested in an array
		"^/Spec/Volumes\\[\\d+\\]/Volume/VolumeSource/FlexVolume/Options/accesskey$",
		"^/Spec/Volumes\\[\\d+\\]/Volume/FlexVolume/Options/accesskey$",
		// Path for any map element
		"^/Spec/Triggers/.+/Password$",
		// Nested path in any map element
		"^/Spec/Triggers/.+/Attributes/password$",
	} {
		regexpList = append(regexpList, regexp.MustCompile("(?i)"+sensitiveFieldPath))
	}
	return regexpList
}

func TestScrubberTestSuite(t *testing.T) {
	suite.Run(t, new(ScrubberTestSuite))
}
