/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the Licensg.
You may obtain a copy of the License at

    http://www.apachg.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the Licensg.
*/

package build

import (
	"encoding/base64"
	"io/ioutil"
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	suite.Suite
	Logger  logger.Logger
	Builder *Builder
	TestID  string
}

// SetupSuite is called for suite setup
func (suite *TestSuite) SetupSuite() {
	var err error

	suite.Logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)
}

// SetupTest is called before each test in the suite
func (suite *TestSuite) SetupTest() {
	var err error
	suite.TestID = xid.New().String()

	suite.Builder, err = NewBuilder(suite.Logger, nil)
	if err != nil {
		suite.Fail("Instantiating Builder failed:", err)
	}

	createFunctionOptions := &platform.CreateFunctionOptions{
		Logger:         suite.Logger,
		FunctionConfig: *functionconfig.NewConfig(),
	}

	createFunctionBuildOptions := &platform.CreateFunctionBuildOptions{
		Logger:         createFunctionOptions.Logger,
		FunctionConfig: createFunctionOptions.FunctionConfig,
	}

	suite.Builder.options = createFunctionBuildOptions
}

// Make sure that "Builder.getRuntimeName" properly reads the runtime name from the configuration given by the user
func (suite *TestSuite) TestGetRuntimeNameFromConfig() {
	suite.Builder.options.FunctionConfig.Spec.Runtime = "foo"
	runtimeName, err := suite.Builder.getRuntimeName()

	if err != nil {
		suite.Fail(err.Error())
	}

	suite.Require().Equal("foo", runtimeName)
}

// Make sure that "Builder.getRuntimeName" properly reads the runtime name from the build path if not set by the user
func (suite *TestSuite) TestGetPythonRuntimeNameFromBuildPath() {
	suite.Builder.options.FunctionConfig.Spec.Runtime = ""
	suite.Builder.options.FunctionConfig.Spec.Build.Path = "/foo.py"
	runtimeName, err := suite.Builder.getRuntimeName()

	suite.Require().NoError(err)

	suite.Require().Equal("python", runtimeName)
}

// Make sure that "Builder.getRuntimeName" properly reads the runtime name from the build path if not set by the user
func (suite *TestSuite) TestGetGoRuntimeNameFromBuildPath() {
	suite.Builder.options.FunctionConfig.Spec.Runtime = ""
	suite.Builder.options.FunctionConfig.Spec.Build.Path = "/foo.go"
	runtimeName, err := suite.Builder.getRuntimeName()

	suite.Require().NoError(err)

	suite.Require().Equal("golang", runtimeName)
}

// Make sure that "Builder.getRuntimeName" returns an error if the user sends an unknown file extension without runtime
func (suite *TestSuite) TestGetRuntimeNameFromBuildPathFailsOnUnknownExtension() {
	suite.Builder.options.FunctionConfig.Spec.Runtime = ""
	suite.Builder.options.FunctionConfig.Spec.Build.Path = "/foo.bar"
	_, err := suite.Builder.getRuntimeName()

	suite.Require().Error(err, "Unsupported file extension: %s", "bar")
}

// Make sure that "Builder.getRuntimeName()" fails when the runtime is empty, and the build path is a directory
func (suite *TestSuite) TestGetRuntimeNameFromBuildDirNoRuntime() {
	suite.Builder.options.FunctionConfig.Spec.Runtime = ""
	suite.Builder.options.FunctionConfig.Spec.Build.Path = "/user/"
	_, err := suite.Builder.getRuntimeName()

	if err == nil {
		suite.Fail("Builder.getRuntimeName() should fail when given a directory for a build path and no runtime")
	}
}

func (suite *TestSuite) TestWriteFunctionSourceCodeToTempFileWritesReturnsFilePath() {
	functionSourceCode := "echo foo"
	encodedFunctionSourceCode := base64.StdEncoding.EncodeToString([]byte(functionSourceCode))
	suite.Builder.options.FunctionConfig.Spec.Runtime = "shell"
	suite.Builder.options.FunctionConfig.Spec.Build.FunctionSourceCode = encodedFunctionSourceCode
	suite.Builder.options.FunctionConfig.Spec.Build.Path = ""

	err := suite.Builder.createTempDir()
	suite.Assert().NoError(err)
	defer suite.Builder.cleanupTempDir()

	tempPath, err := suite.Builder.writeFunctionSourceCodeToTempFile(suite.Builder.options.FunctionConfig.Spec.Build.FunctionSourceCode)
	suite.Assert().NoError(err)
	suite.NotNil(tempPath)

	resultSourceCode, err := ioutil.ReadFile(tempPath)
	suite.Assert().NoError(err)

	suite.Assert().Equal(functionSourceCode, string(resultSourceCode))
}

func (suite *TestSuite) TestWriteFunctionSourceCodeToTempFileFailsOnUnknownExtension() {
	suite.Builder.options.FunctionConfig.Spec.Runtime = "bar"
	suite.Builder.options.FunctionConfig.Spec.Build.FunctionSourceCode = base64.StdEncoding.EncodeToString([]byte("echo foo"))
	suite.Builder.options.FunctionConfig.Spec.Build.Path = ""

	err := suite.Builder.createTempDir()
	suite.Assert().NoError(err)
	defer suite.Builder.cleanupTempDir()

	_, err = suite.Builder.writeFunctionSourceCodeToTempFile(suite.Builder.options.FunctionConfig.Spec.Build.FunctionSourceCode)
	suite.Assert().Error(err)
}

func (suite *TestSuite) TestGetImage() {

	// user specified
	suite.Builder.options.FunctionConfig.Spec.Build.Image = "userSpecified"
	suite.Require().Equal("userSpecified", suite.Builder.getImage())

	// set function name and clear image name
	suite.Builder.options.FunctionConfig.Meta.Name = "test"
	suite.Builder.options.FunctionConfig.Spec.Build.Image = ""

	// registry has no repository - should see "nuclio/" as repository
	suite.Builder.options.FunctionConfig.Spec.Build.Registry = "localhost:5000"
	suite.Require().Equal("nuclio/processor-test", suite.Builder.getImage())

	// registry has a repository - should not see "nuclio/" as repository
	suite.Builder.options.FunctionConfig.Spec.Build.Registry = "registry.hub.docker.com/foo"
	suite.Require().Equal("processor-test", suite.Builder.getImage())

	// registry has a repository - should not see "nuclio/" as repository
	suite.Builder.options.FunctionConfig.Spec.Build.Registry = "index.docker.io/foo"
	suite.Require().Equal("processor-test", suite.Builder.getImage())
}

func TestBuilderSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestSuite))
}
