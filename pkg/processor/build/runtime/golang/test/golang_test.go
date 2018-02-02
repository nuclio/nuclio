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
	"bytes"
	"testing"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime/test/suite"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	buildsuite.TestSuite
}

func (suite *TestSuite) SetupSuite() {
	suite.TestSuite.SetupSuite()

	suite.TestSuite.RuntimeSuite = suite
}

func (suite *TestSuite) TestBuildWithCompilationError() {
	var err error

	deployOptions := suite.GetDeployOptions("compilation-error",
		suite.GetFunctionPath(suite.GetTestFunctionsDir(), "common", "compilation-error", "golang", "compilation-error.go"))

	deployOptions.FunctionConfig.Spec.Build.NoBaseImagesPull = true

	_, err = suite.Platform.BuildFunction(&platform.BuildOptions{
		Logger:         deployOptions.Logger,
		FunctionConfig: deployOptions.FunctionConfig,
		PlatformName:   suite.Platform.GetName(),
	})

	suite.Require().Error(err)

	buffer := bytes.Buffer{}

	// write an err stack
	errors.PrintErrorStack(&buffer, err, 10)

	// error should yell about "fmt.NotAFunction" not existing
	suite.Require().Contains(buffer.String(), "fmt.NotAFunction")
}

func (suite *TestSuite) TestBuildWithContextInitializer() {
	deployOptions := suite.GetDeployOptions("context-init",
		suite.GetFunctionPath(suite.GetTestFunctionsDir(), "common", "context-init", "golang", "contextinit.go"))

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestMethod:        "POST",
			RequestBody:          "",
			ExpectedResponseBody: "User data initialized from context: 0",
		})
}

func (suite *TestSuite) GetFunctionInfo(functionName string) buildsuite.FunctionInfo {
	functionInfo := buildsuite.FunctionInfo{
		Runtime: "golang",
	}

	switch functionName {

	case "reverser":
		functionInfo.Path = []string{suite.GetTestFunctionsDir(), "common", "reverser", "golang", "reverser.go"}

	case "json-parser-with-function-config":
		functionInfo.Path = []string{suite.GetTestFunctionsDir(), "common", "json-parser-with-function-config", "golang"}

	case "json-parser-with-inline-function-config":
		functionInfo.Path = []string{suite.GetTestFunctionsDir(), "common", "json-parser-with-inline-function-config", "golang", "parser.go"}

	case "long-initialization":
		functionInfo.Path = []string{suite.GetTestFunctionsDir(), "common", "long-initialization", "golang", "sleepy.go"}

	default:
		suite.Logger.InfoWith("Test skipped", "functionName", functionName)

		functionInfo.Skip = true
	}

	return functionInfo
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestSuite))
}
