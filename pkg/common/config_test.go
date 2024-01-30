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

func (cts *ConfigTestSuite) TestMergeEnvSlices() {
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
		cts.Run(testCase.name, func() {
			mergedEnvs := MergeEnvSlices(testCase.primaryEnvs, testCase.secondaryEnvs)

			// check that slices are of the same length
			cts.Require().Equal(len(testCase.expectedMergedEnvs), len(mergedEnvs))

			// since order can be different, check that each element of the expected list is in the actual slice
			for _, envVar := range mergedEnvs {
				expectedEnvVarValue := testCase.expectedMergedEnvs[envVar.Name]
				cts.Require().Equal(expectedEnvVarValue, envVar.Value)
			}
		})
	}
}

func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}
