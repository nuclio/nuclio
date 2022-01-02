//go:build test_unit

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

package cmdrunner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type CmdRunnerTestSuite struct {
	suite.Suite
	logger        logger.Logger
	commandRunner CmdRunner
}

func (suite *CmdRunnerTestSuite) SetupTest() {
	var err error

	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	suite.commandRunner, err = NewShellRunner(suite.logger)
	if err != nil {
		panic("Failed to create command runner")
	}
}

func (suite *CmdRunnerTestSuite) TestWorkingDir() {
	currentDirectory, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		suite.Fail("Failed to get current directory")
	}

	options := RunOptions{
		WorkingDir: &currentDirectory,
	}

	runResult, err := suite.commandRunner.Run(&options, "pwd")
	suite.Require().NoError(err)

	// remove "private" on OSX
	privatePrefix := "/private"
	runResult.Output = strings.TrimPrefix(runResult.Output, privatePrefix)
	suite.Require().True(strings.HasPrefix(runResult.Output, currentDirectory))
}

func (suite *CmdRunnerTestSuite) TestFormattedCommand() {
	runResult, err := suite.commandRunner.Run(nil, `echo "%s %d"`, "hello", 1)
	suite.Require().NoError(err)

	// ignore newlines, if any
	suite.Require().True(strings.HasPrefix(runResult.Output, "hello 1"))
}

func (suite *CmdRunnerTestSuite) TestEnv() {
	options := RunOptions{
		Env: map[string]string{
			"ENV1": "env1",
			"ENV2": "env2",
		},
	}

	runResult, err := suite.commandRunner.Run(&options, `echo $ENV1 && echo $ENV2`)
	suite.Require().NoError(err)

	// ignore newlines, if any
	suite.Require().True(strings.HasPrefix(runResult.Output, "env1\nenv2"))
}

func (suite *CmdRunnerTestSuite) TestStdin() {
	stdinValue := "from stdin"

	options := RunOptions{
		Stdin: &stdinValue,
	}

	runResult, err := suite.commandRunner.Run(&options, "more")
	suite.Require().NoError(err)

	// ignore newlines, if any
	suite.Require().True(strings.HasPrefix(runResult.Output, stdinValue))
}

func TestCmdRunnerTestSuite(t *testing.T) {
	suite.Run(t, new(CmdRunnerTestSuite))
}
