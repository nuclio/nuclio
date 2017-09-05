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
package main

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/util/cmdrunner"
	"github.com/nuclio/nuclio/test/suite"
)

type CliSuite struct {
	suite.NuclioTestSuite

	imageName string
}

func (suite *CliSuite) buildCli() {
	options := &cmdrunner.RunOptions{WorkingDir: &suite.NuclioRootPath}
	_, err := suite.Cmd.Run(options, "go build ./cmd/nubuild")
	suite.Require().NoError(err, "Can't build nubuild")
}

func (suite *CliSuite) generateImageName() string {
	buf := make([]byte, 4)
	rand.Seed(time.Now().Unix())
	_, err := rand.Read(buf)
	suite.Require().NoError(err, "Create read random bytes")
	return fmt.Sprintf("nubuild-test-%x", buf)
}

func (suite *CliSuite) SetupSuite() {
	suite.NuclioTestSuite.SetupSuite()
	suite.imageName = suite.generateImageName()
	suite.Logger.DebugWith("Docker image name", "name", suite.imageName)
	suite.buildCli()
}

func (suite *CliSuite) TearDownSuite() {
	suite.Cmd.Run(nil, "docker rmi %s", suite.imageName)
}

func (suite *CliSuite) TestDependencies() {
	cmd := "./nubuild --verbose --nuclio-src-dir %s -n %s ./vendor/github.com/nuclio/nuclio-sdk/examples/os-packages"
	options := &cmdrunner.RunOptions{WorkingDir: &suite.NuclioRootPath}
	_, err := suite.Cmd.Run(options, cmd, suite.NuclioRootPath, suite.imageName)
	suite.Require().NoError(err, "Can't build docker container")
}

func TestCli(t *testing.T) {
	suite.Run(t, new(CliSuite))
}
