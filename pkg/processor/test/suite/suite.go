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

package processorsuite

import (
	"fmt"
	"os"
	"path"
	"time"

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/processor/build"
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/nuclio/nuclio-sdk"
	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
)

//
// This base test suite offers its children the ability to build and run a function, after which
// the child test can communicate with the function container (through an event source of some sort)
//

type TestSuite struct {
	suite.Suite
	Logger       nuclio.Logger
	Builder      *build.Builder
	DockerClient *dockerclient.Client
	TestID       string
	containerID  string
}

func (suite *TestSuite) SetupSuite() {
	var err error

	suite.Logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	suite.DockerClient, err = dockerclient.NewClient(suite.Logger)
	suite.Require().NoError(err)
}

func (suite *TestSuite) SetupTest() {
	suite.TestID = xid.New().String()
}

func (suite *TestSuite) TearDownTest() {

	// if we managed to get a container up, dump logs if we failed and remove the container either way
	if suite.containerID != "" {

		if suite.T().Failed() {

			// wait a bit for things to flush
			time.Sleep(2 * time.Second)

			if logs, err := suite.DockerClient.GetContainerLogs(suite.containerID); err == nil {
				suite.Logger.WarnWith("Test failed, retreived logs", "logs", logs)
			} else {
				suite.Logger.WarnWith("Failed to get docker logs on failure", "err", err)
			}
		}

		suite.DockerClient.RemoveContainer(suite.containerID)
	}
}

func (suite *TestSuite) BuildAndRunFunction(functionName string,
	functionPath string,
	runtime string,
	ports map[int]int,
	onAfterContainerRun func() bool) {

	var err error

	functionName = fmt.Sprintf("%s-%s", functionName, suite.TestID)
	imageName := fmt.Sprintf("nuclio/processor-%s", functionName)

	suite.Builder, err = build.NewBuilder(suite.Logger, &build.Options{
		FunctionName:    functionName,
		FunctionPath:    functionPath,
		Runtime:         runtime,
		NuclioSourceDir: suite.GetNuclioSourceDir(),
		Verbose:         true,
		NoBaseImagePull: true,
	})

	suite.Require().NoError(err)

	// do the build
	imageName, err = suite.Builder.Build()
	suite.Require().NoError(err)

	// remove the image when we're done
	defer suite.DockerClient.RemoveImage(imageName)

	// run the processor
	suite.containerID, err = suite.DockerClient.RunContainer(imageName, ports, "")

	suite.Require().NoError(err)

	// give the container some time - after 10 seconds, give up
	deadline := time.Now().Add(10 * time.Second)

	for {

		// stop after 10 seconds
		if time.Now().After(deadline) {
			dockerLogs, err := suite.DockerClient.GetContainerLogs(suite.containerID)
			if err == nil {
				suite.Logger.DebugWith("Processor didn't come up in time", "logs", dockerLogs)
			}

			suite.FailNow("Processor didn't come up in time")
		}

		// three options for onAfterContainerRun:
		// 1. it calls suite.fail - the suite will stop and fail
		// 2. it returns false - indicating that the container wasn't ready yet
		// 3. it returns true - meaning everything was ok
		if onAfterContainerRun() {
			break
		}
	}
}

func (suite *TestSuite) GetNuclioSourceDir() string {
	return path.Join(os.Getenv("GOPATH"), "src", "github.com", "nuclio", "nuclio")
}
