//go:build test_unit

/*
Copyright 2025 Iguazio Systems Ltd.

Licensed under the Apache License, Version 2.0 (the "License") with
an addition restriction as set forth herein. You may not use this
file except in compliance with the License. You may obtain a copy of
the License at http://www.apache.org/licenses/LICENSE-2.0.

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
implied. See the License for the specific language governing
permissions and limitations under the License.

In addition, you may not use the software for any purposes that are
illegal under applicable law, and the grant of the foregoing license
under the Apache 2.0 license is conditioned upon your compliance with
such restriction.
*/

package resourcescaler

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/common"

	"github.com/stretchr/testify/suite"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ResourceScalerTestSuite struct {
	suite.Suite
}

func (suite *ResourceScalerTestSuite) TestGetResolveTargetsFromIngressCallback() {
	for _, testCase := range []struct {
		name           string
		ingress        *networkingv1.Ingress
		expectedResult []string
		expectError    bool
		errorMsg       string
	}{
		{
			name: "Labels with single function",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						common.NuclioResourceLabelKeyFunctionName: "func1",
					},
				},
			},
			expectedResult: []string{"func1"},
		},
		{
			name: "Labels with canary function",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						common.NuclioResourceLabelKeyFunctionName:       "test-target1",
						common.NuclioResourceLabelKeyCanaryFunctionName: "test-target2",
					},
				},
			},
			expectedResult: []string{"test-target1", "test-target2"},
		},
		{
			name: "Annotation with targets",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						common.NginxConfigurationSnippetAnnotationKey: `proxy_set_header X-Nuclio-Target "test6,test7";`,
					},
				},
			},
			expectError: true,
			errorMsg:    "Failed to resolve ingress targets",
		},
		{
			name: "No labels or annotation",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{},
			},
			expectError: true,
			errorMsg:    "Failed to resolve ingress targets",
		},
		{
			name:        "Nil ingress",
			ingress:     nil,
			expectError: true,
			errorMsg:    "Ingress is nil",
		},
	} {
		suite.Run(testCase.name, func() {
			res, err := ResolveTargetsFromIngressCallback(testCase.ingress)
			if testCase.expectError {
				suite.Require().Error(err)
				suite.Require().Contains(err.Error(), testCase.errorMsg)
			} else {
				suite.Require().NoError(err)
				suite.Require().Equal(testCase.expectedResult, res)
			}
		})
	}
}

func TestResourceScalerTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceScalerTestSuite))
}
