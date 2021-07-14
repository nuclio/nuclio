// +build test_unit

package iguazio

import (
	"net/http"
	"testing"

	"github.com/nuclio/nuclio/pkg/dashboard/auth"
	authconfig "github.com/nuclio/nuclio/pkg/dashboard/auth/config"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func NewTestClient(fn RoundTripFunc) *http.Client { // nolint: interfacer
	return &http.Client{
		Transport: fn,
	}
}

type AuthTestSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *AuthTestSuite) SetupSuite() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("iguazio-auth")
	suite.Require().NoError(err)
}

func (suite *AuthTestSuite) TestAuthenticate() {

	// mocks Iguazio session verification endpoint
	mockedHTTPClient := suite.createHTTPMockClient(func(r *http.Request) *http.Response {
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
				"X-Remote-User":      {"admin"},
				"X-User-Group-Ids":   {"1", "2"},
				"X-User-Id":          {"3"},
				"X-V3io-Session-Key": {"4"},
			},
		}
	})

	for _, testCase := range []struct {
		name            string
		auth            Auth
		incomingRequest *http.Request
		invalidRequest  bool
	}{
		{
			name: "sanity",
			auth: Auth{
				logger: suite.logger,
				options: func() *authconfig.Options {
					authConfig := authconfig.NewOptions(auth.ModeIguazio)
					authConfig.Iguazio.VerificationURL = "http://somewhere.local"
					return authConfig
				}(),
				httpClient: mockedHTTPClient,
			},
			incomingRequest: &http.Request{
				Header: map[string][]string{
					"Authorization": {"Basic YWJjOmVmZwo="},
					"Cookie":        {"session=some-session"},
				},
			},
		},
		{
			name: "missingCookie",
			auth: Auth{
				logger: suite.logger,
				options: func() *authconfig.Options {
					authConfig := authconfig.NewOptions(auth.ModeIguazio)
					authConfig.Iguazio.VerificationURL = "http://somewhere.local"
					return authConfig
				}(),
				httpClient: mockedHTTPClient,
			},
			incomingRequest: &http.Request{
				Header: map[string][]string{
					"Authorization": {"Basic YWJjOmVmZwo="},
				},
			},
			invalidRequest: true,
		},
		{
			name: "missingAuthorizationHeader",
			auth: Auth{
				logger: suite.logger,
				options: func() *authconfig.Options {
					authConfig := authconfig.NewOptions(auth.ModeIguazio)
					authConfig.Iguazio.VerificationURL = "http://somewhere.local"
					return authConfig
				}(),
				httpClient: mockedHTTPClient,
			},
			incomingRequest: &http.Request{
				Header: map[string][]string{
					"Cookie": {"session=some-session"},
				},
			},
			invalidRequest: true,
		},
	} {
		suite.Run(testCase.name, func() {
			err := testCase.auth.Authenticate(testCase.incomingRequest)
			if testCase.invalidRequest {
				suite.Require().Error(err)
				return
			}
			suite.Require().NoError(err)
			suite.Require().Equal("admin", testCase.auth.Username)
			suite.Require().Equal([]string{"1", "2"}, testCase.auth.GroupIDs)
			suite.Require().Equal("3", testCase.auth.UserID)
			suite.Require().Equal("4", testCase.auth.SessionKey)
		})
	}
}

func (suite *AuthTestSuite) createHTTPMockClient(f func(r *http.Request) *http.Response) *http.Client {
	return NewTestClient(func(req *http.Request) *http.Response {
		return f(req)
	})
}

func TestAuthTestSuite(t *testing.T) {
	suite.Run(t, new(AuthTestSuite))
}
