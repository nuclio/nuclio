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
	"k8s.io/api/core/v1"
)

type k8sTestSuite struct {
	suite.Suite
}

func (suite *k8sTestSuite) TestFilterInvalidLabels() {
	invalidLabels := map[string]string{
		"my@weird/label":   "value",
		"my.wierd/label":   "value@",
		"%weird+/label":    "v8$alue",
		"email.like@label": "value",
	}

	labels := map[string]string{
		"valid":          "label",
		"another-valid":  "label-value",
		"also_123_valid": "label_456_value",
	}

	// add invalid labels to labels
	for key, value := range invalidLabels {
		labels[key] = value
	}

	filteredLabels := FilterInvalidLabels(labels)

	suite.Require().Equal(len(filteredLabels), len(labels)-len(invalidLabels))
	for key := range invalidLabels {
		_, ok := filteredLabels[key]
		suite.Require().False(ok, "invalid label %s should not be in filtered labels", key)
	}
}

func (suite *k8sTestSuite) TestEnrichDefaultReadinessProbe() {
	testNum14 := int32(14)
	testNum17 := int32(17)
	for _, testCase := range []struct {
		name           string
		probeConfig    *v1.Probe
		defaultProbe   *v1.Probe
		expectedResult *v1.Probe
	}{
		{
			name:           "enrich all ReadinessProbe in nil case",
			probeConfig:    nil,
			defaultProbe:   suite.newTestProbe(testNum17),
			expectedResult: suite.newTestProbe(testNum17),
		}, {
			name: "enrich defaults besides TimeoutSeconds",
			probeConfig: &v1.Probe{
				TimeoutSeconds: testNum14,
			},
			defaultProbe: suite.newTestProbe(testNum17),
			expectedResult: &v1.Probe{
				InitialDelaySeconds: testNum17,
				TimeoutSeconds:      testNum14,
				PeriodSeconds:       testNum17,
				FailureThreshold:    testNum17,
			},
		}, {
			name:           "don't enrich anything",
			probeConfig:    suite.newTestProbe(testNum17),
			defaultProbe:   suite.newTestProbe(testNum14),
			expectedResult: suite.newTestProbe(testNum17),
		},
	} {
		suite.Run(testCase.name, func() {
			EnrichProbe(&testCase.probeConfig, testCase.defaultProbe)
			suite.Require().NotEmpty(testCase.probeConfig)
			suite.Require().Equal(testCase.expectedResult, testCase.probeConfig)
		})
	}
}

func (suite *k8sTestSuite) TestEnrichDefaultLivenessProbe() {
	testNum14 := int32(14)
	testNum17 := int32(17)
	for _, testCase := range []struct {
		name           string
		probeConfig    *v1.Probe
		defaultProbe   *v1.Probe
		expectedResult *v1.Probe
	}{
		{
			name:           "enrich all LivenessProbe in nil case",
			probeConfig:    nil,
			defaultProbe:   suite.newTestProbe(testNum17),
			expectedResult: suite.newTestProbe(testNum17),
		}, {
			name: "enrich defaults besides TimeoutSeconds",
			probeConfig: &v1.Probe{
				TimeoutSeconds: testNum14,
			},
			defaultProbe: suite.newTestProbe(testNum17),
			expectedResult: &v1.Probe{
				InitialDelaySeconds: testNum17,
				TimeoutSeconds:      testNum14,
				PeriodSeconds:       testNum17,
				FailureThreshold:    testNum17,
			},
		}, {
			name:           "don't enrich anything",
			probeConfig:    suite.newTestProbe(testNum17),
			defaultProbe:   suite.newTestProbe(testNum14),
			expectedResult: suite.newTestProbe(testNum17),
		},
	} {
		suite.Run(testCase.name, func() {
			EnrichProbe(&testCase.probeConfig, testCase.defaultProbe)
			suite.Require().NotEmpty(testCase.probeConfig)
			suite.Require().Equal(testCase.expectedResult, testCase.probeConfig)
		})
	}
}

func (suite *k8sTestSuite) newTestProbe(value int32) *v1.Probe {
	return &v1.Probe{
		InitialDelaySeconds: value,
		TimeoutSeconds:      value,
		PeriodSeconds:       value,
		FailureThreshold:    value,
	}
}

func TestK8sTestSuite(t *testing.T) {
	suite.Run(t, new(k8sTestSuite))
}
