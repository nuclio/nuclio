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

	"github.com/nuclio/nuclio/pkg/processor/build/runtime/test/suite"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	buildsuite.TestSuite
}

func (suite *TestSuite) SetupSuite() {
	suite.TestSuite.SetupSuite()

	suite.Runtime = "python"
	suite.FunctionDir = path.Join(suite.GetProcessorBuildDir(), "python", "test")
}

func (suite *TestSuite) TestBuildFile() {
	deployOptions := suite.GetDeployOptions("reverser",
		suite.GetFunctionPath("reverser", "reverser.py"))

	deployOptions.FunctionConfig.Spec.Handler = "reverser:handler"

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestMethod:        "POST",
			RequestBody:          "abcdef",
			ExpectedResponseBody: "fedcba",
		})
}

func (suite *TestSuite) TestBuildDir() {
	deployOptions := suite.GetDeployOptions("reverser",
		suite.GetFunctionPath("reverser"))

	deployOptions.FunctionConfig.Spec.Handler = "reverser:handler"

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestMethod:        "POST",
			RequestBody:          "abcdef",
			ExpectedResponseBody: "fedcba",
		})
}

func (suite *TestSuite) TestBuildURL() {

	// start an HTTP server to serve the reverser py
	// TODO: needs to be made unique (find a free port)
	httpServer := buildsuite.HTTPFileServer{}
	httpServer.Start(":7777",
		path.Join(suite.FunctionDir, "reverser", "reverser.py"),
		"/some/path/reverser.py")

	defer httpServer.Shutdown(context.TODO())

	deployOptions := suite.GetDeployOptions("reverser",
		"http://localhost:7777/some/path/reverser.py")

	deployOptions.FunctionConfig.Spec.Handler = "reverser:handler"

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestMethod:        "POST",
			RequestBody:          "abcdef",
			ExpectedResponseBody: "fedcba",
		})
}

func (suite *TestSuite) TestBuildDirWithFunctionConfig() {
	deployOptions := suite.GetDeployOptions("",
		suite.GetFunctionPath("json-parser-with-function-config"))

	deployOptions.FunctionConfig.Spec.Runtime = ""
	deployOptions.FunctionConfig.Spec.Handler = ""
	deployOptions.FunctionConfig.Meta.Name = ""

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestBody:          `{"a": 100, "return_this": "returned value"}`,
			ExpectedResponseBody: "returned value",
		})
}

func (suite *TestSuite) TestBuildDirWithInlineFunctionConfig() {
	deployOptions := suite.GetDeployOptions("parser",
		suite.GetFunctionPath("json-parser-with-inline-function-config", "parser.py"))

	deployOptions.FunctionConfig.Spec.Handler = "parser:handler"

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestBody:          `{"a": 100, "return_this": "returned value"}`,
			ExpectedResponseBody: "returned value",
		})
}

func (suite *TestSuite) TestBuildPy2() {
	deployOptions := suite.GetDeployOptions("printer",
		suite.GetFunctionPath("python2", "printer.py"))

	deployOptions.FunctionConfig.Spec.Runtime = "python:2.7"
	deployOptions.FunctionConfig.Spec.Handler = "printer:handler"

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestMethod:        "POST",
			RequestBody:          "",
			ExpectedResponseBody: "printed",
		})
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestSuite))
}
