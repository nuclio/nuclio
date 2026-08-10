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

package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/auth/authproxy"
	httptrigger "github.com/nuclio/nuclio/pkg/processor/trigger/http"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

// fakeAuthenticator is a test double for authproxy.Authenticator; it records calls and, on reject,
// writes a fixed status like the real authenticator does.
type fakeAuthenticator struct {
	authorized bool
	rejectCode int
	calls      int
}

func (f *fakeAuthenticator) Authenticate(responseWriter http.ResponseWriter, _ *http.Request) bool {
	f.calls++
	if f.authorized {
		return true
	}
	responseWriter.WriteHeader(f.rejectCode)
	return false
}

type upstreamStub struct {
	server *httptest.Server
	hits   int
}

type ServerTestSuite struct {
	suite.Suite
	logger logger.Logger
	ctx    context.Context
	cancel context.CancelFunc
}

func (suite *ServerTestSuite) SetupTest() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("auth-proxy-test")
	suite.Require().NoError(err)

	suite.ctx, suite.cancel = context.WithCancel(context.Background())
}

func (suite *ServerTestSuite) TearDownTest() {
	if suite.cancel != nil {
		suite.cancel()
	}
}

// TestModeListenAddresses verifies the only listen-address difference between the modes: reverseProxy is
// exposed on the configured port, authOnly is bound to loopback (reachable only from within the pod).
func (suite *ServerTestSuite) TestModeListenAddresses() {
	for _, testCase := range []struct {
		name                  string
		mode                  auth.ProxyMode
		listenPort            int
		expectedListenAddress string
	}{
		{name: "reverseProxy is exposed", mode: auth.ProxyModeReverseProxy, listenPort: 8080, expectedListenAddress: ":8080"},
		{name: "authOnly is loopback", mode: auth.ProxyModeAuthOnly, listenPort: 8080, expectedListenAddress: "127.0.0.1:8080"},
	} {
		suite.Run(testCase.name, func() {
			handler, err := newHandler(suite.logger,
				testCase.mode,
				"http://127.0.0.1:6080",
				&fakeAuthenticator{authorized: true})
			suite.Require().NoError(err)

			listenAddress, err := resolveListenAddress(testCase.mode, testCase.listenPort)
			suite.Require().NoError(err)
			suite.Require().Equal(testCase.expectedListenAddress, listenAddress)

			server := newServer(suite.logger, listenAddress, handler)
			suite.Require().Equal(listenAddress, server.httpServer.Addr)
		})
	}
}

func (suite *ServerTestSuite) TestUnknownModeRejected() {
	_, err := newHandler(suite.logger,
		"unknown-mode",
		"http://127.0.0.1:6080",
		&fakeAuthenticator{authorized: true})
	suite.Require().Error(err)
	suite.Require().Contains(err.Error(), "Unknown auth-proxy mode")

	_, err = resolveListenAddress("unknown-mode", 8080)
	suite.Require().Error(err)
	suite.Require().Contains(err.Error(), "Unknown auth-proxy mode")
}

// TestStartServersPropagatesListenerFailure verifies that if one listener fails to bind, startServers
// closes the other listeners and returns the error instead of hanging forever. Startup is
// all-or-nothing on purpose: a Service port with no proxy behind it would bypass authentication.
func (suite *ServerTestSuite) TestStartServersPropagatesListenerFailure() {
	handler := newAuthOnlyHandler(&fakeAuthenticator{authorized: true})

	servers := []*server{

		// port 0 has the OS assign a free port, so this listener binds and then blocks serving
		newServer(suite.logger, "127.0.0.1:0", handler),

		// an out-of-range port never resolves, so this listener deterministically fails to bind
		newServer(suite.logger, "127.0.0.1:99999", handler),
	}

	err := startServers(suite.ctx, suite.logger, servers)
	suite.Require().Error(err)
}

// TestRoutesForwardToTheirOwnUpstream verifies that when the auth-proxy fronts several ports, newServers
// wires each listener to the upstream configured for that route - not to a shared one. The servers' handlers
// are driven directly, so the per-route wiring is asserted without binding any port.
func (suite *ServerTestSuite) TestRoutesForwardToTheirOwnUpstream() {
	functionUpstream := suite.newTestUpstreamStubWithBody("function-response")
	defer functionUpstream.server.Close()
	sidecarUpstream := suite.newTestUpstreamStubWithBody("sidecar-response")
	defer sidecarUpstream.server.Close()

	authenticator := &fakeAuthenticator{authorized: true}

	config := &Config{
		Mode: auth.ProxyModeReverseProxy,
		Routes: []authproxy.Route{
			{ListenPort: 8080, UpstreamURL: functionUpstream.server.URL},
			{ListenPort: 6081, UpstreamURL: sidecarUpstream.server.URL},
		},
	}

	servers, err := newServers(suite.logger, config, authenticator)
	suite.Require().NoError(err)
	suite.Require().Len(servers, 2)

	// the listen address is what binds the route to its port; assert it alongside the upstream it reaches
	suite.Require().Equal(":8080", servers[0].httpServer.Addr)
	suite.Require().Equal(":6081", servers[1].httpServer.Addr)

	statusCode, body := suite.doRequest(servers[0].httpServer.Handler, "/invoke")
	suite.Require().Equal(http.StatusOK, statusCode)
	suite.Require().Equal("function-response", body)

	statusCode, body = suite.doRequest(servers[1].httpServer.Handler, "/invoke")
	suite.Require().Equal(http.StatusOK, statusCode)
	suite.Require().Equal("sidecar-response", body)

	suite.Require().Equal(1, functionUpstream.hits)
	suite.Require().Equal(1, sidecarUpstream.hits)
	suite.Require().Equal(2, authenticator.calls)
}

// TestReverseProxyForwardsWhenAuthorized verifies an authorized request is proxied to the upstream.
func (suite *ServerTestSuite) TestReverseProxyForwardsWhenAuthorized() {
	upstream := suite.newTestUpstreamStub()
	defer upstream.server.Close()

	authenticator := &fakeAuthenticator{authorized: true}
	handler, err := newReverseProxyHandler(suite.logger, upstream.server.URL, authenticator)
	suite.Require().NoError(err)

	statusCode, body := suite.doRequest(handler, "/invoke")
	suite.Require().Equal(http.StatusOK, statusCode)
	suite.Require().Equal("upstream-response", body)
	suite.Require().Equal(1, upstream.hits)
	suite.Require().Equal(1, authenticator.calls)
}

// TestReverseProxyRejectsWhenUnauthorized verifies a rejected request never reaches the upstream.
func (suite *ServerTestSuite) TestReverseProxyRejectsWhenUnauthorized() {
	upstream := suite.newTestUpstreamStub()
	defer upstream.server.Close()

	authenticator := &fakeAuthenticator{authorized: false, rejectCode: http.StatusUnauthorized}
	handler, err := newReverseProxyHandler(suite.logger, upstream.server.URL, authenticator)
	suite.Require().NoError(err)

	statusCode, _ := suite.doRequest(handler, "/invoke")
	suite.Require().Equal(http.StatusUnauthorized, statusCode)
	suite.Require().Equal(0, upstream.hits)
}

// TestReverseProxyReadinessAllowlisted verifies the readiness probe is proxied unauthenticated.
func (suite *ServerTestSuite) TestReverseProxyReadinessAllowlisted() {
	upstream := suite.newTestUpstreamStub()
	defer upstream.server.Close()

	authenticator := &fakeAuthenticator{authorized: false, rejectCode: http.StatusUnauthorized}
	handler, err := newReverseProxyHandler(suite.logger, upstream.server.URL, authenticator)
	suite.Require().NoError(err)

	statusCode, _ := suite.doRequest(handler, httptrigger.InternalHealthPath)
	suite.Require().Equal(http.StatusOK, statusCode)
	suite.Require().Equal(1, upstream.hits)

	// the authenticator must not be consulted for the readiness probe
	suite.Require().Equal(0, authenticator.calls)
}

// TestAuthOnlyHandler verifies /auth returns 200 when authorized, the reject code otherwise, and 404 elsewhere.
func (suite *ServerTestSuite) TestAuthOnlyHandler() {
	suite.Run("authorized returns 200", func() {
		handler := newAuthOnlyHandler(&fakeAuthenticator{authorized: true})
		statusCode, _ := suite.doRequest(handler, "/auth")
		suite.Require().Equal(http.StatusOK, statusCode)
	})

	suite.Run("unauthorized returns the reject code", func() {
		handler := newAuthOnlyHandler(&fakeAuthenticator{authorized: false, rejectCode: http.StatusUnauthorized})
		statusCode, _ := suite.doRequest(handler, "/auth")
		suite.Require().Equal(http.StatusUnauthorized, statusCode)
	})

	suite.Run("other paths are not routed", func() {
		handler := newAuthOnlyHandler(&fakeAuthenticator{authorized: true})
		statusCode, _ := suite.doRequest(handler, "/invoke")
		suite.Require().Equal(http.StatusNotFound, statusCode)
	})
}

func (suite *ServerTestSuite) newTestUpstreamStub() *upstreamStub {
	return suite.newTestUpstreamStubWithBody("upstream-response")
}

// newTestUpstreamStubWithBody serves a distinguishable body so a test can tell which upstream was hit.
func (suite *ServerTestSuite) newTestUpstreamStubWithBody(body string) *upstreamStub {
	stub := &upstreamStub{}
	stub.server = httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		stub.hits++
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte(body))
	}))
	return stub
}

// doRequest drives the handler in-memory - no listener, no port, no client - and returns what it wrote.
func (suite *ServerTestSuite) doRequest(handler http.Handler, path string) (int, string) {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder.Code, recorder.Body.String()
}

func TestServerTestSuite(t *testing.T) {
	suite.Run(t, new(ServerTestSuite))
}
