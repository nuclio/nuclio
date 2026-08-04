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
	"testing"

	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/auth/authproxy"

	"github.com/stretchr/testify/suite"
)

type ConfigTestSuite struct {
	suite.Suite
}

// TestValidateRoutes verifies the per-port rules and the per-mode rules: reverseProxy needs an upstream on
// every route, authOnly is reached over a single loopback address and must not be given one at all.
func (suite *ConfigTestSuite) TestValidateRoutes() {
	for _, testCase := range []struct {
		name                 string
		mode                 auth.ProxyMode
		routes               []authproxy.Route
		expectedErrorMessage string
	}{
		{
			name:   "reverseProxy with a single route",
			mode:   auth.ProxyModeReverseProxy,
			routes: []authproxy.Route{{ListenPort: 8080, UpstreamURL: "http://127.0.0.1:6080"}},
		},
		{
			name: "reverseProxy fronting several ports",
			mode: auth.ProxyModeReverseProxy,
			routes: []authproxy.Route{
				{ListenPort: 8080, UpstreamURL: "http://127.0.0.1:6080"},
				{ListenPort: 6081, UpstreamURL: "http://127.0.0.1:8050"},
			},
		},
		{
			name:   "authOnly with a port only",
			mode:   auth.ProxyModeAuthOnly,
			routes: []authproxy.Route{{ListenPort: 8080}},
		},
		{
			name:                 "no routes at all",
			mode:                 auth.ProxyModeReverseProxy,
			routes:               []authproxy.Route{},
			expectedErrorMessage: "At least one route must be provided",
		},
		{
			name:                 "listen port out of range",
			mode:                 auth.ProxyModeReverseProxy,
			routes:               []authproxy.Route{{ListenPort: 70000, UpstreamURL: "http://127.0.0.1:6080"}},
			expectedErrorMessage: "Invalid listen port",
		},
		{
			name:                 "well-known listen port",
			mode:                 auth.ProxyModeReverseProxy,
			routes:               []authproxy.Route{{ListenPort: 80, UpstreamURL: "http://127.0.0.1:6080"}},
			expectedErrorMessage: "reserved for well-known services",
		},
		{
			name: "duplicate listen port",
			mode: auth.ProxyModeReverseProxy,
			routes: []authproxy.Route{
				{ListenPort: 8080, UpstreamURL: "http://127.0.0.1:6080"},
				{ListenPort: 8080, UpstreamURL: "http://127.0.0.1:8050"},
			},
			expectedErrorMessage: "Duplicate listen port: 8080",
		},
		{
			name: "reverseProxy route without an upstream",
			mode: auth.ProxyModeReverseProxy,
			routes: []authproxy.Route{
				{ListenPort: 8080, UpstreamURL: "http://127.0.0.1:6080"},
				{ListenPort: 6081},
			},
			expectedErrorMessage: "Upstream URL must be provided for listen port: 6081",
		},
		{
			name:                 "authOnly with more than one route",
			mode:                 auth.ProxyModeAuthOnly,
			routes:               []authproxy.Route{{ListenPort: 8080}, {ListenPort: 6081}},
			expectedErrorMessage: "authOnly mode requires exactly one route, got 2",
		},
		{
			name:                 "authOnly given an upstream",
			mode:                 auth.ProxyModeAuthOnly,
			routes:               []authproxy.Route{{ListenPort: 8080, UpstreamURL: "http://127.0.0.1:6080"}},
			expectedErrorMessage: "Upstream URL must not be provided in authOnly mode",
		},
	} {
		suite.Run(testCase.name, func() {
			err := validateRoutes(&Config{Mode: testCase.mode, Routes: testCase.routes})
			if testCase.expectedErrorMessage == "" {
				suite.Require().NoError(err)
				return
			}
			suite.Require().Error(err)
			suite.Require().Contains(err.Error(), testCase.expectedErrorMessage)
		})
	}
}

func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}
