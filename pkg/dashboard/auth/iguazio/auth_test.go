// +build test_unit

package iguazio

import (
	"net/http"
	"testing"

	"github.com/nuclio/nuclio/pkg/dashboard/auth"

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

func (suite *AuthTestSuite) TestAuthenticateIguazioCaching() {
	// mocks IguazioConfig session verification endpoint
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
				"X-V3io-session-Key": {"4"},
			},
		}
	})

	newAuth := NewAuth(suite.logger, func() *auth.Config {
		authConfig := auth.NewConfig(auth.KindIguazio)
		authConfig.Iguazio.VerificationURL = "http://somewhere.local"
		return authConfig
	}())
	authInstance := newAuth.(*Auth)
	authInstance.httpClient = mockedHTTPClient
	incomingRequest := &http.Request{
		Header: map[string][]string{
			"Authorization": {"Basic YWJjOmVmZwo="},
			"Cookie":        {"session=some-session"},
		}}

	// step A. successfully authenticate, let it to be cached
	_, err := authInstance.Authenticate(incomingRequest)
	suite.Require().NoError(err)
	suite.Require().NotEmpty(authInstance.cache.Keys())

	// step B. re-authenticate, read from cache
	// nil the http client in order to force it to panic if it was used to make an HTTP request
	authInstance.httpClient = nil
	_, err = authInstance.Authenticate(incomingRequest)
	suite.Require().NoError(err)

	authInstance.cache.Remove(authInstance.cache.Keys()[0])

	// step C. bad authentication + cache remains empty
	authInstance.httpClient = suite.createHTTPMockClient(func(r *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
		}
	})
	_, err = authInstance.Authenticate(incomingRequest)
	suite.Require().Error(err)
	suite.Require().Empty(authInstance.cache.Keys())
}

func (suite *AuthTestSuite) TestAuthenticate() {

	// mocks IguazioConfig session verification endpoint
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
		auth            auth.Auth
		incomingRequest *http.Request
		invalidRequest  bool
	}{
		{
			name: "sanity",
			auth: NewAuth(suite.logger, func() *auth.Config {
				authConfig := auth.NewConfig(auth.KindIguazio)
				authConfig.Iguazio.VerificationURL = "http://somewhere.local"
				return authConfig
			}()),
			incomingRequest: &http.Request{
				Header: map[string][]string{
					"Authorization": {"Basic YWJjOmVmZwo="},
					"Cookie":        {"session=some-session"},
				},
			},
		},
		{
			name: "missingCookie",
			auth: NewAuth(suite.logger, func() *auth.Config {
				authConfig := auth.NewConfig(auth.KindIguazio)
				authConfig.Iguazio.VerificationURL = "http://somewhere.local"
				return authConfig
			}()),
			incomingRequest: &http.Request{
				Header: map[string][]string{
					"Authorization": {"Basic YWJjOmVmZwo="},
				},
			},
			invalidRequest: true,
		},
		{
			name: "missingAuthorizationHeader",
			auth: NewAuth(suite.logger, func() *auth.Config {
				authConfig := auth.NewConfig(auth.KindIguazio)
				authConfig.Iguazio.VerificationURL = "http://somewhere.local"
				return authConfig
			}()),
			incomingRequest: &http.Request{
				Header: map[string][]string{
					"Cookie": {"session=some-session"},
				},
			},
			invalidRequest: true,
		},
	} {
		suite.Run(testCase.name, func() {
			testCase.auth.(*Auth).httpClient = mockedHTTPClient
			authInfo, err := testCase.auth.Authenticate(testCase.incomingRequest)
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

func (suite *AuthTestSuite) createHTTPMockClient(f func(r *http.Request) *http.Response) *http.Client {
	return NewTestClient(func(req *http.Request) *http.Response {
		return f(req)
	})
}

func TestAuthTestSuite(t *testing.T) {
	suite.Run(t, new(AuthTestSuite))
}
