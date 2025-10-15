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

package ingress

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/common"
	commonHeaders "github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type IngressTestSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *IngressTestSuite) SetupSuite() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("test-ingress")
	suite.Require().NoError(err)
}

func (suite *IngressTestSuite) TestCompileSSOAuthAnnotations() {
	tests := []struct {
		name                  string
		spec                  Spec
		platformConfiguration *platformconfig.Config
		expectedAnnotations   map[string]string
		expectedErr           string
	}{
		{
			name: "Take URLs from platform config",
			platformConfiguration: &platformconfig.Config{
				IngressConfig: platformconfig.IngressConfig{
					IguazioAuthURL:   "test-auth-url",
					IguazioSignInURL: "test-sign-in-url",
				},
			},
			expectedAnnotations: map[string]string{
				common.AnnotationNginxAuthURL:    "test-auth-url",
				common.AnnotationNginxAuthSignIn: "test-sign-in-url",
			},
		},
		{
			name: "Take URLs from spec",
			spec: Spec{
				Authentication: &Authentication{
					SSOAuth: &SSOAuth{
						AuthURL:  "spec-auth-url",
						LoginURL: "spec-login-url",
					},
				},
			},
			platformConfiguration: &platformconfig.Config{
				IngressConfig: platformconfig.IngressConfig{
					IguazioAuthURL:   "conf-auth-url",
					IguazioSignInURL: "conf-sign-in-url",
				},
			},
			expectedAnnotations: map[string]string{
				common.AnnotationNginxAuthURL:    "spec-auth-url",
				common.AnnotationNginxAuthSignIn: "spec-login-url",
			},
		},
		{
			name: "missing AuthURL in conf and spec",
			platformConfiguration: &platformconfig.Config{
				IngressConfig: platformconfig.IngressConfig{
					IguazioSignInURL: "conf-sign-in-url",
				},
			},
			expectedAnnotations: map[string]string{},
			expectedErr:         "No SSO auth URL configured",
		},
		{
			name: "missing LoginURL in conf and spec",
			spec: Spec{
				Authentication: &Authentication{
					SSOAuth: &SSOAuth{
						AuthURL: "spec-auth-url",
					},
				},
			},
			platformConfiguration: &platformconfig.Config{
				IngressConfig: platformconfig.IngressConfig{
					IguazioAuthURL: "conf-auth-url",
				},
			},
			expectedAnnotations: map[string]string{},
			expectedErr:         "No SSO login URL configured",
		},
		{
			name: "missing LoginURL and autURL in conf and exist in spec",
			spec: Spec{
				Authentication: &Authentication{
					SSOAuth: &SSOAuth{
						AuthURL:  "spec-auth-url",
						LoginURL: "spec-login-url",
					},
				},
			},
			platformConfiguration: &platformconfig.Config{
				IngressConfig: platformconfig.IngressConfig{},
			},
			expectedAnnotations: map[string]string{
				common.AnnotationNginxAuthURL:    "spec-auth-url",
				common.AnnotationNginxAuthSignIn: "spec-login-url",
			},
		},
		{
			name: "mixed- take LoginURL from spec and autURL from conf",
			spec: Spec{
				Authentication: &Authentication{
					SSOAuth: &SSOAuth{
						LoginURL: "spec-login-url",
					},
				},
			},
			platformConfiguration: &platformconfig.Config{
				IngressConfig: platformconfig.IngressConfig{
					IguazioAuthURL:   "conf-auth-url",
					IguazioSignInURL: "conf-sign-in-url",
				},
			},
			expectedAnnotations: map[string]string{
				common.AnnotationNginxAuthURL:    "conf-auth-url",
				common.AnnotationNginxAuthSignIn: "spec-login-url",
			},
		},
	}

	for _, testCase := range tests {
		suite.Run(testCase.name, func() {
			testManager, err := NewManager(suite.logger, nil, nil, testCase.platformConfiguration)
			suite.Require().NoError(err)
			suite.enrichExpectedAnnotations(testCase.expectedAnnotations)
			result, err := testManager.compileSSOAuthAnnotations(testCase.spec)
			if testCase.expectedErr != "" {
				suite.Require().Error(err)
				suite.Require().Contains(err.Error(), testCase.expectedErr)
				suite.Require().Nil(result)
			} else {
				suite.Require().NoError(err)
				suite.Require().NotNil(result)
				suite.validateAnnotations(result, testCase.expectedAnnotations)
			}
		})
	}
}

func (suite *IngressTestSuite) enrichExpectedAnnotations(annotations map[string]string) {
	annotations[common.AnnotationNginxAuthResponseHeaders] = commonHeaders.AuthorizationHeader
	annotations[common.AnnotationNginxProxyBodySize] = common.NginxDefaultProxyBodySize
	annotations[common.AnnotationNginxProxyBufferSize] = common.NginxDefaultProxyBufferSize
	annotations[common.AnnotationNginxServiceUpstream] = common.NginxDefaultServiceUpstream
	annotations[common.AnnotationNginxSSLRedirect] = common.NginxDefaultSSLRedirect
}

func (suite *IngressTestSuite) validateAnnotations(result map[string]string, expected map[string]string) {
	suite.Require().Equal(len(result), len(expected))
	for annotationKey, annotationValue := range result {
		expectedValue, ok := expected[annotationKey]
		suite.Require().True(ok, "Unexpected annotation key: %s", annotationKey)
		suite.Require().Equal(expectedValue, annotationValue,
			"Unexpected annotation value for key: %s. expected: %s, got: %s", annotationKey, expectedValue, annotationValue)
	}
}

func TestLeaderTestSuite(t *testing.T) {
	suite.Run(t, new(IngressTestSuite))
}
