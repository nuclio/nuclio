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

package iguazio

import (
	"context"
	"net/http"
	"testing"

	authpkg "github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/common/headers"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type AbstractAuthTestSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *AbstractAuthTestSuite) SetupTest() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("iguazio-auth")
	suite.Require().NoError(err)
}

func (suite *AbstractAuthTestSuite) TestBuildIdentityRequest() {
	authConfig := authpkg.NewConfig(authpkg.KindIguazio)
	authConfig.Iguazio.VerificationURL = "http://somewhere.local/identity/self"
	authConfig.Iguazio.VerificationMethod = http.MethodPost

	// Ensure authConfig is valid
	suite.Require().NotNil(authConfig)
	suite.Require().NotEmpty(authConfig.Iguazio.VerificationURL)

	// Initialize authInstance
	authInstance := NewAbstractAuth(suite.logger, authConfig, nil)
	tests := []struct {
		name           string
		authParams     *AuthParameters
		expectedMethod string
		expectedURL    string
		expectedHeader map[string]string
	}{
		{
			name: "WithAuthHeaderAndCookies",
			authParams: NewAuthParameters(
				context.Background(),
				"Bearer test-token",
				"_oauth2_proxy=session-cookie",
				"http://somewhere.local/identity/self",
				[32]byte{},
			),
			expectedMethod: http.MethodPost,
			expectedURL:    "http://somewhere.local/identity/self",
			expectedHeader: map[string]string{
				headers.AuthorizationHeader: "Bearer test-token",
				"Cookie":                    "_oauth2_proxy=session-cookie",
			},
		},
		{
			name: "WithAuthHeaderOnly",
			authParams: NewAuthParameters(
				context.Background(),
				"Bearer test-token",
				"",
				"http://somewhere.local/identity/self",
				[32]byte{},
			),
			expectedMethod: http.MethodPost,
			expectedURL:    "http://somewhere.local/identity/self",
			expectedHeader: map[string]string{
				headers.AuthorizationHeader: "Bearer test-token",
			},
		},
		{
			name: "WithCookiesOnly",
			authParams: NewAuthParameters(
				context.Background(),
				"",
				"_oauth2_proxy=session-cookie",
				"http://somewhere.local/identity/self",
				[32]byte{},
			),
			expectedMethod: http.MethodPost,
			expectedURL:    "http://somewhere.local/identity/self",
			expectedHeader: map[string]string{
				"Cookie": "_oauth2_proxy=session-cookie",
			},
		},
		{
			name: "WithoutAuthHeaderAndCookies",
			authParams: NewAuthParameters(
				context.Background(),
				"",
				"",
				"http://somewhere.local/identity/self",
				[32]byte{},
			),
			expectedMethod: http.MethodPost,
			expectedURL:    "http://somewhere.local/identity/self",
			expectedHeader: map[string]string{},
		},
		{
			name: "WithMultipleCookies",
			authParams: NewAuthParameters(
				context.Background(),
				"Bearer test-token",
				"_oauth2_proxy=oauth2-cookie-value; session=session-cookie-value",
				"http://somewhere.local/identity/self",
				[32]byte{},
			),
			expectedMethod: http.MethodPost,
			expectedURL:    "http://somewhere.local/identity/self",
			expectedHeader: map[string]string{
				headers.AuthorizationHeader: "Bearer test-token",
				"Cookie":                    "_oauth2_proxy=oauth2-cookie-value; session=session-cookie-value",
			},
		},
	}

	for _, testCase := range tests {
		suite.Run(testCase.name, func() {
			// Call buildIdentityRequest and check for errors
			req, err := authInstance.buildIdentityRequest(testCase.authParams)
			suite.Require().NoError(err)
			suite.Require().NotNil(req)

			// Validate request properties
			suite.Require().Equal(testCase.expectedMethod, req.Method)
			suite.Require().Equal(testCase.expectedURL, req.URL.String())

			for key, value := range testCase.expectedHeader {
				suite.Require().Equal(value, req.Header.Get(key))
			}
		})
	}
}

func TestAbstractAuthTestSuite(t *testing.T) {
	suite.Run(t, new(AbstractAuthTestSuite))
}
