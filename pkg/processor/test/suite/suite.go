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
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/local"
	"github.com/nuclio/nuclio/pkg/processor/build"
	"github.com/nuclio/nuclio/pkg/version"
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/nuclio/nuclio-sdk"
	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
)

type RunOptions struct {
	dockerclient.RunOptions
}

// TestSuite is a base test suite that offers its children the ability to build
// and run a function, after which the child test can communicate with the
// function container (through an trigger of some sort)
type TestSuite struct {
	suite.Suite
	Logger       nuclio.Logger
	DockerClient dockerclient.Client
	Builder      *build.Builder
	Platform     platform.Platform
	TestID       string
	Runtime      string
	FunctionDir  string
	containerID  string
}

// SetupSuite is called for suite setup
func (suite *TestSuite) SetupSuite() {
	var err error

	// update version so that linker doesn't need to inject it
	version.Set(&version.Info{
		GitCommit: "c",
		Label:     "latest",
		Arch:      "amd64",
		OS:        "linux",
	})

	suite.Logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	suite.DockerClient, err = dockerclient.NewShellClient(suite.Logger)
	suite.Require().NoError(err)

	suite.Platform, err = local.NewPlatform(suite.Logger)
	suite.Require().NoError(err)
}

// SetupTest is called before each test in the suite
func (suite *TestSuite) SetupTest() {
	suite.TestID = xid.New().String()
}

// TearDownTest is called after each test in the suite
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

// DeployFunction builds a docker image, runs a container from it and then
// runs onAfterContainerRun
func (suite *TestSuite) DeployFunction(deployOptions *platform.DeployOptions,
	onAfterContainerRun func(deployResult *platform.DeployResult) bool) *platform.DeployResult {

	deployOptions.FunctionConfig.Meta.Name = fmt.Sprintf("%s-%s", deployOptions.FunctionConfig.Meta.Name, suite.TestID)
	deployOptions.FunctionConfig.Spec.Build.NuclioSourceDir = suite.GetNuclioSourceDir()
	deployOptions.FunctionConfig.Spec.Build.NoBaseImagesPull = true

	// deploy the function
	deployResult, err := suite.Platform.DeployFunction(deployOptions)
	suite.Require().NoError(err)

	// remove the image when we're done
	defer suite.DockerClient.RemoveImage(deployResult.ImageName)

	// give the container some time - after 10 seconds, give up
	deadline := time.Now().Add(10 * time.Second)

	for {

		// stop after 10 seconds
		if time.Now().After(deadline) {
			var dockerLogs string

			dockerLogs, err = suite.DockerClient.GetContainerLogs(suite.containerID)
			if err == nil {
				suite.Logger.DebugWith("Processor didn't come up in time", "logs", dockerLogs)
			}

			suite.FailNow("Processor didn't come up in time")
		}

		// three options for onAfterContainerRun:
		// 1. it calls suite.fail - the suite will stop and fail
		// 2. it returns false - indicating that the container wasn't ready yet
		// 3. it returns true - meaning everything was ok
		if onAfterContainerRun(deployResult) {
			break
		}
	}

	// delete the function
	err = suite.Platform.DeleteFunction(&platform.DeleteOptions{
		FunctionConfig: deployOptions.FunctionConfig,
	})

	suite.Require().NoError(err)

	return deployResult
}

// GetNuclioSourceDir returns path to nuclio source directory
func (suite *TestSuite) GetNuclioSourceDir() string {
	return path.Join(os.Getenv("GOPATH"), "src", "github.com", "nuclio", "nuclio")
}

// GetDeployOptions populates a platform.DeployOptions structure from function name and path
func (suite *TestSuite) GetDeployOptions(functionName string, functionPath string) *platform.DeployOptions {

	deployOptions := &platform.DeployOptions{
		Logger:         suite.Logger,
		FunctionConfig: *functionconfig.NewConfig(),
	}

	deployOptions.FunctionConfig.Meta.Name = functionName
	deployOptions.FunctionConfig.Spec.Runtime = suite.Runtime
	deployOptions.FunctionConfig.Spec.Build.Path = functionPath

	return deployOptions
}

// GetFunctionPath returns the non-relative function path (given a relative path)
func (suite *TestSuite) GetFunctionPath(functionRelativePath ...string) string {

	// functionPath = FunctionDir + functionRelativePath
	functionPath := []string{suite.FunctionDir}
	functionPath = append(functionPath, functionRelativePath...)

	return path.Join(functionPath...)
}
