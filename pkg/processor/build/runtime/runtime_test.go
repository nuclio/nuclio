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

package runtime

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/common/testutils/mocks"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/stretchr/testify/mock"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type AbstractRuntimeTestSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *AbstractRuntimeTestSuite) SetupSuite() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)
}

func (suite *AbstractRuntimeTestSuite) TestGetBaseImageFromMap() {
	testCases := []struct {
		name              string
		baseImages        map[string]string
		runtime           string
		expectedBaseImage string
	}{
		{
			name: "Base image configured for runtime name only",
			baseImages: map[string]string{
				"node": "custom-node:20",
			},
			runtime:           "node:20",
			expectedBaseImage: "custom-node:20",
		},
		{
			name: "Base image configured for runtime name and version",
			baseImages: map[string]string{
				"python:3.12": "custom-python-3.12:latest",
			},
			runtime:           "python:3.12",
			expectedBaseImage: "custom-python-3.12:latest",
		},
		{
			name: "Version-specific override takes precedence over name-only",
			baseImages: map[string]string{
				"node":    "nodejs-base:latest",
				"node:20": "nodejs-20-specific:latest",
			},
			runtime:           "node:20",
			expectedBaseImage: "nodejs-20-specific:latest",
		},
		{
			name: "No matching base image - returns default",
			baseImages: map[string]string{
				"golang": "custom-golang:latest",
			},
			runtime:           "python:3.12",
			expectedBaseImage: "",
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			functionConfig := &functionconfig.Config{
				Spec: functionconfig.Spec{
					Runtime: testCase.runtime,
				},
			}
			testAbstractRuntime, err := NewAbstractRuntime(suite.logger, "nop", "/tmp", functionConfig)
			suite.Require().NoError(err)

			result := testAbstractRuntime.GetBaseImageFromMap(testCase.baseImages)

			suite.Require().Equal(testCase.expectedBaseImage, result)
		})
	}
}

func (suite *AbstractRuntimeTestSuite) TestGetBaseImageFromMapWithMockLogger() {
	testCases := []struct {
		name               string
		baseImages         map[string]string
		runtime            string
		expectedBaseImage  string
		expectWarnWithCall bool
	}{
		{
			name: "Base image found - no warning logged",
			baseImages: map[string]string{
				"python:3.12": "custom-python:3.12",
			},
			runtime:            "python:3.12",
			expectedBaseImage:  "custom-python:3.12",
			expectWarnWithCall: false,
		},
		{
			name: "Base image not found - warning logged",
			baseImages: map[string]string{
				"golang": "custom-golang:latest",
			},
			runtime:            "python:3.12",
			expectedBaseImage:  "",
			expectWarnWithCall: true,
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			mockLogger := new(mocks.MockLogger)

			if testCase.expectWarnWithCall {
				mockLogger.On("WarnWith", "Failed to find base image for runtime", mock.Anything).Once()
			}

			functionConfig := &functionconfig.Config{
				Spec: functionconfig.Spec{
					Runtime: testCase.runtime,
				},
			}

			testAbstractRuntime := &AbstractRuntime{
				Logger:         mockLogger,
				FunctionConfig: functionConfig,
			}

			result := testAbstractRuntime.GetBaseImageFromMap(testCase.baseImages)

			suite.Require().Equal(testCase.expectedBaseImage, result)
			mockLogger.AssertExpectations(suite.T())
		})
	}
}

func (suite *AbstractRuntimeTestSuite) TestGetOverrideImageRegistryFromMapWithMockLogger() {
	testCases := []struct {
		name                  string
		imageRegistries       map[string]string
		runtime               string
		expectedImageRegistry string
	}{
		{
			name: "Override image registry found - no warning logged",
			imageRegistries: map[string]string{
				"python:3.12": "custom-registry.io",
			},
			runtime:               "python:3.12",
			expectedImageRegistry: "custom-registry.io",
		},
		{
			name: "Override image registry not found - no warning logged",
			imageRegistries: map[string]string{},
			runtime:               "python:3.12",
			expectedImageRegistry: "",
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			mockLogger := new(mocks.MockLogger)
			functionConfig := &functionconfig.Config{
				Spec: functionconfig.Spec{
					Runtime: testCase.runtime,
				},
			}
			testAbstractRuntime := &AbstractRuntime{
				Logger:         mockLogger,
				FunctionConfig: functionConfig,
			}

			result := testAbstractRuntime.GetOverrideImageRegistryFromMap(testCase.imageRegistries)

			suite.Require().Equal(testCase.expectedImageRegistry, result)
			mockLogger.AssertExpectations(suite.T())
		})
	}
}

func TestAbstractRuntimeTestSuite(t *testing.T) {
	suite.Run(t, new(AbstractRuntimeTestSuite))
}
