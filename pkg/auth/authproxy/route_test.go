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
	"testing"

	"github.com/stretchr/testify/suite"
)

type RouteTestSuite struct {
	suite.Suite
}

// TestParseRoutes verifies the listenPort=upstreamURL syntax, including the bare-port form authOnly uses.
func (suite *RouteTestSuite) TestParseRoutes() {
	for _, testCase := range []struct {
		name           string
		routes         string
		expectedRoutes []Route
		expectError    bool
	}{
		{
			name:           "single route",
			routes:         "8080=http://127.0.0.1:6080",
			expectedRoutes: []Route{{ListenPort: 8080, UpstreamURL: "http://127.0.0.1:6080"}},
		},
		{
			name:   "function route followed by sidecar routes",
			routes: "8080=http://127.0.0.1:6080,6081=http://127.0.0.1:8050,6082=http://127.0.0.1:9000",
			expectedRoutes: []Route{
				{ListenPort: 8080, UpstreamURL: "http://127.0.0.1:6080"},
				{ListenPort: 6081, UpstreamURL: "http://127.0.0.1:8050"},
				{ListenPort: 6082, UpstreamURL: "http://127.0.0.1:9000"},
			},
		},
		{
			name:           "bare port yields no upstream",
			routes:         "8080",
			expectedRoutes: []Route{{ListenPort: 8080}},
		},
		{
			name:           "trailing separator yields no upstream",
			routes:         "8080=",
			expectedRoutes: []Route{{ListenPort: 8080}},
		},
		{
			name:           "surrounding whitespace is tolerated",
			routes:         " 8080 = http://127.0.0.1:6080 , 6081 = http://127.0.0.1:8050 ",
			expectedRoutes: []Route{{ListenPort: 8080, UpstreamURL: "http://127.0.0.1:6080"}, {ListenPort: 6081, UpstreamURL: "http://127.0.0.1:8050"}},
		},
		{
			name:           "empty string yields no routes",
			routes:         "",
			expectedRoutes: nil,
		},
		{
			name:           "empty string with spaces yields no routes",
			routes:         "   ",
			expectedRoutes: nil,
		},
		{
			name:        "non-numeric listen port is rejected",
			routes:      "not-a-port=http://127.0.0.1:6080",
			expectError: true,
		},
	} {
		suite.Run(testCase.name, func() {
			parsedRoutes, err := ParseRoutes(testCase.routes)
			if testCase.expectError {
				suite.Require().Error(err)
				return
			}
			suite.Require().NoError(err)
			suite.Require().Equal(testCase.expectedRoutes, parsedRoutes)
		})
	}
}

// TestFormatRoutesRoundTrip verifies the controller renders what the auth-proxy parses back.
func (suite *RouteTestSuite) TestFormatRoutesRoundTrip() {
	for _, testCase := range []struct {
		name           string
		routes         []Route
		expectedRoutes string
	}{
		{
			name:           "function route only",
			routes:         []Route{LoopbackRoute(8080, 6080)},
			expectedRoutes: "8080=http://127.0.0.1:6080",
		},
		{
			name:           "function route plus sidecar routes",
			routes:         []Route{LoopbackRoute(8080, 6080), LoopbackRoute(6081, 8050)},
			expectedRoutes: "8080=http://127.0.0.1:6080,6081=http://127.0.0.1:8050",
		},
		{
			name:           "route without an upstream renders as a bare port",
			routes:         []Route{{ListenPort: 8080}},
			expectedRoutes: "8080",
		},
	} {
		suite.Run(testCase.name, func() {
			formattedRoutes := FormatRoutes(testCase.routes)
			suite.Require().Equal(testCase.expectedRoutes, formattedRoutes)

			parsedRoutes, err := ParseRoutes(formattedRoutes)
			suite.Require().NoError(err)
			suite.Require().Equal(testCase.routes, parsedRoutes)
		})
	}
}

func TestRouteTestSuite(t *testing.T) {
	suite.Run(t, new(RouteTestSuite))
}
