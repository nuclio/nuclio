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
	"context"
	"path"
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

	suite.Runtime = "golang"
	suite.FunctionDir = path.Join(suite.GetProcessorBuildDir(), "golang", "test")
}

func (suite *TestSuite) TestBuildFile() {
	deployOptions := suite.GetDeployOptions("incrementor",
		suite.GetFunctionPath("_incrementor", "incrementor.go"))

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestBody:          "abcdef",
			ExpectedResponseBody: "bcdefg",
		})
}

func (suite *TestSuite) TestBuildFileWithDeps() {
	deployOptions := suite.GetDeployOptions("slugger",
		suite.GetFunctionPath("_slugger", "slugger.go"))

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestBody:          "make a slug",
			ExpectedResponseBody: "make-a-slug",
		})
}

func (suite *TestSuite) TestBuildDir() {
	deployOptions := suite.GetDeployOptions("incrementor",
		suite.GetFunctionPath("_incrementor"))

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestBody:          "abcdef",
			ExpectedResponseBody: "bcdefg",
		})
}

func (suite *TestSuite) TestBuildURL() {

	// start an HTTP server to serve the reverser py
	// TODO: needs to be made unique (find a free port)
	httpServer := buildsuite.HTTPFileServer{}
	httpServer.Start(":6666",
		path.Join(suite.FunctionDir, "_incrementor", "incrementor.go"),
		"/some/path/incrementor.go")

	defer httpServer.Shutdown(context.TODO())

	deployOptions := suite.GetDeployOptions("incrementor",
		"http://localhost:6666/some/path/incrementor.go")

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestBody:          "abcdef",
			ExpectedResponseBody: "bcdefg",
		})
}

func (suite *TestSuite) TestBuildInvalidFunctionPath() {
	var err error

	deployOptions := suite.GetDeployOptions("invalid", "invalidpath")

	_, err = suite.Platform.BuildFunction(&platform.BuildOptions{
		Logger:         deployOptions.Logger,
		FunctionConfig: deployOptions.FunctionConfig,
	})

	suite.Require().Contains(errors.Cause(err).Error(), "invalidpath")
}

func (suite *TestSuite) TestBuildCustomImageName() {
	deployOptions := suite.GetDeployOptions("incrementor",
		suite.GetFunctionPath("_incrementor"))

	// update image name
	deployOptions.FunctionConfig.Spec.Build.ImageName = "myname" + suite.TestID

	deployResult := suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestBody:          "abcdef",
			ExpectedResponseBody: "bcdefg",
		})

	suite.Require().Equal(deployOptions.FunctionConfig.Spec.Build.ImageName+":latest", deployResult.ImageName)
}

func (suite *TestSuite) TestBuildWithCompilationError() {
	var err error

	deployOptions := suite.GetDeployOptions("compilation-error", "_compilation-error")
	deployOptions.FunctionConfig.Spec.Build.NoBaseImagesPull = true
	deployOptions.FunctionConfig.Spec.Build.NuclioSourceDir = suite.GetNuclioSourceDir()

	_, err = suite.Platform.BuildFunction(&platform.BuildOptions{
		Logger:         deployOptions.Logger,
		FunctionConfig: deployOptions.FunctionConfig,
	})

	suite.Require().Error(err)

	buffer := bytes.Buffer{}

	// write an err stack
	errors.PrintErrorStack(&buffer, err, 10)

	// error should yell about "fmt.NotAFunction" not existing
	suite.Require().Contains(buffer.String(), "fmt.NotAFunction")
}

func (suite *TestSuite) TestBuildDirWithFunctionConfig() {
	deployOptions := suite.GetDeployOptions("incrementor",
		suite.GetFunctionPath("_incrementor-with-function-config"))

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestBody:          "abcdef",
			ExpectedResponseBody: "bcdefg",
		})
}

func (suite *TestSuite) TestBuildDirWithInlineFunctionConfig() {
	deployOptions := suite.GetDeployOptions("incrementor",
		suite.GetFunctionPath("_incrementor-with-inline-function-config", "incrementor.go"))

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestBody:          "abcdef",
			ExpectedResponseBody: "bcdefg",
		})
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestSuite))
}
