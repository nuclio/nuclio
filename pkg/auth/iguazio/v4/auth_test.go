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
	"time"

	"github.com/golang-jwt/jwt/v5"
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

func (suite *AuthTestSuite) TestAuthentication() {
	authConfig := authpkg.NewConfig(authpkg.KindIguazioV4)
	authConfig.Iguazio.VerificationURL = "http://somewhere.local/identity/self"

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
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewBufferString(`{
  "metadata": {
    "resourceType": "user",
    "username": "test"
  },
  "relationships": [
    {
      "@type": "type.googleapis.com/group.Group",
      "metadata": {
        "id": "61c12dc0-9863-4e56-9151-a1e09e8a69ed",
        "parentId": "6497f385-0958-42c0-88f0-8ee05e91bf8d",
        "path": "/g1/g3"
      },
      "spec": {
        "name": "g3"
      },
      "status": {}
    },
    {
      "@type": "type.googleapis.com/group.Group",
      "metadata": {
        "id": "6497f385-0958-42c0-88f0-8ee05e91bf8d",
        "path": "/g1",
        "subGroupCount": 1
      },
      "spec": {
        "name": "g1"
      },
      "status": {}
    }
  ],
  "status": {
    "ctx": "69d28f61-9e39-4e44-82ab-27f93d1e16a8",
    "statusCode": 200
  }
}`)),
			},
			cookieValue:      "_oauth2_proxy=session-cookie",
			expectedGroups:   []string{"61c12dc0-9863-4e56-9151-a1e09e8a69ed", "6497f385-0958-42c0-88f0-8ee05e91bf8d"},
			expectedUsername: "test",
		},
		{
			name:                 "StatusUnauthorizedEmptyAuth",
			responseFromIdentity: &http.Response{StatusCode: http.StatusUnauthorized},
			expectedErr:          "Failed to get authentication parameters",
		},
		{
			name:                     "StatusUnauthorizedWrongHeader",
			responseFromIdentity:     &http.Response{StatusCode: http.StatusUnauthorized},
			authorizationHeaderValue: "Bearer test-token",
			expectedErr:              "Authentication failed",
		},
		{
			name:                 "StatusUnauthorizedWrongCookie",
			responseFromIdentity: &http.Response{StatusCode: http.StatusUnauthorized},
			cookieValue:          "_oauth2_proxy=wrong-session-cookie",
			expectedErr:          "Authentication failed",
		},
		{
			name:                     "UnexpectedStatusCode",
			responseFromIdentity:     &http.Response{StatusCode: http.StatusInternalServerError},
			authorizationHeaderValue: "Bearer test-token",
			expectedErr:              "Authentication failed",
		},
	}

	for _, testCase := range tests {
		suite.Run(testCase.name, func() {
			mockClient := testutils.CreateDummyHTTPClient(func(r *http.Request) *http.Response {
				return testCase.responseFromIdentity
			})
			authInstance := suite.newAuthWithMockHTTPClient(authConfig, mockClient)
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
				suite.Require().Equal(testCase.expectedUsername, session.GetUsername())
				suite.Require().Equal(testCase.expectedGroups, session.GetGroupIDs())

			}
		})
	}

}

func (suite *AuthTestSuite) TestAuthenticateIguazioCaching() {
	// Generate a valid JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"sub": "test",
	})
	jwtToken, err := token.SignedString([]byte("test-secret"))
	suite.Require().NoError(err)

	// Define reusable response body
	successResponseBody := `{
  "metadata": {
    "resourceType": "user",
    "username": "test"
  },
  "relationships": [
    {
      "@type": "type.googleapis.com/group.Group",
      "metadata": {
        "id": "group1"
      }
    },
    {
      "@type": "type.googleapis.com/group.Group",
      "metadata": {
        "id": "group2"
      }
    }
  ]
}`

	tests := []struct {
		name             string
		headers          map[string]string
		mockHTTPClient   func(r *http.Request) *http.Response
		expectCache      bool
		expectError      bool
		expectedUsername string
		expectedGroupIDs []string
	}{
		{
			name: "WithAuthorizationHeader",
			headers: map[string]string{
				headers.AuthorizationHeader: "Bearer " + jwtToken,
			},
			mockHTTPClient: func(r *http.Request) *http.Response {
				authorization := r.Header.Get(headers.AuthorizationHeader)
				if authorization != "Bearer "+jwtToken {
					return &http.Response{
						StatusCode: http.StatusUnauthorized,
						Body:       io.NopCloser(bytes.NewBufferString(`{"error": "Invalid credentials"}`)),
					}
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(successResponseBody)),
				}
			},
			expectCache:      true,
			expectError:      false,
			expectedUsername: "test",
			expectedGroupIDs: []string{"group1", "group2"},
		},
		{
			name: "WithCookieOnly",
			headers: map[string]string{
				"Cookie": "_oauth2_proxy=session-cookie",
			},
			mockHTTPClient: func(r *http.Request) *http.Response {
				cookie := r.Header.Get("Cookie")
				if cookie != "_oauth2_proxy=session-cookie" {
					return &http.Response{
						StatusCode: http.StatusUnauthorized,
						Body:       io.NopCloser(bytes.NewBufferString(`{"error": "Invalid credentials"}`)),
					}
				}
				// Return a valid response body for the cookie-only case
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(successResponseBody)),
				}
			},
			expectCache:      false,
			expectError:      false,
			expectedUsername: "test",
		},
	}

	for _, testCase := range tests {
		suite.Run(testCase.name, func() {
			mockedHTTPClient := testutils.CreateDummyHTTPClient(testCase.mockHTTPClient)

			authConfig := authpkg.NewConfig(authpkg.KindIguazioV4)
			authConfig.Iguazio.VerificationURL = "http://somewhere.local/identity/self"

			authInstance := suite.newAuthWithMockHTTPClient(authConfig, mockedHTTPClient)
			authOptions := &authpkg.Options{}
			req, err := http.NewRequest("post", "", nil)
			suite.Require().NoError(err)

			for key, value := range testCase.headers {
				req.Header.Set(key, value)
			}

			// Step A: Authenticate and check cache behavior
			_, err = authInstance.Authenticate(req, authOptions)
			if testCase.expectError {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
			}

			if testCase.expectCache {
				suite.Require().NotEmpty(authInstance.Cache.Keys())

				// Step B: Re-authenticate and read from cache
				authInstance.HttpClient = nil
				session, err := authInstance.Authenticate(req, authOptions)
				suite.Require().NoError(err)
				suite.Require().Equal(testCase.expectedUsername, session.GetUsername())
				suite.Require().Equal(testCase.expectedGroupIDs, session.GetGroupIDs())

				// Remove the cached session
				authInstance.Cache.Remove(authInstance.Cache.Keys()[0])
			} else {
				suite.Require().Empty(authInstance.Cache.Keys())
			}
		})
	}
}

func (suite *AuthTestSuite) newAuthWithMockHTTPClient(authConfig *authpkg.Config, mockHttpClient *http.Client) *Auth {
	authInstance := NewAuth(suite.logger, authConfig)
	authInstanceIGZ := authInstance.(*Auth)
	authInstanceIGZ.HttpClient = mockHttpClient
	return authInstanceIGZ
}

func TestV4AuthTestSuite(t *testing.T) {
	suite.Run(t, new(AuthTestSuite))
}
