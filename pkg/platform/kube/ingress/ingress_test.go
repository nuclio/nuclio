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

	"github.com/nuclio/nuclio/pkg/common/annotations"
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

func (suite *IngressTestSuite) TestCompileIguazioAuthAnnotations() {
	tests := []struct {
		name                  string
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
				annotations.NginxAuthURL:    "test-auth-url",
				annotations.NginxAuthSignIn: "test-sign-in-url",
			},
		},
		{
			name: "missing AuthURL in conf",
			platformConfiguration: &platformconfig.Config{
				IngressConfig: platformconfig.IngressConfig{
					IguazioSignInURL: "conf-sign-in-url",
				},
			},
			expectedAnnotations: map[string]string{},
			expectedErr:         "No SSO auth URL configured",
		},
		{
			name: "missing LoginURL in conf",
			platformConfiguration: &platformconfig.Config{
				IngressConfig: platformconfig.IngressConfig{
					IguazioAuthURL: "conf-auth-url",
				},
			},
			expectedAnnotations: map[string]string{},
			expectedErr:         "No SSO login URL configured",
		},
	}

	for _, testCase := range tests {
		suite.Run(testCase.name, func() {
			testManager, err := NewManager(suite.logger, nil, nil, testCase.platformConfiguration)
			suite.Require().NoError(err)
			result, err := testManager.compileIguazioAuthAnnotations()
			if testCase.expectedErr != "" {
				suite.Require().Error(err)
				suite.Require().Contains(err.Error(), testCase.expectedErr)
				suite.Require().Nil(result)
			} else {
				suite.Require().NoError(err)
				suite.Require().NotNil(result)
				suite.validateAnnotations(result, suite.getExpectedAnnotations(testCase.expectedAnnotations))
			}
		})
	}
}

func (suite *IngressTestSuite) getExpectedAnnotations(testAnnotations map[string]string) map[string]string {
	iguazioAnnotations := annotations.GetIguazioAuthenticationModeAnnotations()
	iguazioAnnotations[annotations.NginxAuthURL] = testAnnotations[annotations.NginxAuthURL]
	iguazioAnnotations[annotations.NginxAuthSignIn] = testAnnotations[annotations.NginxAuthSignIn]

	return iguazioAnnotations
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
