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
	"context"
	"path"
	"testing"

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/processor/build"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime/test/suite"
	"github.com/nuclio/nuclio/pkg/processor/eventsource/http/test/suite"
	"github.com/nuclio/nuclio/pkg/processor/test/suite"

	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	buildsuite.TestSuite
}

func (suite *TestSuite) TestBuildFile() {
	buildOptions := build.Options{
		FunctionName: "reverser",
		FunctionPath: path.Join(suite.getPythonDir(), "reverser", "reverser.py"),
	}

	suite.FunctionBuildRunAndRequest(&buildOptions,
		nil,
		&httpsuite.Request{
			RequestMethod:        "POST",
			RequestBody:          "abcdef",
			ExpectedResponseBody: "fedcba",
		})
}

func (suite *TestSuite) TestBuildDir() {
	buildOptions := build.Options{
		FunctionName: "reverser",
		FunctionPath: path.Join(suite.getPythonDir(), "reverser"),
		Runtime:      "python",
	}

	suite.FunctionBuildRunAndRequest(&buildOptions,
		nil,
		&httpsuite.Request{
			RequestMethod:        "POST",
			RequestBody:          "abcdef",
			ExpectedResponseBody: "fedcba",
		})
}

func (suite *TestSuite) TestBuildDirWithProcessorYAML() {
	buildOptions := build.Options{
		FunctionName: "reverser",
		FunctionPath: path.Join(suite.getPythonDir(), "reverser-with-processor"),
		Runtime:      "python",
	}

	runOptions := processorsuite.RunOptions{
		RunOptions: dockerclient.RunOptions{
			Ports: map[int]int{8888: 8888},
		},
	}

	suite.FunctionBuildRunAndRequest(&buildOptions,
		&runOptions,
		&httpsuite.Request{
			RequestPort:          8888,
			RequestBody:          "abcdef",
			ExpectedResponseBody: "fedcba",
		})
}

func (suite *TestSuite) TestBuildURL() {

	// start an HTTP server to serve the reverser py
	// TODO: needs to be made unique (find a free port)
	httpServer := buildsuite.HTTPFileServer{}
	httpServer.Start(":7777",
		path.Join(suite.getPythonDir(), "reverser", "reverser.py"),
		"/some/path/reverser.py")

	defer httpServer.Shutdown(context.TODO())

	buildOptions := build.Options{
		FunctionName: "reverser",
		FunctionPath: "http://localhost:7777/some/path/reverser.py",
	}

	suite.FunctionBuildRunAndRequest(&buildOptions,
		nil,
		&httpsuite.Request{
			RequestMethod:        "POST",
			RequestBody:          "abcdef",
			ExpectedResponseBody: "fedcba",
		})
}

func (suite *TestSuite) TestBuildDirWithBuildYAML() {
	buildOptions := build.Options{
		FunctionName: "parser",
		FunctionPath: path.Join(suite.getPythonDir(), "json-parser-with-build"),
		Runtime:      "python",
	}

	suite.FunctionBuildRunAndRequest(&buildOptions,
		nil,
		&httpsuite.Request{
			RequestBody:          `{"a": 100, "return_this": "returned value"}`,
			ExpectedResponseBody: "returned value",
		})
}

func (suite *TestSuite) TestBuildURLWithInlineBlock() {

	// start an HTTP server to serve the reverser py
	// TODO: needs to be made unique (find a free port)
	httpServer := buildsuite.HTTPFileServer{}
	httpServer.Start(":7777",
		path.Join(suite.getPythonDir(), "json-parser-with-inline", "parser.py"),
		"/some/path/parser.py")

	defer httpServer.Shutdown(context.TODO())

	buildOptions := build.Options{
		FunctionName: "parser",
		FunctionPath: "http://localhost:7777/some/path/parser.py",
	}

	runOptions := processorsuite.RunOptions{
		RunOptions: dockerclient.RunOptions{
			Ports: map[int]int{7979: 7979},
		},
	}

	suite.FunctionBuildRunAndRequest(&buildOptions,
		&runOptions,
		&httpsuite.Request{
			RequestPort:          7979,
			RequestBody:          `{"a": 100, "return_this": "returned value"}`,
			ExpectedResponseBody: "returned value",
		})
}

func (suite *TestSuite) getPythonDir() string {
	return path.Join(suite.GetProcessorBuildDir(), "python", "test")
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestSuite))
}
