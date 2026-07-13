//go:build test_unit

/*
Copyright 2026 The Nuclio Authors.

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

package authproxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	authpkg "github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/auth/factory"
	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	nuclioiofake "github.com/nuclio/nuclio/pkg/platform/kube/clients/nuclio/clientset/versioned/fake"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const validToken = "valid-token"

// authURLStub stands in for the auth-url (GetSelf): it admits a single valid bearer token and records
// what the sidecar forwarded.
type authURLStub struct {
	server          *httptest.Server
	callCount       int
	lastKind        string
	lastOriginalURI string
}

func (s *authURLStub) close() {
	s.server.Close()
}

type AuthProxyTestSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *AuthProxyTestSuite) SetupTest() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("authproxy-test")
	suite.Require().NoError(err)
}

func (suite *AuthProxyTestSuite) newAuthURLStub() *authURLStub {
	stub := &authURLStub{}
	stub.server = httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		stub.callCount++
		stub.lastKind = request.Header.Get(headers.IguazioAuthenticatorKind)
		stub.lastOriginalURI = request.Header.Get(headers.OriginalURI)

		// admit only the actual valid credential, regardless of the declared authenticator kind
		if request.Header.Get(headers.AuthorizationHeader) == "Bearer "+validToken {
			responseWriter.WriteHeader(http.StatusOK)
			_, _ = responseWriter.Write([]byte(`{"metadata":{"username":"alice","id":"user-1"}}`))
			return
		}
		responseWriter.WriteHeader(http.StatusUnauthorized)
	}))
	return stub
}

func (suite *AuthProxyTestSuite) newAuth(verificationURL string) authpkg.Auth {
	authConfig := authpkg.NewConfig(authpkg.KindIguazioV4)
	authConfig.Iguazio.VerificationURL = verificationURL
	authConfig.Iguazio.Timeout = AuthTimeout
	return factory.NewAuth(suite.logger, authConfig)
}

func (suite *AuthProxyTestSuite) authenticatedRequest(token string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, "http://function.nuclio/some/path?q=1", nil)
	if token != "" {
		request.Header.Set(headers.AuthorizationHeader, "Bearer "+token)
	}
	return request
}

// TestModeNoneAllows verifies none mode admits without calling the auth-url.
func (suite *AuthProxyTestSuite) TestModeNoneAllows() {
	stub := suite.newAuthURLStub()
	defer stub.close()

	authenticator := NewStaticAuthenticator(suite.logger, suite.newAuth(stub.server.URL), "", Settings{Mode: ModeNone})

	recorder := httptest.NewRecorder()
	suite.Require().True(authenticator.Authenticate(recorder, suite.authenticatedRequest("")))
	suite.Require().Equal(0, stub.callCount)
}

// TestModeAPI verifies api mode: valid credential is admitted (with identity headers), invalid/missing is 401.
func (suite *AuthProxyTestSuite) TestModeAPI() {
	stub := suite.newAuthURLStub()
	defer stub.close()

	authenticator := NewStaticAuthenticator(suite.logger, suite.newAuth(stub.server.URL), "", Settings{Mode: ModeAPI})

	suite.Run("valid is admitted with identity headers", func() {
		request := suite.authenticatedRequest(validToken)
		recorder := httptest.NewRecorder()
		suite.Require().True(authenticator.Authenticate(recorder, request))
		suite.Require().Equal("alice", request.Header.Get(headers.RemoteUser))
		suite.Require().Equal("user-1", request.Header.Get(headers.UserID))
	})

	suite.Run("invalid is rejected with 401", func() {
		recorder := httptest.NewRecorder()
		suite.Require().False(authenticator.Authenticate(recorder, suite.authenticatedRequest("bad-token")))
		suite.Require().Equal(http.StatusUnauthorized, recorder.Code)
	})

	suite.Run("missing credential is rejected without an auth-url call", func() {
		callsBefore := stub.callCount
		recorder := httptest.NewRecorder()
		suite.Require().False(authenticator.Authenticate(recorder, suite.authenticatedRequest("")))
		suite.Require().Equal(http.StatusUnauthorized, recorder.Code)
		suite.Require().Equal(callsBefore, stub.callCount)
	})
}

// TestModeBrowser verifies browser mode redirects (302) to the sign-in URL on failure and admits on success.
func (suite *AuthProxyTestSuite) TestModeBrowser() {
	stub := suite.newAuthURLStub()
	defer stub.close()

	const signinURL = "https://signin.example.com/oauth2/start"
	authenticator := NewStaticAuthenticator(suite.logger, suite.newAuth(stub.server.URL), signinURL, Settings{Mode: ModeBrowser})

	suite.Run("invalid redirects to sign-in with rd", func() {
		recorder := httptest.NewRecorder()
		suite.Require().False(authenticator.Authenticate(recorder, suite.authenticatedRequest("bad-token")))
		suite.Require().Equal(http.StatusFound, recorder.Code)
		location := recorder.Header().Get("Location")
		suite.Require().Contains(location, "signin.example.com/oauth2/start")
		suite.Require().Contains(location, "rd=")
	})

	suite.Run("valid is admitted", func() {
		recorder := httptest.NewRecorder()
		suite.Require().True(authenticator.Authenticate(recorder, suite.authenticatedRequest(validToken)))
	})
}

// TestModeBasicAuth verifies basicAuth is verified locally, never via the auth-url.
func (suite *AuthProxyTestSuite) TestModeBasicAuth() {
	stub := suite.newAuthURLStub()
	defer stub.close()

	authenticator := NewStaticAuthenticator(suite.logger,
		suite.newAuth(stub.server.URL),
		"",
		Settings{Mode: ModeBasicAuth, BasicAuthUsername: "user", BasicAuthPassword: "pass"})

	suite.Run("valid credentials admitted locally", func() {
		request := suite.authenticatedRequest("")
		request.SetBasicAuth("user", "pass")
		recorder := httptest.NewRecorder()
		suite.Require().True(authenticator.Authenticate(recorder, request))
		suite.Require().Equal(0, stub.callCount)
	})

	suite.Run("invalid credentials rejected with 401", func() {
		request := suite.authenticatedRequest("")
		request.SetBasicAuth("user", "wrong")
		recorder := httptest.NewRecorder()
		suite.Require().False(authenticator.Authenticate(recorder, request))
		suite.Require().Equal(http.StatusUnauthorized, recorder.Code)
		suite.Require().Equal(0, stub.callCount)
	})
}

// TestAuthenticatorKindForwardedAndCannotBypass verifies the caller's kind is forwarded to the auth-url,
// but a forged kind cannot bypass the check because the actual credential is still validated.
func (suite *AuthProxyTestSuite) TestAuthenticatorKindForwardedAndCannotBypass() {
	stub := suite.newAuthURLStub()
	defer stub.close()

	authenticator := NewStaticAuthenticator(suite.logger, suite.newAuth(stub.server.URL), "", Settings{Mode: ModeAPI})

	suite.Run("kind forwarded on valid credential", func() {
		request := suite.authenticatedRequest(validToken)
		request.Header.Set(headers.IguazioAuthenticatorKind, "sa")
		recorder := httptest.NewRecorder()
		suite.Require().True(authenticator.Authenticate(recorder, request))
		suite.Require().Equal("sa", stub.lastKind)
	})

	suite.Run("forged kind with invalid credential is rejected", func() {
		request := suite.authenticatedRequest("bad-token")
		request.Header.Set(headers.IguazioAuthenticatorKind, "sa")
		recorder := httptest.NewRecorder()
		suite.Require().False(authenticator.Authenticate(recorder, request))
		suite.Require().Equal(http.StatusUnauthorized, recorder.Code)
	})
}

// TestFailClosedOnUpstreamError verifies non-2xx and transport errors from the auth-url fail closed.
func (suite *AuthProxyTestSuite) TestFailClosedOnUpstreamError() {
	suite.Run("non-2xx from auth-url", func() {
		errorServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
			responseWriter.WriteHeader(http.StatusInternalServerError)
		}))
		defer errorServer.Close()

		authenticator := NewStaticAuthenticator(suite.logger, suite.newAuth(errorServer.URL), "", Settings{Mode: ModeAPI})
		recorder := httptest.NewRecorder()
		suite.Require().False(authenticator.Authenticate(recorder, suite.authenticatedRequest(validToken)))
		suite.Require().Equal(http.StatusUnauthorized, recorder.Code)
	})

	suite.Run("transport error to auth-url", func() {

		// nothing is listening here -> connection refused
		authenticator := NewStaticAuthenticator(suite.logger, suite.newAuth("http://127.0.0.1:1"), "", Settings{Mode: ModeAPI})
		recorder := httptest.NewRecorder()
		suite.Require().False(authenticator.Authenticate(recorder, suite.authenticatedRequest(validToken)))
		suite.Require().Equal(http.StatusUnauthorized, recorder.Code)
	})
}

// TestCRDAuthenticatorResolvesSettings verifies the authOnly provider resolves the mode from the target
// function's CRD (name from header, namespace from startup config).
func (suite *AuthProxyTestSuite) TestCRDAuthenticatorResolvesSettings() {
	stub := suite.newAuthURLStub()
	defer stub.close()

	const namespace = "nuclio"

	apiFunction := &nuclioio.NuclioFunction{
		ObjectMeta: metav1.ObjectMeta{Name: "api-func", Namespace: namespace},
		Spec: functionconfig.Spec{
			Triggers: map[string]functionconfig.Trigger{
				"http": {
					Kind:       "http",
					Attributes: map[string]interface{}{"authenticationMode": ModeAPI},
				},
			},
		},
	}
	basicAuthFunction := &nuclioio.NuclioFunction{
		ObjectMeta: metav1.ObjectMeta{Name: "basic-func", Namespace: namespace},
		Spec: functionconfig.Spec{
			Triggers: map[string]functionconfig.Trigger{
				"http": {
					Kind: "http",
					Attributes: map[string]interface{}{
						"authenticationMode": ModeBasicAuth,
						"authentication": map[string]interface{}{
							"basicAuth": map[string]interface{}{
								"username": "user",
								"password": "pass",
							},
						},
					},
				},
			},
		},
	}

	nuclioClientSet := nuclioiofake.NewSimpleClientset(apiFunction, basicAuthFunction)
	authenticator := NewCRDAuthenticator(suite.logger, suite.newAuth(stub.server.URL), "", nuclioClientSet, namespace)

	suite.Run("api function resolved and valid credential admitted", func() {
		request := suite.authenticatedRequest(validToken)
		request.Header.Set(headers.TargetFunctionName, "api-func")
		recorder := httptest.NewRecorder()
		suite.Require().True(authenticator.Authenticate(recorder, request))
	})

	suite.Run("basicAuth function resolved and verified locally", func() {
		request := suite.authenticatedRequest("")
		request.Header.Set(headers.TargetFunctionName, "basic-func")
		request.SetBasicAuth("user", "pass")
		recorder := httptest.NewRecorder()
		suite.Require().True(authenticator.Authenticate(recorder, request))
	})

	suite.Run("missing target function header fails closed", func() {
		recorder := httptest.NewRecorder()
		suite.Require().False(authenticator.Authenticate(recorder, suite.authenticatedRequest(validToken)))
		suite.Require().Equal(http.StatusForbidden, recorder.Code)
	})

	suite.Run("unknown function fails closed", func() {
		request := suite.authenticatedRequest(validToken)
		request.Header.Set(headers.TargetFunctionName, "does-not-exist")
		recorder := httptest.NewRecorder()
		suite.Require().False(authenticator.Authenticate(recorder, request))
		suite.Require().Equal(http.StatusForbidden, recorder.Code)
	})
}

func TestAuthProxyTestSuite(t *testing.T) {
	suite.Run(t, new(AuthProxyTestSuite))
}
