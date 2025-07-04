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

package v4

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	authpkg "github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/common/testutils"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type AuthTestSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *AuthTestSuite) SetupSuite() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("iguazio-auth-v4")
	suite.Require().NoError(err)
}

func (suite *AuthTestSuite) TestAuthenticationNegative() {
	authConfig := authpkg.NewConfig(authpkg.KindIguazioV4)
	authConfig.Iguazio.VerificationURL = "http://somewhere.local"
	authConfig.Iguazio.VerificationEndpoint = "identity/self"

	tests := []struct {
		name                     string
		responseFromIdentity     *http.Response
		authorizationHeaderValue string
		cookieValue              string

		expectedUsername string
		expectedGroups   []string

		expectedErr string
	}{
		{
			name: "AuthorizeSuccess",
			responseFromIdentity: &http.Response{
				StatusCode: http.StatusAccepted,
				Body: io.NopCloser(bytes.NewBufferString(`{
				"metadata": { "username": "test" },
				"relationships": [
					{"metadata": {"id": "/g1"}},
					{"metadata": {"id": "/g2"}}
				]
			}`)),
			},
			cookieValue:      "_oauth2_proxy=session-cookie",
			expectedGroups:   []string{"/g1", "/g2"},
			expectedUsername: "test",
		},
		{
			name:                 "StatusUnauthorizedEmptyAuth",
			responseFromIdentity: &http.Response{StatusCode: http.StatusUnauthorized},
			expectedErr:          "Authentication headers are missing",
		},
		{
			name:                     "StatusUnauthorizedWrongHeader",
			responseFromIdentity:     &http.Response{StatusCode: http.StatusUnauthorized},
			authorizationHeaderValue: "Bearer test-token",
			expectedErr:              "Invalid credentials",
		},
		{
			name:                 "StatusUnauthorizedWrongCookie",
			responseFromIdentity: &http.Response{StatusCode: http.StatusUnauthorized},
			cookieValue:          "_oauth2_proxy=wrong-session-cookie",
			expectedErr:          "Invalid credentials",
		},
		{
			name:                     "UnexpectedStatusCode",
			responseFromIdentity:     &http.Response{StatusCode: http.StatusInternalServerError},
			authorizationHeaderValue: "Bearer test-token",
			expectedErr:              "Unexpected response from identity endpoint",
		},
	}

	for _, testCase := range tests {
		suite.Run(testCase.name, func() {
			mockClient := testutils.CreateDummyHTTPClient(func(r *http.Request) *http.Response {
				return testCase.responseFromIdentity
			})
			authInstance := suite.newAuthWithMockHttpClient(authConfig, mockClient)
			suite.Require().Equal("http://somewhere.local/identity/self", authInstance.verificationURL)

			req, err := http.NewRequest("post", "", nil)
			suite.Require().NoError(err)
			if testCase.authorizationHeaderValue != "" {
				req.Header.Set(headers.AuthorizationHeader, "Bearer test-token")
			}
			if testCase.cookieValue != "" {
				req.Header.Set("Cookie", testCase.cookieValue)
			}

			session, err := authInstance.Authenticate(req, &authpkg.Options{})
			if testCase.expectedErr != "" {
				suite.Require().Error(err)
				suite.Require().Empty(session)
				suite.Contains(err.Error(), testCase.expectedErr)
			} else {
				suite.Require().NoError(err)
				suite.Require().Equal(session.GetUsername(), testCase.expectedUsername)
				suite.Require().Equal(session.GetGroupIDs(), testCase.expectedGroups)

			}
		})
	}

}

func (suite *AuthTestSuite) newAuthWithMockHttpClient(authConfig *authpkg.Config, mockHttpClient *http.Client) *Auth {
	authInstance, err := NewAuth(suite.logger, authConfig)
	suite.Require().NoError(err)
	authInstanceIGZ := authInstance.(*Auth)
	authInstanceIGZ.HttpClient = mockHttpClient
	return authInstanceIGZ
}

func TestV4AuthTestSuite(t *testing.T) {
	suite.Run(t, new(AuthTestSuite))
}
