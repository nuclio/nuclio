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

package resourcescaler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/auth/authproxy"
	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	kubeclient "github.com/nuclio/nuclio/pkg/platform/kube/clients/kube"
	nuclioiofake "github.com/nuclio/nuclio/pkg/platform/kube/clients/nuclio/clientset/versioned/fake"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

const (
	testNamespace = "nuclio-test"
	testSigninURL = "https://signin.example.com/oauth2/start"
	validToken    = "valid-token"
)

// TargetAuthenticatorTestSuite exercises the DLX-side client against a real authOnly auth-proxy, so
// the whole hop is covered: client -> HTTP -> authOnly authenticator -> CRD lookup -> auth-url.
type TargetAuthenticatorTestSuite struct {
	suite.Suite
	logger logger.Logger

	// the request line the auth-proxy actually saw, to prove the caller's own is what reaches it
	authProxyRequestURI string
}

func (suite *TargetAuthenticatorTestSuite) SetupTest() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)
}

// TestNoneModeAllowsWithoutCallingAuthURL verifies a function that opts out is admitted, and that the
// admission costs no auth-url round trip.
func (suite *TargetAuthenticatorTestSuite) TestNoneModeAllowsWithoutCallingAuthURL() {
	authURLCallCount := 0
	authenticator, closeServers := suite.newAuthOnlyAuthenticator(&authURLCallCount,
		suite.newFunction("none-func", map[string]interface{}{
			"authenticationMode": auth.AuthenticationModeNone,
		}))
	defer closeServers()

	recorder := httptest.NewRecorder()
	suite.Require().True(authenticator.AuthenticateTarget(recorder, suite.callerRequest(validToken), "none-func"))
	suite.Require().Zero(authURLCallCount)
}

// TestAPIModeValidCredentialAllows verifies the happy path: the DLX forwards the credential, the proxy
// validates it against the auth-url, and the function may be scaled.
func (suite *TargetAuthenticatorTestSuite) TestAPIModeValidCredentialAllows() {
	authURLCallCount := 0
	authenticator, closeServers := suite.newAuthOnlyAuthenticator(&authURLCallCount,
		suite.newFunction("api-func", map[string]interface{}{
			"authenticationMode": auth.AuthenticationModeAPI,
		}))
	defer closeServers()

	recorder := httptest.NewRecorder()
	suite.Require().True(authenticator.AuthenticateTarget(recorder, suite.callerRequest(validToken), "api-func"))
	suite.Require().Equal(1, authURLCallCount)
}

// TestAPIModeInvalidCredentialRelays401 is the core acceptance criterion: an invalid credential must
// reach the caller as a 401, not as a silent success.
func (suite *TargetAuthenticatorTestSuite) TestAPIModeInvalidCredentialRelays401() {
	authURLCallCount := 0
	authenticator, closeServers := suite.newAuthOnlyAuthenticator(&authURLCallCount,
		suite.newFunction("api-func", map[string]interface{}{
			"authenticationMode": auth.AuthenticationModeAPI,
		}))
	defer closeServers()

	recorder := httptest.NewRecorder()
	suite.Require().False(authenticator.AuthenticateTarget(recorder, suite.callerRequest("bad-token"), "api-func"))
	suite.Require().Equal(http.StatusUnauthorized, recorder.Code)
}

// TestBrowserModeRelays302ToTheOriginalURL verifies the redirect points back at what the caller asked
// for, reconstructed from the replayed request line plus the forwarded host - no header carries it.
func (suite *TargetAuthenticatorTestSuite) TestBrowserModeRelays302ToTheOriginalURL() {
	authURLCallCount := 0
	authenticator, closeServers := suite.newAuthOnlyAuthenticator(&authURLCallCount,
		suite.newFunction("browser-func", map[string]interface{}{
			"authenticationMode": auth.AuthenticationModeBrowser,
		}))
	defer closeServers()

	recorder := httptest.NewRecorder()
	suite.Require().False(authenticator.AuthenticateTarget(recorder, suite.callerRequest("bad-token"), "browser-func"))
	suite.Require().Equal(http.StatusFound, recorder.Code)

	redirectTarget, err := url.Parse(recorder.Header().Get("Location"))
	suite.Require().NoError(err)
	suite.Require().Contains(redirectTarget.String(), "signin.example.com/oauth2/start")
	suite.Require().Equal("https://func.example.com/some/path?q=1", redirectTarget.Query().Get("rd"))
}

// TestCallerRequestLineReachesTheAuthProxy verifies the DLX replays the caller's own request line
// rather than a fixed decision path, which is what lets the proxy build a redirect back to it.
func (suite *TargetAuthenticatorTestSuite) TestCallerRequestLineReachesTheAuthProxy() {
	authURLCallCount := 0
	authenticator, closeServers := suite.newAuthOnlyAuthenticator(&authURLCallCount,
		suite.newFunction("api-func", map[string]interface{}{
			"authenticationMode": auth.AuthenticationModeAPI,
		}))
	defer closeServers()

	recorder := httptest.NewRecorder()
	suite.Require().True(authenticator.AuthenticateTarget(recorder, suite.callerRequest(validToken), "api-func"))
	suite.Require().Equal("/some/path?q=1", suite.authProxyRequestURI)
}

// TestDLXResolvedTargetOverridesTheCallerHeader verifies the name the DLX resolved is the one the
// proxy decides on. A caller naming a permissive function must not get a stricter one scaled on that
// verdict, so the target header is set from the argument rather than forwarded.
func (suite *TargetAuthenticatorTestSuite) TestDLXResolvedTargetOverridesTheCallerHeader() {
	authURLCallCount := 0
	authenticator, closeServers := suite.newAuthOnlyAuthenticator(&authURLCallCount,
		suite.newFunction("none-func", map[string]interface{}{
			"authenticationMode": auth.AuthenticationModeNone,
		}),
		suite.newFunction("api-func", map[string]interface{}{
			"authenticationMode": auth.AuthenticationModeAPI,
		}))
	defer closeServers()

	// the caller claims the function that admits everyone, but api-func is what is about to be scaled
	request := suite.callerRequest("bad-token")
	request.Header.Set(headers.TargetFunctionName, "none-func")

	recorder := httptest.NewRecorder()
	suite.Require().False(authenticator.AuthenticateTarget(recorder, request, "api-func"))
	suite.Require().Equal(http.StatusUnauthorized, recorder.Code)
}

// TestBasicAuthRelaysChallenge verifies the WWW-Authenticate challenge survives the relay - without it
// the caller gets a 401 it cannot act on.
func (suite *TargetAuthenticatorTestSuite) TestBasicAuthRelaysChallenge() {
	authURLCallCount := 0
	authenticator, closeServers := suite.newAuthOnlyAuthenticator(&authURLCallCount,
		suite.newFunction("basic-func", map[string]interface{}{
			"authenticationMode": auth.AuthenticationModeBasicAuth,
			"authentication": map[string]interface{}{
				"basicAuth": map[string]interface{}{"username": "user", "password": "pass"},
			},
		}))
	defer closeServers()

	suite.Run("valid credential allows", func() {
		request := suite.callerRequest("")
		request.SetBasicAuth("user", "pass")
		recorder := httptest.NewRecorder()
		suite.Require().True(authenticator.AuthenticateTarget(recorder, request, "basic-func"))
	})

	suite.Run("wrong password is rejected with a challenge", func() {
		request := suite.callerRequest("")
		request.SetBasicAuth("user", "wrong")
		recorder := httptest.NewRecorder()
		suite.Require().False(authenticator.AuthenticateTarget(recorder, request, "basic-func"))
		suite.Require().Equal(http.StatusUnauthorized, recorder.Code)
		suite.Require().Contains(recorder.Header().Get("WWW-Authenticate"), "Basic realm")
	})
}

// TestUnknownFunctionFailsClosed verifies a target whose CRD cannot be read is never scaled.
func (suite *TargetAuthenticatorTestSuite) TestUnknownFunctionFailsClosed() {
	authURLCallCount := 0
	authenticator, closeServers := suite.newAuthOnlyAuthenticator(&authURLCallCount,
		suite.newFunction("api-func", map[string]interface{}{
			"authenticationMode": auth.AuthenticationModeAPI,
		}))
	defer closeServers()

	recorder := httptest.NewRecorder()
	suite.Require().False(authenticator.AuthenticateTarget(recorder, suite.callerRequest(validToken), "does-not-exist"))
	suite.Require().Equal(http.StatusForbidden, recorder.Code)
}

// TestUnreachableAuthProxyFailsClosed verifies the DLX rejects rather than scales when it cannot get a
// verdict at all - the sidecar being down must not become an open door.
func (suite *TargetAuthenticatorTestSuite) TestUnreachableAuthProxyFailsClosed() {
	authenticator := &AuthOnlyAuthenticator{
		logger: suite.logger,

		// nothing is listening here
		authProxy:  "http://127.0.0.1:1",
		httpClient: newAuthProxyHTTPClient(),
	}

	recorder := httptest.NewRecorder()
	suite.Require().False(authenticator.AuthenticateTarget(recorder, suite.callerRequest(validToken), "api-func"))
	suite.Require().Equal(http.StatusBadGateway, recorder.Code)
}

// TestDisabledFeatureFlagYieldsNoAuthenticator verifies the DLX skips the check entirely when the
// platform-wide flag is off, which is what keeps the pre-feature behavior intact.
func (suite *TargetAuthenticatorTestSuite) TestDisabledFeatureFlagYieldsNoAuthenticator() {
	for _, testCase := range []struct {
		name                  string
		authenticationConfig  *platformconfig.Authentication
		expectedAuthenticator bool
	}{
		{name: "authentication config absent", authenticationConfig: nil},
		{
			name:                 "flag explicitly off",
			authenticationConfig: &platformconfig.Authentication{FunctionAuthenticationEnabled: false},
		},
		{
			name:                  "flag on",
			authenticationConfig:  &platformconfig.Authentication{FunctionAuthenticationEnabled: true},
			expectedAuthenticator: true,
		},
	} {
		suite.Run(testCase.name, func() {
			resourceScaler := &NuclioResourceScaler{
				logger:                suite.logger,
				platformConfiguration: &platformconfig.Config{Authentication: testCase.authenticationConfig},
			}

			if testCase.expectedAuthenticator {
				suite.Require().NotNil(resourceScaler.newAuthOnlyAuthenticator())
			} else {
				suite.Require().Nil(resourceScaler.newAuthOnlyAuthenticator())
			}
		})
	}
}

// --- test helpers ---

// newAuthOnlyAuthenticator stands up an auth-url stub plus a real authOnly auth-proxy over HTTP, and
// returns a client pointed at it. The authenticator, its decision logic, and the handler shape are the
// production ones, so the whole hop is under test.
func (suite *TargetAuthenticatorTestSuite) newAuthOnlyAuthenticator(authURLCallCount *int,
	functions ...*nuclioio.NuclioFunction) (*AuthOnlyAuthenticator, func()) {

	authURLServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		*authURLCallCount++

		// admit only the actual credential, never the caller-declared authenticator kind
		if request.Header.Get(headers.AuthorizationHeader) != "Bearer "+validToken {
			responseWriter.WriteHeader(http.StatusUnauthorized)
			return
		}
		responseWriter.WriteHeader(http.StatusOK)
		_, err := responseWriter.Write([]byte(`{"metadata":{"username":"alice","id":"user-1"}}`))
		suite.Require().NoError(err)
	}))

	objects := make([]runtime.Object, 0, len(functions))
	for _, function := range functions {
		objects = append(objects, function)
	}

	authenticator, err := authproxy.NewAuthOnlyAuthenticator(suite.logger,
		authURLServer.URL,
		testSigninURL,
		auth.KindIguazioV4,
		nuclioiofake.NewSimpleClientset(objects...),
		kubeclient.NewClientWithRetryFromClient(k8sfake.NewClientset()),
		testNamespace)
	suite.Require().NoError(err)

	// the same catch-all shape as the production newAuthOnlyHandler: the DLX replays the caller's
	// request line, so the path is whatever the caller asked for
	authProxyServer := httptest.NewServer(http.HandlerFunc(
		func(responseWriter http.ResponseWriter, request *http.Request) {
			suite.authProxyRequestURI = request.URL.RequestURI()
			if authenticator.Authenticate(responseWriter, request) {
				responseWriter.WriteHeader(http.StatusOK)
			}
		}))

	return &AuthOnlyAuthenticator{
			logger:     suite.logger,
			authProxy:  authProxyServer.URL,
			httpClient: newAuthProxyHTTPClient(),
		}, func() {
			authProxyServer.Close()
			authURLServer.Close()
		}
}

// callerRequest builds the request as it reaches the DLX from the ingress: the caller's host and path
// arrive on X-Forwarded-Host, which is what a browser-mode redirect must be built from.
func (suite *TargetAuthenticatorTestSuite) callerRequest(token string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, "http://dlx.nuclio/some/path?q=1", nil)
	request.Header.Set(headers.ForwardHost, "func.example.com")
	if token != "" {
		request.Header.Set(headers.AuthorizationHeader, "Bearer "+token)
	}
	return request
}

func (suite *TargetAuthenticatorTestSuite) newFunction(name string,
	attributes map[string]interface{}) *nuclioio.NuclioFunction {
	return &nuclioio.NuclioFunction{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
		Spec: functionconfig.Spec{
			Triggers: map[string]functionconfig.Trigger{
				"http": {Kind: "http", Attributes: attributes},
			},
		},
	}
}

func TestTargetAuthenticatorTestSuite(t *testing.T) {
	suite.Run(t, new(TargetAuthenticatorTestSuite))
}
