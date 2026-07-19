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

package containerimagebuilderpusher

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type KanikoTestSuite struct {
	suite.Suite
	kaniko *Kaniko
}

func (suite *KanikoTestSuite) TestNewContainerBuilderConfigurationParsesKanikoPodLabels() {
	for _, testCase := range []struct {
		name      string
		envValue  string
		expected  map[string]string
		expectErr bool
	}{
		{
			name:     "Unset",
			envValue: "",
			expected: nil,
		},
		{
			name:     "SingleLabel",
			envValue: `{"azure.workload.identity/use":"true"}`,
			expected: map[string]string{"azure.workload.identity/use": "true"},
		},
		{
			name:     "MultipleLabels",
			envValue: `{"a":"1","b":"2"}`,
			expected: map[string]string{"a": "1", "b": "2"},
		},
		{
			name:      "InvalidJSON",
			envValue:  "not-json",
			expectErr: true,
		},
	} {
		suite.Run(testCase.name, func() {
			if testCase.envValue != "" {
				suite.T().Setenv("NUCLIO_KANIKO_POD_LABELS", testCase.envValue)
			}

			config, err := NewContainerBuilderConfiguration()
			if testCase.expectErr {
				suite.Require().Error(err)
				return
			}
			suite.Require().NoError(err)
			suite.Equal(testCase.expected, config.KanikoPodLabels)
		})
	}
}

func (suite *KanikoTestSuite) SetupTest() {
	suite.kaniko = &Kaniko{jobRunner: &jobRunner{}}
}

func (suite *KanikoTestSuite) TestResolveAWSRegistryId() {
	for _, testCase := range []struct {
		name        string
		registryURL string
		expected    string
	}{
		{
			name:        "StandardECRURL",
			registryURL: "123456789012.dkr.ecr.us-east-1.amazonaws.com",
			expected:    "123456789012",
		},
		{
			name:        "DifferentRegion",
			registryURL: "987654321098.dkr.ecr.eu-west-1.amazonaws.com",
			expected:    "987654321098",
		},
		{
			name:        "APRegion",
			registryURL: "111222333444.dkr.ecr.ap-southeast-1.amazonaws.com",
			expected:    "111222333444",
		},
	} {
		suite.Run(testCase.name, func() {
			result := suite.kaniko.resolveAWSRegistryId(testCase.registryURL)
			suite.Require().Equal(testCase.expected, result)
		})
	}
}

func (suite *KanikoTestSuite) TestResolveAWSRegionFromECR() {
	for _, testCase := range []struct {
		name        string
		registryURL string
		expected    string
	}{
		{
			name:        "USEast1",
			registryURL: "123456789012.dkr.ecr.us-east-1.amazonaws.com",
			expected:    "us-east-1",
		},
		{
			name:        "EUWest1",
			registryURL: "987654321098.dkr.ecr.eu-west-1.amazonaws.com",
			expected:    "eu-west-1",
		},
		{
			name:        "APSoutheast1",
			registryURL: "111222333444.dkr.ecr.ap-southeast-1.amazonaws.com",
			expected:    "ap-southeast-1",
		},
	} {
		suite.Run(testCase.name, func() {
			result := suite.kaniko.resolveAWSRegionFromECR(testCase.registryURL)
			suite.Require().Equal(testCase.expected, result)
		})
	}
}

func TestKanikoTestSuite(t *testing.T) {
	suite.Run(t, new(KanikoTestSuite))
}
