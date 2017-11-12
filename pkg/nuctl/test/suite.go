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
	"io"
	"os"
	"path"

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/nuctl/command"
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/nuclio/nuclio-sdk"
	"github.com/stretchr/testify/suite"
)

const (
	nuclioPlatformEnvVarName = "NUCLIO_PLATFORM"
)

type Suite struct {
	suite.Suite
	origPlatformType string
	logger           nuclio.Logger
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
	suite.dockerClient, err = dockerclient.NewShellClient(suite.logger)
	suite.Require().NoError(err)

	// make sure we use the "local" platform
	suite.origPlatformType = os.Getenv(nuclioPlatformEnvVarName)
	os.Setenv(nuclioPlatformEnvVarName, "local")

}

func (suite *Suite) TearDownSuite() {

	// restore platform type
	os.Setenv(nuclioPlatformEnvVarName, suite.origPlatformType)
}

func (suite *Suite) SetupTest() {
	suite.rootCommandeer = command.NewRootCommandeer()

	// set the output so we can capture it (but also output to stdout)
	suite.rootCommandeer.GetCmd().SetOutput(io.MultiWriter(os.Stdout, &suite.outputBuffer))
}

// ExecuteNutcl populates os.Args and executes nuctl as if it were executed from shell
func (suite *Suite) ExecuteNutcl(positionalArgs []string,
	namedArgs map[string]string) error {

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

	// execute
	return suite.rootCommandeer.Execute()
}

// GetNuclioSourceDir returns path to nuclio source directory
func (suite *Suite) GetNuclioSourceDir() string {
	return path.Join(os.Getenv("GOPATH"), "src", "github.com", "nuclio", "nuclio")
}
