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

package runtime

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/version"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type testSuite struct {
	suite.Suite
	logger logger.Logger
	ar     *AbstractRuntime
}

func (suite *testSuite) SetupTest() {
	var err error

	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	suite.ar = &AbstractRuntime{
		Logger:         suite.logger,
		FunctionConfig: &functionconfig.Config{},
	}

	version.Set(&version.Info{
		Label: "theLabel",
		Arch:  "theArch",
	})
}

func (suite *testSuite) TestGetBuildArgsDefault() {
	suite.getBuildArgsAndVerify("", "")
}

func (suite *testSuite) TestGetBuildArgsBaseImage() {
	suite.ar.FunctionConfig.Spec.Build.BaseImage = "someCustomBaseImage"

	suite.getBuildArgsAndVerify("someCustomBaseImage", "")
}

func (suite *testSuite) TestGetBuildArgsAlpineBaseImage() {
	suite.ar.FunctionConfig.Spec.Build.BaseImage = "alpine"

	suite.getBuildArgsAndVerify("alpine:3.6", "")
}

func (suite *testSuite) TestGetBuildArgsJessieBaseImage() {
	suite.ar.FunctionConfig.Spec.Build.BaseImage = "jessie"

	suite.getBuildArgsAndVerify("debian:jessie", "")
}

func (suite *testSuite) TestGetBuildArgsUnformattedOnbuild() {
	suite.ar.FunctionConfig.Spec.Build.OnbuildImage = "theOnbuildImage"

	suite.getBuildArgsAndVerify("", "theOnbuildImage")
}

func (suite *testSuite) TestGetBuildArgsFormattedOnbuild() {
	suite.ar.FunctionConfig.Spec.Build.OnbuildImage = "theOnbuildImage-{{.Label}}-{{.Arch}}"

	suite.getBuildArgsAndVerify("", "theOnbuildImage-theLabel-theArch")
}

func (suite *testSuite) getBuildArgsAndVerify(baseImage string, onbuildImage string) {

	// get build args with no custom onbuild and no custom base image
	buildArgs, err := suite.ar.GetBuildArgs()
	suite.Require().NoError(err)

	suite.Require().Equal("theLabel", buildArgs["NUCLIO_LABEL"])
	suite.Require().Equal("theArch", buildArgs["NUCLIO_ARCH"])
	suite.verifyBuildArgValue(buildArgs, "NUCLIO_BASE_IMAGE", baseImage)
	suite.verifyBuildArgValue(buildArgs, "NUCLIO_ONBUILD_IMAGE", onbuildImage)
}

func (suite *testSuite) verifyBuildArgValue(buildArgs map[string]string, keyName string, value string) {
	if value == "" {
		suite.Require().NotContains(buildArgs, keyName)
	} else {
		suite.Require().Equal(value, buildArgs[keyName])
	}
}

func TestRuntimeSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(testSuite))
}
