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

	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	kubeclient "github.com/nuclio/nuclio/pkg/platform/kube/clients/kube"
	nuclioiofake "github.com/nuclio/nuclio/pkg/platform/kube/clients/nuclio/clientset/versioned/fake"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

const (
	testNamespace = "test-namespace"
	testSigninURL = "https://signin.example.com/oauth2/start"
)

type AuthOnlyTestSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *AuthOnlyTestSuite) SetupTest() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("authonly-test")
	suite.Require().NoError(err)
}

func (suite *AuthOnlyTestSuite) newTestAuthURLStub() *authURLStub {
	stub := &authURLStub{}
	stub.server = httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		stub.callCount++
		stub.lastKind = request.Header.Get(headers.IguazioAuthenticatorKind)
		if request.Header.Get(headers.AuthorizationHeader) == "Bearer "+validToken {
			responseWriter.WriteHeader(http.StatusOK)
			_, _ = responseWriter.Write([]byte(`{"metadata":{"username":"alice","id":"user-1"}}`))
			return
		}
		responseWriter.WriteHeader(http.StatusUnauthorized)
	}))
	return stub
}

func (suite *AuthOnlyTestSuite) authenticatedRequest(token string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, "http://function.nuclio/some/path?q=1", nil)
	if token != "" {
		request.Header.Set(headers.AuthorizationHeader, "Bearer "+token)
	}
	return request
}

// TestBrowserModeResolvedFromCRD verifies browser mode is resolved from the CRD: unauthenticated
// requests are redirected (302) to the sign-in URL; valid credentials are admitted.
func (suite *AuthOnlyTestSuite) TestBrowserModeResolvedFromCRD() {
	stub := suite.newTestAuthURLStub()
	defer stub.close()

	browserFunction := newHTTPTriggerFunction("browser-func", testNamespace, map[string]interface{}{
		"authenticationMode": auth.AuthenticationModeBrowser,
	})

	kubeClientSet := kubeclient.NewClientWithRetryFromClient(k8sfake.NewClientset())
	nuclioClientSet := nuclioiofake.NewSimpleClientset(browserFunction)
	authenticator, err := NewAuthOnlyAuthenticator(suite.logger, stub.server.URL, testSigninURL, auth.KindIguazioV4, nuclioClientSet, kubeClientSet, testNamespace)
	suite.Require().NoError(err)

	suite.Run("invalid credential redirects to sign-in with rd", func() {
		request := suite.authenticatedRequest("bad-token")
		request.Header.Set(headers.TargetFunctionName, "browser-func")
		recorder := httptest.NewRecorder()
		suite.Require().False(authenticator.Authenticate(recorder, request))
		suite.Require().Equal(http.StatusFound, recorder.Code)
		location := recorder.Header().Get("Location")
		suite.Require().Contains(location, "signin.example.com/oauth2/start")
		suite.Require().Contains(location, "rd=")
	})

	suite.Run("valid credential admitted", func() {
		request := suite.authenticatedRequest(validToken)
		request.Header.Set(headers.TargetFunctionName, "browser-func")
		recorder := httptest.NewRecorder()
		suite.Require().True(authenticator.Authenticate(recorder, request))
	})
}

// TestModeResolvedFromCRD verifies that the authOnly authenticator correctly resolves every
// authentication mode from the target function's CRD and authenticates accordingly.
func (suite *AuthOnlyTestSuite) TestModeResolvedFromCRD() {
	stub := suite.newTestAuthURLStub()
	defer stub.close()

	const signinURL = "https://signin.example.com/oauth2/start"

	for _, testCase := range []struct {
		name                 string
		function             *nuclioio.NuclioFunction
		token                string
		basicAuthUser        string
		basicAuthPass        string
		expectedAllowed      bool
		expectedAuthURLCalls int
	}{
		{
			name: "none mode allows without auth-url call",
			function: newHTTPTriggerFunction("none-func", testNamespace, map[string]interface{}{
				"authenticationMode": auth.AuthenticationModeNone,
			}),
			expectedAllowed:      true,
			expectedAuthURLCalls: 0,
		},
		{
			name: "no http trigger defaults to none",
			function: &nuclioio.NuclioFunction{
				ObjectMeta: metav1.ObjectMeta{Name: "no-trigger-func", Namespace: testNamespace},
				Spec:       functionconfig.Spec{},
			},
			expectedAllowed:      true,
			expectedAuthURLCalls: 0,
		},
		{
			name: "api mode admits valid token",
			function: newHTTPTriggerFunction("api-func", testNamespace, map[string]interface{}{
				"authenticationMode": auth.AuthenticationModeAPI,
			}),
			token:                validToken,
			expectedAllowed:      true,
			expectedAuthURLCalls: 1,
		},
		{
			name: "browser mode admits valid token",
			function: newHTTPTriggerFunction("browser-func", testNamespace, map[string]interface{}{
				"authenticationMode": auth.AuthenticationModeBrowser,
			}),
			token:                validToken,
			expectedAllowed:      true,
			expectedAuthURLCalls: 1,
		},
		{
			name: "basicAuth mode admits valid credentials without auth-url call",
			function: newHTTPTriggerFunction("basic-func", testNamespace, map[string]interface{}{
				"authenticationMode": auth.AuthenticationModeBasicAuth,
				"authentication": map[string]interface{}{
					"basicAuth": map[string]interface{}{"username": "user", "password": "pass"},
				},
			}),
			basicAuthUser:        "user",
			basicAuthPass:        "pass",
			expectedAllowed:      true,
			expectedAuthURLCalls: 0,
		},
	} {
		suite.Run(testCase.name, func() {
			callsBefore := stub.callCount

			kubeClientSet := kubeclient.NewClientWithRetryFromClient(k8sfake.NewClientset())
			nuclioClientSet := nuclioiofake.NewSimpleClientset(testCase.function)
			authenticator, err := NewAuthOnlyAuthenticator(suite.logger, stub.server.URL, signinURL, auth.KindIguazioV4, nuclioClientSet, kubeClientSet, testNamespace)
			suite.Require().NoError(err)

			request := suite.authenticatedRequest(testCase.token)
			request.Header.Set(headers.TargetFunctionName, testCase.function.Name)
			if testCase.basicAuthUser != "" {
				request.SetBasicAuth(testCase.basicAuthUser, testCase.basicAuthPass)
			}
			recorder := httptest.NewRecorder()

			suite.Require().Equal(testCase.expectedAllowed, authenticator.Authenticate(recorder, request))
			suite.Require().Equal(testCase.expectedAuthURLCalls, stub.callCount-callsBefore)
		})
	}
}

// TestGetAuthSpecFunctionNotFound verifies that getAuthSpec fails closed when the target function CRD
// does not exist.
func (suite *AuthOnlyTestSuite) TestGetAuthSpecFunctionNotFound() {
	stub := suite.newTestAuthURLStub()
	defer stub.close()

	// no functions registered in the fake client
	kubeClientSet := kubeclient.NewClientWithRetryFromClient(k8sfake.NewClientset())
	nuclioClientSet := nuclioiofake.NewSimpleClientset()
	authenticator, err := NewAuthOnlyAuthenticator(suite.logger, stub.server.URL, "", auth.KindIguazioV4, nuclioClientSet, kubeClientSet, testNamespace)
	suite.Require().NoError(err)

	request := suite.authenticatedRequest(validToken)
	request.Header.Set(headers.TargetFunctionName, "does-not-exist")
	recorder := httptest.NewRecorder()
	suite.Require().False(authenticator.Authenticate(recorder, request))
	suite.Require().Equal(http.StatusForbidden, recorder.Code)
	suite.Require().Equal(0, stub.callCount)
}

func TestAuthOnlyTestSuite(t *testing.T) {
	suite.Run(t, new(AuthOnlyTestSuite))
}
