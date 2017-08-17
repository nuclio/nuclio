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
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/util/cmdrunner"
	nucliozap "github.com/nuclio/nuclio/pkg/zap"
)

type CmdLineSuite struct {
	suite.Suite

	logger     nuclio.Logger
	imageName  string
	cmd        *cmdrunner.CmdRunner
	runOptions *cmdrunner.RunOptions
}

func (suite *CmdLineSuite) failOnError(err error, fmt string, args ...interface{}) {
	if err != nil {
		suite.FailNowf(err.Error(), fmt, args...)
	}
}

func (suite *CmdLineSuite) gitRoot() string {
	out, err := suite.cmd.Run(nil, "git rev-parse --show-toplevel")
	suite.failOnError(err, "Can't find git root")
	return strings.TrimSpace(out)
}

func (suite *CmdLineSuite) SetupSuite() {
	var loggerLevel nucliozap.Level
	if testing.Verbose() {
		loggerLevel = nucliozap.DebugLevel
	} else {
		loggerLevel = nucliozap.InfoLevel
	}

	zap, err := nucliozap.NewNuclioZap("test-build", loggerLevel)
	suite.failOnError(err, "Can't create logger")
	suite.logger = zap
	suite.cmd, err = cmdrunner.NewCmdRunner(suite.logger)
	suite.failOnError(err, "Can't create runner")
	root := suite.gitRoot()
	suite.runOptions = &cmdrunner.RunOptions{WorkingDir: &root}

	suite.imageName = "bld-test"
	suite.logger.InfoWith("Image name", "name", suite.imageName)
}

func (suite *CmdLineSuite) TearDownSuite() {
	suite.cmd.Run(nil, "docker rmi %s", suite.imageName)
}

func (suite *CmdLineSuite) TestBuild() {
	_, err := suite.cmd.Run(suite.runOptions, "go build ./cmd/nubuild")
	suite.failOnError(err, "Can't build nubuild")
	_, err = suite.cmd.Run(suite.runOptions, "./nubuild --verbose -n %s ../nuclio-sdk/examples/hello-world", suite.imageName)
	suite.failOnError(err, "Can't run nubuild")
}

func TestCmdLine(t *testing.T) {
	suite.Run(t, new(CmdLineSuite))
}
