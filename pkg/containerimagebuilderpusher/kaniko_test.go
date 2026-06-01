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

func TestNewContainerBuilderConfigurationParsesKanikoPodLabels(t *testing.T) {
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
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.envValue != "" {
				t.Setenv("NUCLIO_KANIKO_POD_LABELS", testCase.envValue)
			}

			config, err := NewContainerBuilderConfiguration()
			if testCase.expectErr {
				if err == nil {
					t.Fatalf("expected an error, got nil; config: %#v", config)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(config.KanikoPodLabels) != len(testCase.expected) {
				t.Fatalf("expected %d labels, got %d (%#v)",
					len(testCase.expected), len(config.KanikoPodLabels), config.KanikoPodLabels)
			}
			for key, expectedValue := range testCase.expected {
				if actual, ok := config.KanikoPodLabels[key]; !ok || actual != expectedValue {
					t.Fatalf("label %q: expected %q, got %q (present=%v)", key, expectedValue, actual, ok)
				}
			}
		})
	}
}

func TestResolveKanikoPodLabelsCopiesAndIsolatesFromConfig(t *testing.T) {
	configLabels := map[string]string{"azure.workload.identity/use": "true"}
	k := &Kaniko{
		builderConfiguration: &ContainerBuilderConfiguration{
			KanikoPodLabels: configLabels,
		},
	}

	resolved := k.resolveKanikoPodLabels()
	if got := resolved["azure.workload.identity/use"]; got != "true" {
		t.Fatalf("expected workload-identity label to be propagated, got %q", got)
	}

	// Mutating the returned map must not leak back into the shared config map.
	resolved["mutated"] = "yes"
	if _, leaked := configLabels["mutated"]; leaked {
		t.Fatalf("resolveKanikoPodLabels must return a copy; mutation leaked into builderConfiguration")
	}
}

func TestResolveKanikoPodLabelsReturnsNilWhenNoLabelsConfigured(t *testing.T) {
	k := &Kaniko{builderConfiguration: &ContainerBuilderConfiguration{}}
	if got := k.resolveKanikoPodLabels(); got != nil {
		t.Fatalf("expected nil when no labels are configured, got %#v", got)
	}
}

type KanikoTestSuite struct {
	suite.Suite
	kaniko *Kaniko
}

func (suite *KanikoTestSuite) SetupTest() {
	suite.kaniko = &Kaniko{}
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
