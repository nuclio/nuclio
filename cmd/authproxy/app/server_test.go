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
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/auth"
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
}

func (suite *ServerTestSuite) SetupTest() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("auth-proxy-test")
	suite.Require().NoError(err)
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

// TestStartServerFailsOnOccupiedPort verifies that server.start returns an error when the port is already in use.
func (suite *ServerTestSuite) TestStartServerFailsOnOccupiedPort() {
	upstream := suite.newTestUpstreamStub()
	defer upstream.server.Close()

	handler, err := newReverseProxyHandler(suite.logger, upstream.server.URL, &fakeAuthenticator{authorized: true})
	suite.Require().NoError(err)

	// occupy a port so the listener deterministically fails to bind
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	suite.Require().NoError(err)
	defer occupied.Close() // nolint: errcheck

	err = newServer(suite.logger, occupied.Addr().String(), handler).start()
	suite.Require().Error(err)
}

// requireServing polls listenAddress until it accepts connections or the deadline elapses.
func (suite *ServerTestSuite) requireServing(listenAddress string) {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("tcp", listenAddress)
		if err == nil {
			conn.Close() // nolint: errcheck
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	suite.FailNow("Server never started listening", "listenAddress", listenAddress)
}

// doRequestToAddress performs an HTTP GET against an already-listening address (as opposed to doRequest,
// which spins up its own httptest.Server).
func (suite *ServerTestSuite) doRequestToAddress(listenAddress string, path string) (int, string) {
	response, err := http.Get(fmt.Sprintf("http://%s%s", listenAddress, path))
	suite.Require().NoError(err)
	defer func(body io.ReadCloser) {
		if err := body.Close(); err != nil {
			suite.logger.WarnWith("Failed to close response body", "err", err)
		}
	}(response.Body)

	body, err := io.ReadAll(response.Body)
	suite.Require().NoError(err)
	return response.StatusCode, string(body)
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
	stub := &upstreamStub{}
	stub.server = httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		stub.hits++
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte("upstream-response"))
	}))
	return stub
}

func (suite *ServerTestSuite) doRequest(handler http.Handler, path string) (int, string) {
	server := httptest.NewServer(handler)
	defer server.Close()

	response, err := http.Get(server.URL + path)
	suite.Require().NoError(err)
	defer func(body io.ReadCloser) {
		if err := body.Close(); err != nil {
			suite.logger.WarnWith("Failed to close response body", "err", err)
		}
	}(response.Body)

	body, err := io.ReadAll(response.Body)
	suite.Require().NoError(err)
	return response.StatusCode, string(body)
}

func TestServerTestSuite(t *testing.T) {
	suite.Run(t, new(ServerTestSuite))
}
