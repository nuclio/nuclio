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
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

type SanitizeKubernetesNameTestSuite struct {
	suite.Suite
}

func (suite *SanitizeKubernetesNameTestSuite) TestSanitizeKubernetesName() {
	for _, testCase := range []struct {
		name            string
		prefix          string
		value           string
		forGenerateName bool
		expected        string
	}{
		{
			name:     "LowercasesAndReplacesInvalidChars",
			value:    "Registry.Example.Com/My_Func:v1.2",
			expected: "registry-example-com-my-func-v1-2",
		},
		{
			name:     "RunOfInvalidCharsBecomesOneDash",
			value:    "a...b",
			expected: "a-b",
		},
		{
			name:     "LeadingAndTrailingDashesTrimmed",
			value:    "--a-b--",
			expected: "a-b",
		},
		{
			name:     "PrefixUsedVerbatim",
			prefix:   "registry-login-azure-",
			value:    "myregistry.azurecr.io",
			expected: "registry-login-azure-myregistry-azurecr-io",
		},
		{
			name:     "EmptyValueLeavesNoDanglingDash",
			prefix:   "registry-login-aws-",
			value:    "///:::",
			expected: "registry-login-aws",
		},
		{
			name:     "TruncatedToLabelLimit",
			prefix:   "registry-login-aws-",
			value:    strings.Repeat("a", 80),
			expected: "registry-login-aws-" + strings.Repeat("a", KubernetesDomainLevelMaxLength-len("registry-login-aws-")),
		},
		{
			name:            "GenerateNameAppendsDashAndReservesSuffix",
			prefix:          "nuclio-buildjob-",
			value:           "my-func:latest",
			forGenerateName: true,
			expected:        "nuclio-buildjob-my-func-latest-",
		},
		{
			name:            "GenerateNameTruncatesLeavingRoomForSuffix",
			prefix:          "nuclio-buildjob-",
			value:           strings.Repeat("x", 80),
			forGenerateName: true,
			expected:        "nuclio-buildjob-" + strings.Repeat("x", 41) + "-",
		},
		{
			name:            "GenerateNameTruncationLandingOnDashIsTrimmed",
			prefix:          "nuclio-buildjob-",
			value:           strings.Repeat("a", 41) + "." + strings.Repeat("b", 40),
			forGenerateName: true,
			expected:        "nuclio-buildjob-" + strings.Repeat("a", 41) + "-",
		},
	} {
		suite.Run(testCase.name, func() {
			result, err := SanitizeKubernetesName(testCase.prefix, testCase.value, testCase.forGenerateName)

			suite.Require().NoError(err)
			suite.Equal(testCase.expected, result)

			// the result must always fit a Kubernetes name, including the generated random suffix
			totalLength := len(result)
			if testCase.forGenerateName {
				totalLength += generateNameRandomSuffixLength
			}
			suite.LessOrEqual(totalLength, KubernetesDomainLevelMaxLength)
		})
	}
}

func (suite *SanitizeKubernetesNameTestSuite) TestSanitizeKubernetesNameErrorsOnInvalidPrefix() {
	for _, prefix := range []string{"Nuclio-", "nuclio_", "nuclio/", "nuclio ", "has.dot-", "-nuclio-"} {
		_, err := SanitizeKubernetesName(prefix, "value", false)
		suite.Require().Error(err, "prefix %q should be rejected", prefix)
	}
}

func (suite *SanitizeKubernetesNameTestSuite) TestSanitizeKubernetesNameAcceptsValidPrefix() {
	for _, prefix := range []string{"", "nuclio-", "a-b-c-", "abc123"} {
		_, err := SanitizeKubernetesName(prefix, "value", false)
		suite.Require().NoError(err, "prefix %q should be accepted", prefix)
	}
}

func TestK8sTestSuite(t *testing.T) {
	suite.Run(t, new(SanitizeKubernetesNameTestSuite))
}
