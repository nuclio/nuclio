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
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/nuclio/nuclio-sdk"
	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	suite.Suite
	Logger  nuclio.Logger
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

	suite.Builder, err = NewBuilder(suite.Logger)
	if err != nil {
		suite.Fail("Instantiating Builder failed:", err)
	}

	deployOptions := &platform.DeployOptions{
		Logger:         suite.Logger,
		FunctionConfig: *functionconfig.NewConfig(),
	}

	buildOptions := &platform.BuildOptions{
		Logger:         deployOptions.Logger,
		FunctionConfig: deployOptions.FunctionConfig,
	}

	suite.Builder.options = buildOptions
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

func (suite *TestSuite) TestGetImageSpecificCommandsReturnsEmptyOnUnknownBaseImage() {
	var expectedResult []string = nil
	result := suite.Builder.getImageSpecificCommands("foo")

	suite.Require().Equal(expectedResult, result)
}

func (suite *TestSuite) TestGetImageSpecificCommandsAddsCaCertificatesFlagForAlpine() {
	result := suite.Builder.getImageSpecificCommands("alpine")

	suite.Require().EqualValues([]string{"apk update && apk add --update ca-certificates && rm -rf /var/cache/apk/*",}, result)
}

func (suite *TestSuite) TestGetImageSpecificEnvVarsReturnsEmptyOnUnknownBaseImage() {
	var expectedResult []string = nil
	result := suite.Builder.getImageSpecificEnvVars("foo")

	suite.Require().Equal(expectedResult, result)
}

func (suite *TestSuite) TestGetImageSpecificEnvVarsAddsNonInteractiveFlagForJessie() {
	result := suite.Builder.getImageSpecificEnvVars("jessie")

	suite.Require().EqualValues([]string{"DEBIAN_FRONTEND noninteractive",}, result)
}

func (suite *TestSuite) TestReplaceBuildCommandDirectivesReturnsNewDirectives() {
	commands := []string{
		"test 1",
		"test 2",
	}
	result := suite.Builder.replaceBuildCommandDirectives(commands, "")
	commands = append(commands, "test 3")

	suite.Require().NotEqual(commands, result)
	suite.Require().EqualValues(commands, append(result, "test 3"))
}

func (suite *TestSuite) TestReplaceBuildCommandDirectivesOverwritesKnownDirectives() {
	commands := []string{
		"test 1",
		"@nuclio.noCache",
	}
	result := suite.Builder.replaceBuildCommandDirectives(commands, "foo")

	suite.Require().NotEqual(commands, result)
	suite.Require().Equal("RUN echo foo > /dev/null", result[1])
}

func (suite *TestSuite) TestReplaceBuildCommandDirectivesIgnoresUnknownDirectives() {
	commands := []string{
		"test 1",
		"@nuclio.bla",
	}
	result := suite.Builder.replaceBuildCommandDirectives(commands, "")

	suite.Require().EqualValues(commands, result)
}

func TestBuilderSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestSuite))
}
