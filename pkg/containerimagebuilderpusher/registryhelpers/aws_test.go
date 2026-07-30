//go:build test_unit

/*
Copyright 2026 The Nuclio Authors.

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

package registryhelpers

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type AWSTestSuite struct {
	suite.Suite
}

func (suite *AWSTestSuite) TestECRRegistryID() {
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
			suite.Require().Equal(testCase.expected, ECRRegistryID(testCase.registryURL))
		})
	}
}

func (suite *AWSTestSuite) TestECRRegion() {
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
			suite.Require().Equal(testCase.expected, ECRRegion(testCase.registryURL))
		})
	}
}

func (suite *AWSTestSuite) TestIsECRHost() {
	for _, testCase := range []struct {
		name     string
		url      string
		expected bool
	}{
		{name: "ECRHost", url: "123456789012.dkr.ecr.us-east-1.amazonaws.com", expected: true},
		{name: "ArtifactoryHost", url: "system-registry.artifactory.example.com", expected: false},
		{name: "AzureHost", url: "myregistry.azurecr.io", expected: false},
	} {
		suite.Run(testCase.name, func() {
			suite.Require().Equal(testCase.expected, IsECRHost(testCase.url))
		})
	}
}

func TestAWSTestSuite(t *testing.T) {
	suite.Run(t, new(AWSTestSuite))
}
