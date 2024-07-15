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

package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type KubeNuclioTestSuite struct {
	suite.Suite
}

func (suite *KubeNuclioTestSuite) TestEnrichNodeSelector() {
	for _, testCase := range []struct {
		name                         string
		functionNodeSelector         map[string]string
		platformNodeSelector         map[string]string
		projectNodeSelector          map[string]string
		expectedFunctionNodeSelector map[string]string
	}{
		{
			name:                         "all-selectors-empty",
			expectedFunctionNodeSelector: nil,
		},
		{
			name:                         "get-selector-from-platform",
			platformNodeSelector:         map[string]string{"test": "test"},
			expectedFunctionNodeSelector: map[string]string{"test": "test"},
		},
		{
			name: "get-selector-from-project",
			platformNodeSelector: map[string]string{
				"test":  "from-platform",
				"test2": "from-platform2",
			},
			projectNodeSelector: map[string]string{
				"test":  "from-project",
				"test1": "from-project1",
			},
			expectedFunctionNodeSelector: map[string]string{
				"test":  "from-project",
				"test1": "from-project1",
				"test2": "from-platform2",
			},
		},
		{
			name: "get-selector-from-project",
			platformNodeSelector: map[string]string{
				"test":  "from-platform",
				"test2": "from-platform2",
			},
			projectNodeSelector: map[string]string{
				"test":  "from-project",
				"test1": "from-project1",
			},
			functionNodeSelector: map[string]string{"test": "from-function"},
			expectedFunctionNodeSelector: map[string]string{
				"test":  "from-function",
				"test1": "from-project1",
				"test2": "from-platform2",
			},
		},
	} {
		suite.Run(testCase.name, func() {
			function := &NuclioFunction{}
			function.Spec.NodeSelector = testCase.functionNodeSelector
			function.EnrichNodeSelector(testCase.platformNodeSelector, testCase.projectNodeSelector)
			suite.Require().Equal(testCase.expectedFunctionNodeSelector, function.Status.EnrichedNodeSelector)

		})
	}
}

func TestKubePlatformTestSuite(t *testing.T) {
	suite.Run(t, new(KubeNuclioTestSuite))
}
