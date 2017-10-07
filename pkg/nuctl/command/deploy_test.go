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

package command

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/nuclio/nuclio-sdk"
	"github.com/stretchr/testify/suite"
)

type RunTestSuite struct {
	suite.Suite
	logger        nuclio.Logger
	commonOptions platform.CommonOptions
	runOptions    platform.DeployOptions
}

func (suite *RunTestSuite) SetupTest() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")

	suite.commonOptions = platform.CommonOptions{}
	suite.runOptions = platform.DeployOptions{}

	// by default image version is latest, as set by cobra
	suite.runOptions.Build.ImageVersion = "latest"
}

func (suite *RunTestSuite) TestRunFromSpecFile() {
	suite.T().Skip("Specfile not supported yet")

	specFilePath := suite.createSpecFile(`
apiVersion: nuclio.io/v1
kind: Function
metadata:
  name: function-name
spec:
  image: 5.5.5.5:2000/image-name:image-version`)

	defer os.Remove(specFilePath)

	// only set spec file path and code path
	suite.runOptions.SpecPath = specFilePath
	suite.runOptions.Build.Path = "f.go"

	err := prepareDeployerOptions([]string{}, true, &suite.commonOptions, &suite.runOptions)
	suite.Require().NoError(err)

	suite.Require().Equal("function-name", suite.runOptions.Common.Identifier)
	suite.Require().Equal("5.5.5.5:2000", suite.runOptions.Build.Registry)
	suite.Require().Equal("image-name", suite.runOptions.Build.ImageName)
	suite.Require().Equal("image-version", suite.runOptions.Build.ImageVersion)
	suite.Require().Equal("5.5.5.5:2000", suite.runOptions.RunRegistry)
}

func (suite *RunTestSuite) TestRunFromSpecFileWithRunRegistry() {
	suite.T().Skip("Specfile not supported yet")

	specFilePath := suite.createSpecFile(`
apiVersion: nuclio.io/v1
kind: Function
metadata:
  name: function-name
spec:
  image: 5.5.5.5:2000/image-name:image-version`)

	defer os.Remove(specFilePath)

	suite.runOptions.SpecPath = specFilePath
	suite.runOptions.Build.Path = "f.go"
	suite.runOptions.RunRegistry = "1.1.1.1:9000"

	err := prepareDeployerOptions([]string{}, true, &suite.commonOptions, &suite.runOptions)
	suite.Require().NoError(err)

	suite.Require().Equal("function-name", suite.runOptions.Common.Identifier)
	suite.Require().Equal("5.5.5.5:2000", suite.runOptions.Build.Registry)
	suite.Require().Equal("image-name", suite.runOptions.Build.ImageName)
	suite.Require().Equal("image-version", suite.runOptions.Build.ImageVersion)
	suite.Require().Equal("1.1.1.1:9000", suite.runOptions.RunRegistry)
}

func (suite *RunTestSuite) TestRunPushRegistryAndFuncName() {
	suite.runOptions.Build.Path = "f.go"
	suite.runOptions.Build.Registry = "5.5.5.5:2000"

	err := prepareDeployerOptions([]string{"function-name"}, true, &suite.commonOptions, &suite.runOptions)
	suite.Require().NoError(err)

	suite.Require().Equal("function-name", suite.runOptions.Common.Identifier)
	suite.Require().Equal("5.5.5.5:2000", suite.runOptions.Build.Registry)
	suite.Require().Equal("function-name", suite.runOptions.Build.ImageName)
	suite.Require().Equal("latest", suite.runOptions.Build.ImageVersion)
	suite.Require().Equal("5.5.5.5:2000", suite.runOptions.RunRegistry)
}

func (suite *RunTestSuite) TestRunPushRegistryRunRegistryAndFuncName() {
	suite.runOptions.Build.Path = "f.go"
	suite.runOptions.Build.Registry = "5.5.5.5:2000"
	suite.runOptions.RunRegistry = "1.1.1.1:9000"

	err := prepareDeployerOptions([]string{"function-name"}, true, &suite.commonOptions, &suite.runOptions)
	suite.Require().NoError(err)

	suite.Require().Equal("function-name", suite.runOptions.Common.Identifier)
	suite.Require().Equal("5.5.5.5:2000", suite.runOptions.Build.Registry)
	suite.Require().Equal("function-name", suite.runOptions.Build.ImageName)
	suite.Require().Equal("latest", suite.runOptions.Build.ImageVersion)
	suite.Require().Equal("1.1.1.1:9000", suite.runOptions.RunRegistry)
}

func (suite *RunTestSuite) TestRunPushRegistryRunRegistryImageVersionAndFuncName() {
	suite.runOptions.Build.Path = "f.go"
	suite.runOptions.Build.Registry = "5.5.5.5:2000"
	suite.runOptions.Build.ImageName = "image-name"
	suite.runOptions.Build.ImageVersion = "image-version"
	suite.runOptions.RunRegistry = "1.1.1.1:9000"

	err := prepareDeployerOptions([]string{"function-name"}, true, &suite.commonOptions, &suite.runOptions)
	suite.Require().NoError(err)

	suite.Require().Equal("function-name", suite.runOptions.Common.Identifier)
	suite.Require().Equal("5.5.5.5:2000", suite.runOptions.Build.Registry)
	suite.Require().Equal("image-name", suite.runOptions.Build.ImageName)
	suite.Require().Equal("image-version", suite.runOptions.Build.ImageVersion)
	suite.Require().Equal("1.1.1.1:9000", suite.runOptions.RunRegistry)
}

func (suite *RunTestSuite) TestErrInvalidSpecFile() {
	specFilePath := suite.createSpecFile(`
	invalid
	---contents`)

	defer os.Remove(specFilePath)

	suite.runOptions.SpecPath = specFilePath
	suite.runOptions.Build.Path = "f.go"

	err := prepareDeployerOptions([]string{}, true, &suite.commonOptions, &suite.runOptions)
	suite.Require().Error(err)
}

func (suite *RunTestSuite) TestErrNoSpecFileNoArguments() {
	suite.runOptions.Build.Path = "f.go"

	err := prepareDeployerOptions([]string{}, true, &suite.commonOptions, &suite.runOptions)
	suite.Require().Equal("Function run requires name", err.Error())
}

func (suite *RunTestSuite) TestErrNoSpecImageNoBuildRegistry() {
	suite.T().Skip("Specfile not supported yet")

	specFilePath := suite.createSpecFile(`
apiVersion: nuclio.io/v1
kind: Function
metadata:
  name: function-name
`)

	defer os.Remove(specFilePath)

	suite.runOptions.SpecPath = specFilePath
	suite.runOptions.Build.Path = "f.go"

	err := prepareDeployerOptions([]string{}, true, &suite.commonOptions, &suite.runOptions)
	suite.Require().Equal("Registry is required (can also be specified in spec.image or a NUCTL_REGISTRY env var", err.Error())
}

func (suite *RunTestSuite) TestErrNoPathAndNoInline() {
	suite.T().Skip("Specfile not supported yet")

	specFilePath := suite.createSpecFile(`
apiVersion: nuclio.io/v1
kind: Function
metadata:
  name: function-name
spec:
  image: 5.5.5.5:2000/image-name:image-version`)

	defer os.Remove(specFilePath)

	suite.runOptions.SpecPath = specFilePath

	err := prepareDeployerOptions([]string{}, true, &suite.commonOptions, &suite.runOptions)
	suite.Require().Equal("Function code must be provided either in path or inline in a spec file", err.Error())
}

func (suite *RunTestSuite) TestErrInvalidSpecImage() {
	suite.T().Skip("Specfile not supported yet")

	specFilePath := suite.createSpecFile(`
apiVersion: nuclio.io/v1
kind: Function
metadata:
  name: function-name
spec:
  image: notavalidimage`)

	defer os.Remove(specFilePath)

	suite.runOptions.SpecPath = specFilePath
	suite.runOptions.Build.Path = "f.go"

	err := prepareDeployerOptions([]string{}, true, &suite.commonOptions, &suite.runOptions)
	suite.Require().Equal("Failed to parse image URL: Failed looking for image splitter: /", err.Error())
}

func (suite *RunTestSuite) createSpecFile(contents string) string {
	file, err := ioutil.TempFile("", "specfile")
	suite.Require().NoError(err)

	defer file.Close()

	bytesWritten, err := file.WriteString(contents)
	suite.Require().NoError(err)
	suite.Require().Equal(len(contents), bytesWritten)

	return file.Name()
}

func TestRunTestSuite(t *testing.T) {
	suite.Run(t, new(RunTestSuite))
}