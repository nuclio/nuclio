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

package common

import (
	"testing"

	"github.com/stretchr/testify/suite"
	v1 "k8s.io/api/core/v1"
)

type ConfigTestSuite struct {
	suite.Suite
}

func (suite *ConfigTestSuite) TestMergeEnvSlices() {
	for _, testCase := range []struct {
		name               string
		primaryEnvs        []v1.EnvVar
		secondaryEnvs      []v1.EnvVar
		expectedMergedEnvs map[string]string
	}{

		{
			name:               "same-key-different-value",
			primaryEnvs:        []v1.EnvVar{{Name: "test1", Value: "a"}, {Name: "test2", Value: "c"}},
			secondaryEnvs:      []v1.EnvVar{{Name: "test1", Value: "b"}, {Name: "test3", Value: "d"}},
			expectedMergedEnvs: map[string]string{"test1": "a", "test2": "c", "test3": "d"},
		},
		{
			name:               "empty-secondary",
			primaryEnvs:        []v1.EnvVar{{Name: "test1", Value: "a"}},
			expectedMergedEnvs: map[string]string{"test1": "a"},
		},
		{
			name:               "empty-primary",
			primaryEnvs:        []v1.EnvVar{{Name: "test1", Value: "a"}},
			expectedMergedEnvs: map[string]string{"test1": "a"},
		},
	} {
		suite.Run(testCase.name, func() {
			mergedEnvs := MergeEnvSlices(testCase.primaryEnvs, testCase.secondaryEnvs)

			// check that slices are of the same length
			suite.Require().Len(mergedEnvs, len(testCase.expectedMergedEnvs))

			// since order can be different, check that each element of the expected list is in the actual slice
			for _, envVar := range mergedEnvs {
				expectedEnvVarValue := testCase.expectedMergedEnvs[envVar.Name]
				suite.Require().Equal(expectedEnvVarValue, envVar.Value)
			}
		})
	}
}

const (
	testNum14 = int32(14)
	testNum17 = int32(17)
)

var testProbe17 = &v1.Probe{InitialDelaySeconds: testNum17, TimeoutSeconds: testNum17, PeriodSeconds: testNum17, FailureThreshold: testNum17}
var testProbe14 = &v1.Probe{InitialDelaySeconds: testNum14, TimeoutSeconds: testNum14, PeriodSeconds: testNum14, FailureThreshold: testNum14}

func (suite *ConfigTestSuite) TestEnrichDefaultReadinessProbe() {
	for _, testCase := range []struct {
		name           string
		probeConfig    *v1.Probe
		defaultProbe   *v1.Probe
		expectedResult *v1.Probe
	}{
		{
			name:           "enrich all ReadinessProbe in nil case",
			probeConfig:    nil,
			defaultProbe:   testProbe17,
			expectedResult: testProbe17,
		}, {
			name: "enrich defaults besides TimeoutSeconds",
			probeConfig: &v1.Probe{
				TimeoutSeconds: testNum14,
			},
			defaultProbe: testProbe17,
			expectedResult: &v1.Probe{
				InitialDelaySeconds: testNum17,
				TimeoutSeconds:      testNum14,
				PeriodSeconds:       testNum17,
				FailureThreshold:    testNum17,
			},
		}, {
			name:           "don't enrich anything",
			probeConfig:    testProbe17,
			defaultProbe:   testProbe14,
			expectedResult: testProbe17,
		},
	} {
		suite.Run(testCase.name, func() {
			EnrichReadinessProbe(&testCase.probeConfig, testCase.defaultProbe)
			suite.Require().NotEmpty(testCase.probeConfig)
			suite.Require().Equal(testCase.expectedResult, testCase.probeConfig)
		})
	}
}

func (suite *ConfigTestSuite) TestEnrichDefaultLivenessProbe() {
	for _, testCase := range []struct {
		name           string
		probeConfig    *v1.Probe
		defaultProbe   *v1.Probe
		expectedResult *v1.Probe
	}{
		{
			name:           "enrich all LivenessProbe in nil case",
			probeConfig:    nil,
			defaultProbe:   testProbe17,
			expectedResult: testProbe17,
		}, {
			name: "enrich defaults besides TimeoutSeconds",
			probeConfig: &v1.Probe{
				TimeoutSeconds: testNum14,
			},
			defaultProbe: testProbe17,
			expectedResult: &v1.Probe{
				InitialDelaySeconds: testNum17,
				TimeoutSeconds:      testNum14,
				PeriodSeconds:       testNum17,
			},
		}, {
			name:           "don't enrich anything",
			probeConfig:    testProbe17,
			defaultProbe:   testProbe14,
			expectedResult: testProbe17,
		},
	} {
		suite.Run(testCase.name, func() {
			EnrichLivenessProbe(&testCase.probeConfig, testCase.defaultProbe)
			suite.Require().NotEmpty(testCase.probeConfig)
			suite.Require().Equal(testCase.expectedResult, testCase.probeConfig)
		})
	}
}

func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}
