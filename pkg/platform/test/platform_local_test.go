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
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/factory"
	"github.com/nuclio/nuclio/pkg/platform/local"
	"github.com/nuclio/nuclio/pkg/version"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	suite.Suite
	Logger           logger.Logger
	DockerClient     dockerclient.Client
	Platform         platform.Platform
	TestID           string
	Runtime          string
	RuntimeDir       string
	FunctionDir      string
	containerID      string
	TempDir          string
	CleanupTemp      bool
	DefaultNamespace string
}

func (suite *TestSuite) SetupSuite() {
	err := version.Set(&version.Info{
		GitCommit: "c",
		Label:     "latest",
		Arch:      "amd64",
		OS:        "linux",
	})
	suite.Require().NoError(err, "Failed to set version info")

	suite.DefaultNamespace = "nuclio"

	suite.Logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err, "Logger should create successfully")

	suite.DockerClient, err = dockerclient.NewShellClient(suite.Logger, nil)
	suite.Require().NoError(err, "Docker client should create successfully")

	platformType := common.GetEnvOrDefaultString("NUCLIO_PLATFORM", "local")
	suite.Platform, err = factory.CreatePlatform(suite.Logger, platformType, nil, suite.DefaultNamespace)
	suite.Require().NoError(err, "Platform should create successfully")
}

func (suite *TestSuite) SetupTest() {
	suite.TestID = xid.New().String()
}

// TearDownTest is called after each test in the suite
func (suite *TestSuite) TearDownTest() {
	defer os.RemoveAll(suite.TempDir)
	if suite.containerID != "" {
		if common.GetEnvOrDefaultString("NUCLIO_TEST_KEEP_DOCKER", "") == "" {

			// remove container
			defer suite.DockerClient.RemoveContainer(suite.containerID)
		}
	}
}

// Test function containers healthiness validation
func (suite *TestSuite) TestValidateFunctionContainersHealthiness() {

	// Create the function
	createFunctionOptions := suite.GetMockDeploymentFunction("echoer")
	createdFunction, err := suite.Platform.CreateFunction(createFunctionOptions)
	suite.NoError(err, "Could not create function")
	suite.containerID = createdFunction.ContainerID

	// Get the functions from local store
	functions, err := suite.Platform.GetFunctions(&platform.GetFunctionsOptions{
		Namespace: createdFunction.CreateFunctionBuildResult.UpdatedFunctionConfig.Meta.Namespace,
		Name:      createdFunction.CreateFunctionBuildResult.UpdatedFunctionConfig.Meta.Name,
	})
	suite.NoError(err, "Could not get functions")
	suite.Len(functions, 1, "Expected to find the newly created function")
	function := functions[0]

	// Check its state is ready
	suite.Equal(function.GetStatus().State, functionconfig.FunctionStateReady)

	// Remove the container
	err = suite.DockerClient.RemoveContainer(createdFunction.ContainerID)
	suite.Require().NoError(err, "Could not remove container")

	// Trigger function containers healthiness validation
	suite.Platform.(*local.Platform).ValidateFunctionContainersHealthiness()

	// Get functions again from local store
	functions, err = suite.Platform.GetFunctions(&platform.GetFunctionsOptions{
		Namespace: createdFunction.CreateFunctionBuildResult.UpdatedFunctionConfig.Meta.Namespace,
		Name:      createdFunction.CreateFunctionBuildResult.UpdatedFunctionConfig.Meta.Name,
	})
	suite.NoError(err, "Could not get functions")
	suite.Len(functions, 1, "Expected to find the newly created function")
	function = functions[0]

	// Now the function state should be error
	suite.Require().Equal(function.GetStatus().State, functionconfig.FunctionStateError)
}

// GetDeployOptions populates a platform.CreateFunctionOptions structure from function name and path
func (suite *TestSuite) GetMockDeploymentFunction(functionName string) *platform.CreateFunctionOptions {
	functionConfig := *functionconfig.NewConfig()
	functionConfig.Spec.ReadinessTimeoutSeconds = 30
	functionConfig.Meta.Namespace = suite.DefaultNamespace

	createFunctionOptions := &platform.CreateFunctionOptions{
		Logger:         suite.Logger,
		FunctionConfig: functionConfig,
	}

	createFunctionOptions.FunctionConfig.Meta.Name = functionName
	createFunctionOptions.FunctionConfig.Spec.Runtime = "shell"
	createFunctionOptions.FunctionConfig.Spec.Build.Path = "/dev/null"

	// Save tempdir so we can delete that dir later
	suite.TempDir = suite.CreateTempDir()
	createFunctionOptions.FunctionConfig.Spec.Build.TempDir = suite.TempDir

	createFunctionOptions.FunctionConfig.Spec.Runtime = "shell"
	createFunctionOptions.FunctionConfig.Spec.Handler = "echo"

	// don't explicitly pull base images before building
	createFunctionOptions.FunctionConfig.Spec.Build.NoBaseImagesPull = true

	return createFunctionOptions
}

func (suite *TestSuite) CreateTempDir() string {
	tempDir, err := ioutil.TempDir("", fmt.Sprintf("build-test-%s", suite.TestID))
	suite.Require().NoErrorf(err, "Failed to create temporary dir %s for test %s", suite.TempDir, suite.TestID)
	return tempDir
}

func TestProjectTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestSuite))
}
