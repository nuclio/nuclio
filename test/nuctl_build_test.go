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
	"path"
	"strings"
	"testing"

	"github.com/nuclio/nuclio/pkg/util/cmdrunner"
	"github.com/nuclio/nuclio/test/suite"

	"github.com/rs/xid"
)

type CliSuite struct {
	suite.NuclioTestSuite

	imageName   string
	containerID string
}

func (suite *CliSuite) buildCli() {
	options := &cmdrunner.RunOptions{WorkingDir: &suite.NuclioRootPath}
	_, err := suite.Cmd.Run(options, "go build ./cmd/nuctl")
	suite.Require().NoError(err, "Can't build nuctl")
}

func (suite *CliSuite) generateImageName() string {
	return fmt.Sprintf("nuctl-test-%s", xid.New())
}

func (suite *CliSuite) SetupSuite() {
	suite.NuclioTestSuite.SetupSuite()
	suite.imageName = suite.generateImageName()
	suite.Logger.DebugWith("Docker image name", "name", suite.imageName)
	suite.buildCli()
}

func (suite *CliSuite) TearDownSuite() {
	// Don't care about errors here
	if suite.containerID != "" {
		suite.Cmd.Run(nil, "docker rm -f %s", suite.containerID)
	}

	suite.Cmd.Run(nil, "docker rmi %s", suite.imageName)
}

func (suite *CliSuite) TestDependencies() {
	pkgDirPath := path.Join(suite.NuclioRootPath, "test/_os-packages")
	cmd := "./nuctl build --verbose --nuclio-src-dir %s --path %s %s"
	options := &cmdrunner.RunOptions{WorkingDir: &suite.NuclioRootPath}
	_, err := suite.Cmd.Run(options, cmd, suite.NuclioRootPath, pkgDirPath, suite.imageName)
	suite.Require().NoError(err, "Can't build docker container")
	out, err := suite.Cmd.Run(nil, "docker run -d %s", suite.imageName)
	suite.Require().NoError(err, "Can't run docker image")
	suite.containerID = strings.TrimSpace(out)

	filesOnContainer := []string{
		// From commands
		"/cmd1.out",
		"/cmd2.out",

		// Script
		"/opt/nuclio/install.sh",
		// From script invocation
		"/hello.txt",

		// From copy
		"/opt/nuclio/key.txt",
	}

	for _, path := range filesOnContainer {
		suite.Require().True(suite.FileInDocker(path), "Can't find %s on container", path)
	}
}

func (suite *CliSuite) FileInDocker(path string) bool {
	_, err := suite.Cmd.Run(nil, "docker exec %s ls %s", suite.containerID, path)
	return err == nil
}

func TestCli(t *testing.T) {
	suite.Run(t, new(CliSuite))
}
