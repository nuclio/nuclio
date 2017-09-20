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

package python

import (
	"net/http"
	"path"
	"testing"

	"github.com/nuclio/nuclio/pkg/processor/test/suite"

	"github.com/stretchr/testify/suite"
)

type PythonBuildTestSuite struct {
	processorsuite.ProcessorTestSuite
}

func (suite *PythonBuildTestSuite) TestBuildFile() {
	suite.T().Skip()

	suite.FunctionBuildRunAndRequest("reverser",
		path.Join(suite.getProcessorTestPythonDir(), "reverser", "reverser.py"),
		"",
		map[int]int{8080: 8080},
		8080,
		"abcdef",
		"fedcba")
}

func (suite *PythonBuildTestSuite) TestBuildDir() {
	suite.T().Skip()

	suite.FunctionBuildRunAndRequest("reverser",
		path.Join(suite.getProcessorTestPythonDir(), "reverser"),
		"python",
		map[int]int{8080: 8080},
		8080,
		"abcdef",
		"fedcba")
}

func (suite *PythonBuildTestSuite) TestBuildDirWithProcessorYAML() {
	suite.T().Skip()

	suite.FunctionBuildRunAndRequest("reverser",
		path.Join(suite.getProcessorTestPythonDir(), "reverser-with-processor"),
		"python",
		map[int]int{8888: 8888},
		8888,
		"abcdef",
		"fedcba")
}

func (suite *PythonBuildTestSuite) TestBuildURL() {
	suite.T().Skip()

	// start an HTTP server to serve the reverser py
	// TODO: needs to be made unique (find a free port)
	httpServer := processorsuite.HTTPFileServer{}
	httpServer.Start(":7777",
		path.Join(suite.getProcessorTestPythonDir(), "reverser", "reverser.py"),
		"/some/path/reverser.py")

	defer httpServer.Shutdown(nil)

	suite.FunctionBuildRunAndRequest("reverser",
		"http://localhost:7777/some/path/reverser.py",
		"",
		map[int]int{8080: 8080},
		8080,
		"abcdef",
		"fedcba")
}

func (suite *PythonBuildTestSuite) TestBuildDirWithBuildYAML() {
	suite.T().Skip()

	suite.FunctionBuildRunAndRequest("parser",
		path.Join(suite.getProcessorTestPythonDir(), "json-parser-with-build"),
		"python",
		map[int]int{8080: 8080},
		8080,
		`{"a": 100, "return_this": "returned value"}`,
		"returned value")
}

func (suite *PythonBuildTestSuite) TestBuildURLWithInlineBlock() {
	suite.T().Skip()

	// start an HTTP server to serve the reverser py
	// TODO: needs to be made unique (find a free port)
	httpServer := processorsuite.HTTPFileServer{}
	httpServer.Start(":7777",
		path.Join(suite.getProcessorTestPythonDir(), "json-parser-with-inline", "parser.py"),
		"/some/path/parser.py")

	defer httpServer.Shutdown(nil)

	suite.FunctionBuildRunAndRequest("parser",
		"http://localhost:7777/some/path/parser.py",
		"",
		map[int]int{7979: 7979},
		7979,
		`{"a": 100, "return_this": "returned value"}`,
		"returned value")
}

func (suite *PythonBuildTestSuite) TestOutputs() {
	// suite.T().Skip()

	suite.BuildAndRunFunction("parser",
		path.Join(suite.getProcessorTestPythonDir(), "outputter"),
		"python",
		map[int]int{8080: 8080},
		func() bool {
			requestPort := 8080

			if !suite.SendRequestVerifyResponse(requestPort,
				"return_string",
				"a string",
				http.StatusOK) {
				return false
			}

			if !suite.SendRequestVerifyResponse(requestPort,
				"return_status_and_string",
				"a string after status",
				http.StatusCreated) {
				return false
			}

			if !suite.SendRequestVerifyResponse(requestPort,
				"return_dict",
				map[string]interface{}{"a": "dict", "b": "foo"},
				http.StatusOK) {
				return false
			}

			if !suite.SendRequestVerifyResponse(requestPort,
				"return_status_and_dict",
				map[string]interface{}{"a": "dict after status", "b": "foo"},
				http.StatusCreated) {
				return false
			}

			return true
		})
}



func (suite *PythonBuildTestSuite) getProcessorTestPythonDir() string {
	return path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "test", "python")
}

func TestPythonBuildTestSuite(t *testing.T) {
	suite.Run(t, new(PythonBuildTestSuite))
}
