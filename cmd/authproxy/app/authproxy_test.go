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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nuclio/nuclio/pkg/auth"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type AuthProxyTestSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *AuthProxyTestSuite) SetupTest() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("auth-proxy-test")
	suite.Require().NoError(err)
}

// TestModeListenAddress verifies the only difference between the modes: reverse-proxy is exposed on the
// configurable port, auth-only is bound to loopback (reachable only from within the pod).
func (suite *AuthProxyTestSuite) TestModeListenAddress() {
	for _, testCase := range []struct {
		name                  string
		mode                  auth.ProxyMode
		expectedListenAddress string
	}{
		{name: "reverse-proxy is exposed", mode: auth.ProxyModeReverseProxy, expectedListenAddress: ":8080"},
		{name: "auth-only is loopback", mode: auth.ProxyModeAuthOnly, expectedListenAddress: "127.0.0.1:8080"},
	} {
		suite.Run(testCase.name, func() {
			server, err := newServer(suite.logger, testCase.mode, 8080, "http://127.0.0.1:6080", "", "")
			suite.Require().NoError(err)
			suite.Require().Equal(testCase.expectedListenAddress, server.httpServer.Addr)
		})
	}
}

// TestAuthOnlyHandlerRouting verifies only /auth is served (reserved for NUC-837); any other path 404s.
func (suite *AuthProxyTestSuite) TestAuthOnlyHandlerRouting() {
	handlerServer := httptest.NewServer(newAuthOnlyHandler(suite.logger))
	defer handlerServer.Close()

	for _, testCase := range []struct {
		name               string
		path               string
		expectedStatusCode int
	}{
		{name: "auth endpoint reserved for NUC-837", path: "/auth", expectedStatusCode: http.StatusNotImplemented},
		{name: "root not routed", path: "/", expectedStatusCode: http.StatusNotFound},
		{name: "arbitrary path not routed", path: "/invoke", expectedStatusCode: http.StatusNotFound},
	} {
		suite.Run(testCase.name, func() {
			response, err := http.Get(handlerServer.URL + testCase.path)
			suite.Require().NoError(err)
			defer response.Body.Close()

			suite.Require().Equal(testCase.expectedStatusCode, response.StatusCode)
		})
	}
}

func (suite *AuthProxyTestSuite) TestUnknownModeRejected() {
	_, err := newServer(suite.logger, "unknown-mode", 8080, "http://127.0.0.1:6080", "", "")
	suite.Require().Error(err)
	suite.Require().Contains(err.Error(), "Unknown auth-proxy mode")
}

func TestAuthProxyTestSuite(t *testing.T) {
	suite.Run(t, new(AuthProxyTestSuite))
}
