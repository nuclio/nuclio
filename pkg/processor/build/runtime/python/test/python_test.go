/*
Copyright 2017 The Nuclio Authors.

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

package test

import (
	"path"
	"testing"

	"github.com/nuclio/nuclio/pkg/processor/build/runtime/test/suite"

	"github.com/stretchr/testify/suite"
	"github.com/nuclio/nuclio/pkg/processor/eventsource/http/test/suite"
)

type TestSuite struct {
	buildsuite.TestSuite
}

func (suite *TestSuite) TestBuildFile() {
	// suite.T().Skip()

	suite.FunctionBuildRunAndRequest("reverser",
		path.Join(suite.getPythonDir(), "reverser", "reverser.py"),
		"",
		map[int]int{8080: 8080},
		&httpsuite.Request{
			RequestPort: 8080,
			RequestPath: "/",
			RequestMethod: "POST",
			RequestBody: "abcdef",
			ExpectedResponseBody: "fedcba",
		})
}

func (suite *TestSuite) TestBuildDir() {
	// suite.T().Skip()

	suite.FunctionBuildRunAndRequest("reverser",
		path.Join(suite.getPythonDir(), "reverser"),
		"python",
		map[int]int{8080: 8080},
		&httpsuite.Request{
			RequestPort: 8080,
			RequestPath: "/",
			RequestMethod: "POST",
			RequestBody: "abcdef",
			ExpectedResponseBody: "fedcba",
		})
}

func (suite *TestSuite) TestBuildDirWithProcessorYAML() {
	// suite.T().Skip()

	suite.FunctionBuildRunAndRequest("reverser",
		path.Join(suite.getPythonDir(), "reverser-with-processor"),
		"python",
		map[int]int{8888: 8888},
		&httpsuite.Request{
			RequestPort: 8888,
			RequestPath: "/",
			RequestMethod: "POST",
			RequestBody: "abcdef",
			ExpectedResponseBody: "fedcba",
		})
}

func (suite *TestSuite) TestBuildURL() {
	// suite.T().Skip()

	// start an HTTP server to serve the reverser py
	// TODO: needs to be made unique (find a free port)
	httpServer := buildsuite.HTTPFileServer{}
	httpServer.Start(":7777",
		path.Join(suite.getPythonDir(), "reverser", "reverser.py"),
		"/some/path/reverser.py")

	defer httpServer.Shutdown(nil)

	suite.FunctionBuildRunAndRequest("reverser",
		"http://localhost:7777/some/path/reverser.py",
		"",
		map[int]int{8080: 8080},
		&httpsuite.Request{
			RequestPort: 8080,
			RequestPath: "/",
			RequestMethod: "POST",
			RequestBody: "abcdef",
			ExpectedResponseBody: "fedcba",
		})
}

func (suite *TestSuite) TestBuildDirWithBuildYAML() {
	// suite.T().Skip()

	suite.FunctionBuildRunAndRequest("parser",
		path.Join(suite.getPythonDir(), "json-parser-with-build"),
		"python",
		map[int]int{8080: 8080},
		&httpsuite.Request{
			RequestPort: 8080,
			RequestPath: "/",
			RequestMethod: "POST",
			RequestBody: `{"a": 100, "return_this": "returned value"}`,
			ExpectedResponseBody: "returned value",
		})
}

func (suite *TestSuite) TestBuildURLWithInlineBlock() {
	// suite.T().Skip()

	// start an HTTP server to serve the reverser py
	// TODO: needs to be made unique (find a free port)
	httpServer := buildsuite.HTTPFileServer{}
	httpServer.Start(":7777",
		path.Join(suite.getPythonDir(), "json-parser-with-inline", "parser.py"),
		"/some/path/parser.py")

	defer httpServer.Shutdown(nil)

	suite.FunctionBuildRunAndRequest("parser",
		"http://localhost:7777/some/path/parser.py",
		"",
		map[int]int{7979: 7979},
		&httpsuite.Request{
			RequestPort: 7979,
			RequestPath: "/",
			RequestMethod: "POST",
			RequestBody: `{"a": 100, "return_this": "returned value"}`,
			ExpectedResponseBody: "returned value",
		})
}

func (suite *TestSuite) getPythonDir() string {
	return path.Join(suite.GetProcessorBuildDir(), "python", "test")
}

func TestTestSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}
