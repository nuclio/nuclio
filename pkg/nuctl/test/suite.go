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
	"bufio"
	"bytes"
	"io"
	"os"
	"path"
	"strings"

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/nuctl/command"
	"github.com/nuclio/nuclio/pkg/version"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

const (
	nuctlPlatformEnvVarName = "NUCTL_PLATFORM"
)

type Suite struct {
	suite.Suite
	origPlatformType string
	logger           logger.Logger
	rootCommandeer   *command.RootCommandeer
	dockerClient     dockerclient.Client
	outputBuffer     bytes.Buffer
}

func (suite *Suite) SetupSuite() {
	var err error

	// create logger
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	// create docker client
	suite.dockerClient, err = dockerclient.NewShellClient(suite.logger, nil)
	suite.Require().NoError(err)

	// save platform type before the test
	suite.origPlatformType = os.Getenv(nuctlPlatformEnvVarName)

	// default to local platform if platform isn't set
	if os.Getenv(nuctlPlatformEnvVarName) == "" {
		err = os.Setenv(nuctlPlatformEnvVarName, "local")
		suite.Require().NoError(err)
	}

	// update version so that linker doesn't need to inject it
	err = version.Set(&version.Info{
		GitCommit: "c",
		Label:     "latest",
		Arch:      "amd64",
		OS:        "linux",
	})
	suite.Require().NoError(err)
}

func (suite *Suite) TearDownSuite() {

	// restore platform type
	err := os.Setenv(nuctlPlatformEnvVarName, suite.origPlatformType)
	suite.Require().NoError(err)
}

// ExecuteNutcl populates os.Args and executes nuctl as if it were executed from shell
func (suite *Suite) ExecuteNutcl(positionalArgs []string,
	namedArgs map[string]string) error {

	suite.rootCommandeer = command.NewRootCommandeer()

	// set the output so we can capture it (but also output to stdout)
	suite.rootCommandeer.GetCmd().SetOutput(io.MultiWriter(os.Stdout, &suite.outputBuffer))

	// since args[0] is the executable name, just shove something there
	argsStringSlice := []string{
		"nuctl",
	}

	// add positional arguments
	argsStringSlice = append(argsStringSlice, positionalArgs...)

	for argName, argValue := range namedArgs {
		argsStringSlice = append(argsStringSlice, "--"+argName)

		if argValue != "" {
			argsStringSlice = append(argsStringSlice, argValue)
		}
	}

	// override os.Args (this can't go wrong horribly, can it?)
	os.Args = argsStringSlice

	suite.logger.DebugWith("Executing nuctl", "args", argsStringSlice)

	// execute
	return suite.rootCommandeer.Execute()
}

// GetNuclioSourceDir returns path to nuclio source directory
func (suite *Suite) GetNuclioSourceDir() string {
	return path.Join(os.Getenv("GOPATH"), "src", "github.com", "nuclio", "nuclio")
}

// GetNuclioSourceDir returns path to nuclio source directory
func (suite *Suite) GetFunctionsDir() string {
	return path.Join(suite.GetNuclioSourceDir(), "test", "_functions")
}

func (suite *Suite) findPatternsInOutput(patternsMustExist []string, patternsMustNotExist []string) {
	foundPatternsMustExist := make([]bool, len(patternsMustExist))
	foundPatternsMustNotExist := make([]bool, len(patternsMustNotExist))

	// iterate over all lines in result
	scanner := bufio.NewScanner(&suite.outputBuffer)
	for scanner.Scan() {

		for patternIdx, patternName := range patternsMustExist {
			if strings.Contains(scanner.Text(), patternName) {
				foundPatternsMustExist[patternIdx] = true
				break
			}
		}

		for patternIdx, patternName := range patternsMustNotExist {
			if strings.Contains(scanner.Text(), patternName) {
				foundPatternsMustNotExist[patternIdx] = true
				break
			}
		}
	}

	// all patterns that must exist must exist
	for _, foundPattern := range foundPatternsMustExist {
		suite.Require().True(foundPattern)
	}

	// all patterns that must not exist must not exist
	for _, foundPattern := range foundPatternsMustNotExist {
		suite.Require().False(foundPattern)
	}
}
