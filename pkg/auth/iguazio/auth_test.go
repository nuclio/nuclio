//go:build test_unit

/*
Copyright 2017 The Nuclio Authors.

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
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/nuclio/nuclio/pkg/auth"
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
	suite.logger, err = nucliozap.NewNuclioZapTest("iguazio-auth")
	suite.Require().NoError(err)
}

func (suite *AuthTestSuite) TestAuthenticateIguazioCaching() {
	// mocks IguazioConfig session verification endpoint
	mockedHTTPClient := testutils.CreateDummyHTTPClient(func(r *http.Request) *http.Response {
		authorization := r.Header.Get("Authorization")
		cookie := r.Header.Get("Cookie")
		if authorization != "Basic YWJjOmVmZwo=" || cookie != "session=some-session" {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
			}
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: map[string][]string{
				headers.RemoteUser:     {"admin"},
				headers.UserGroupIds:   {"1,2", "3"},
				headers.UserID:         {"some-user-id"},
				headers.V3IOSessionKey: {"some-password"},
			},
			Body: io.NopCloser(bytes.NewBufferString(`
{
    "data": {
        "type": "session_verification",
        "attributes": {
            "username": "some-username",
            "context": {
                "id": "1234",
                "authentication": {
                    "user_id": "some-user-id",
                    "tenant_id": "some-tenant-id",
                    "group_ids": [
                        "1,2", 
                        "3"
                    ],
                    "mode": "normal"
                }
            }
        }
    },
    "meta": {
        "ctx": "1234"
    }
}`)),
		}
	})

	newAuth := NewAuth(suite.logger, func() *auth.Config {
		authConfig := auth.NewConfig(auth.KindIguazio)
		authConfig.Iguazio.VerificationURL = "http://somewhere.local"
		return authConfig
	}())
	authInstance := newAuth.(*Auth)
	authInstance.httpClient = mockedHTTPClient
	authOptions := auth.Options{}
	incomingRequest := &http.Request{
		Header: map[string][]string{
			"Authorization": {"Basic YWJjOmVmZwo="},
			"Cookie":        {"session=some-session"},
		}}

	// step A. successfully authenticate, let it to be cached
	_, err := authInstance.Authenticate(incomingRequest, &authOptions)
	suite.Require().NoError(err)
	suite.Require().NotEmpty(authInstance.cache.Keys())

	// step B. re-authenticate, read from cache
	// nil the http client in order to force it to panic if it was used to make an HTTP request
	authInstance.httpClient = nil
	session, err := authInstance.Authenticate(incomingRequest, &authOptions)
	suite.Require().NoError(err)
	suite.Require().Equal("some-user-id", session.GetUserID())
	suite.Require().Equal([]string{"1", "2", "3"}, session.GetGroupIDs())
	suite.Require().Equal("admin", session.GetUsername())
	suite.Require().Equal("some-password", session.GetPassword())

	authInstance.cache.Remove(authInstance.cache.Keys()[0])

	// step C. bad authentication + cache remains empty
	authInstance.httpClient = testutils.CreateDummyHTTPClient(func(r *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
		}
	})
	_, err = authInstance.Authenticate(incomingRequest, &authOptions)
	suite.Require().Error(err)
	suite.Require().Empty(authInstance.cache.Keys())
}

func (suite *AuthTestSuite) TestAuthenticate() {

	for _, testCase := range []struct {
		name                string
		auth                auth.Auth
		authOptions         auth.Options
		incomingRequest     *http.Request
		invalidRequest      bool
		includeResponseBody bool
	}{
		{
			name: "sanity",
			auth: NewAuth(suite.logger, func() *auth.Config {
				authConfig := auth.NewConfig(auth.KindIguazio)
				authConfig.Iguazio.VerificationURL = "http://somewhere.local"
				return authConfig
			}()),
			authOptions: auth.Options{},
			incomingRequest: &http.Request{
				Header: map[string][]string{
					"Authorization": {"Basic YWJjOmVmZwo="},
					"Cookie":        {"session=some-session"},
				},
			},
			includeResponseBody: true,
		},
		{
			name: "backwardsCompatibilitySanity",
			auth: NewAuth(suite.logger, func() *auth.Config {
				authConfig := auth.NewConfig(auth.KindIguazio)
				authConfig.Iguazio.VerificationURL = "http://somewhere.local"
				return authConfig
			}()),
			authOptions: auth.Options{},
			incomingRequest: &http.Request{
				Header: map[string][]string{
					"Authorization": {"Basic YWJjOmVmZwo="},
					"Cookie":        {"session=some-session"},
				},
			},
			includeResponseBody: true,
		},
		{
			name: "enrichmentSanity",
			auth: NewAuth(suite.logger, func() *auth.Config {
				authConfig := auth.NewConfig(auth.KindIguazio)
				authConfig.Iguazio.VerificationURL = "http://somewhere.local"
				return authConfig
			}()),
			authOptions: auth.Options{
				EnrichDataPlane: true,
			},
			incomingRequest: &http.Request{
				Header: map[string][]string{
					"Authorization": {"Basic YWJjOmVmZwo="},
					"Cookie":        {"session=some-session"},
				},
			},
			includeResponseBody: true,
		},
		{
			name: "missingCookie",
			auth: NewAuth(suite.logger, func() *auth.Config {
				authConfig := auth.NewConfig(auth.KindIguazio)
				authConfig.Iguazio.VerificationURL = "http://somewhere.local"
				return authConfig
			}()),
			authOptions: auth.Options{},
			incomingRequest: &http.Request{
				Header: map[string][]string{
					"Authorization": {"Basic YWJjOmVmZwo="},
				},
			},
			invalidRequest:      true,
			includeResponseBody: true,
		},
		{
			name: "missingAuthorizationHeader",
			auth: NewAuth(suite.logger, func() *auth.Config {
				authConfig := auth.NewConfig(auth.KindIguazio)
				authConfig.Iguazio.VerificationURL = "http://somewhere.local"
				return authConfig
			}()),
			authOptions: auth.Options{},
			incomingRequest: &http.Request{
				Header: map[string][]string{
					"Cookie": {"session=some-session"},
				},
			},
			invalidRequest:      true,
			includeResponseBody: false,
		},
	} {
		suite.Run(testCase.name, func() {
			testCase.auth.(*Auth).httpClient = testutils.CreateDummyHTTPClient(suite.resolveMockHttpClientHandler(testCase.includeResponseBody))
			authInfo, err := testCase.auth.Authenticate(testCase.incomingRequest, &testCase.authOptions)
			if testCase.invalidRequest {
				suite.Require().Error(err)
				return
			}
			suite.Require().NoError(err)
			suite.Require().Equal("admin", authInfo.GetUsername())
			suite.Require().Equal([]string{"1", "2"}, authInfo.GetGroupIDs())
			suite.Require().Equal("3", authInfo.GetUserID())
			suite.Require().Equal("4", authInfo.GetPassword())
		})
	}
}

func (suite *AuthTestSuite) resolveMockHttpClientHandler(includeResponseBody bool) func(r *http.Request) *http.Response {

	response := &http.Response{
		StatusCode: http.StatusOK,
		Header: map[string][]string{
			headers.RemoteUser:     {"admin"},
			headers.UserGroupIds:   {"1", "2"},
			headers.UserID:         {"3"},
			headers.V3IOSessionKey: {"4"},
		},
	}

	if includeResponseBody {
		response.Body = io.NopCloser(bytes.NewBufferString(`
{
    "data": {
        "type": "session_verification",
        "attributes": {
            "username": "some-username",
            "context": {
                "id": "1234",
                "authentication": {
                    "user_id": "3",
                    "tenant_id": "some-tenant-id",
                    "group_ids": [
                        "1", 
                        "2"
                    ],
                    "mode": "normal"
                }
            }
        }
    },
    "meta": {
        "ctx": "1234"
    }
}`))
	}

	return func(r *http.Request) *http.Response {
		authorization := r.Header.Get("Authorization")
		cookie := r.Header.Get("Cookie")
		if authorization != "Basic YWJjOmVmZwo=" || cookie != "session=some-session" {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
			}
		}
		return response
	}
}
func TestAuthTestSuite(t *testing.T) {
	suite.Run(t, new(AuthTestSuite))
}
