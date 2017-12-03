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

	suite.Runtime = "shell"
	suite.FunctionDir = path.Join(suite.GetProcessorBuildDir(), "shell", "test")
}

func (suite *TestSuite) TestBuildScriptFile() {
	deployOptions := suite.GetDeployOptions("reverser",
		suite.GetFunctionPath("reverser", "reverser.sh"))

	deployOptions.FunctionConfig.Spec.Handler = "reverser.sh:main"

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestMethod:        "POST",
			RequestBody:          "abcdef",
			ExpectedResponseBody: "fedcba",
		})
}

func (suite *TestSuite) TestBuildScriptDir() {
	deployOptions := suite.GetDeployOptions("reverser",
		suite.GetFunctionPath("reverser"))

	deployOptions.FunctionConfig.Spec.Handler = "reverser.sh:main"

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestMethod:        "POST",
			RequestBody:          "abcdef",
			ExpectedResponseBody: "fedcba",
		})
}

func (suite *TestSuite) TestBuildScriptURL() {

	// start an HTTP server to serve the reverser py
	// TODO: needs to be made unique (find a free port)
	httpServer := buildsuite.HTTPFileServer{}
	httpServer.Start(":7777",
		path.Join(suite.FunctionDir, "reverser", "reverser.sh"),
		"/some/path/reverser.sh")

	defer httpServer.Shutdown(context.TODO())

	deployOptions := suite.GetDeployOptions("reverser",
		"http://localhost:7777/some/path/reverser.sh")

	deployOptions.FunctionConfig.Spec.Handler = "reverser.sh:main"

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestMethod:        "POST",
			RequestBody:          "abcdef",
			ExpectedResponseBody: "fedcba",
		})
}

func (suite *TestSuite) TestBuildBinaryWithStdin() {
	deployOptions := suite.GetDeployOptions("reverser", "/dev/null")

	deployOptions.FunctionConfig.Spec.Handler = "rev"

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestMethod:        "POST",
			RequestBody:          "abcdef",
			ExpectedResponseBody: "fedcba",
		})
}

func (suite *TestSuite) TestBuildBinaryWithArguments() {
	deployOptions := suite.GetDeployOptions("echoer", "/dev/null")

	deployOptions.FunctionConfig.Spec.Handler = "echo"
	deployOptions.FunctionConfig.Spec.RuntimeAttributes = map[string]interface{}{
		"arguments": "abcdef",
	}

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestMethod:        "GET",
			ExpectedResponseBody: "abcdef\n",
		})
}

func (suite *TestSuite) TestBuildBinaryWithArgumentsFromEvent() {
	deployOptions := suite.GetDeployOptions("echoer", "/dev/null")

	deployOptions.FunctionConfig.Spec.Handler = "echo"
	deployOptions.FunctionConfig.Spec.RuntimeAttributes = map[string]interface{}{
		"arguments": "abcdef",
	}

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestMethod: "GET",
			RequestHeaders: map[string]string{
				"x-nuclio-arguments": "123456",
			},
			ExpectedResponseBody: "123456\n",
		})
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestSuite))
}
