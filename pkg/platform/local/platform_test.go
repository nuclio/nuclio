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
package local

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
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
	var err error
	version.SetFromEnv()

	suite.DefaultNamespace = "nuclio"

	suite.Logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err, "Logger should create successfully")

	suite.DockerClient, err = dockerclient.NewShellClient(suite.Logger, nil)
	suite.Require().NoError(err, "Docker client should create successfully")

	suite.Platform, err = NewPlatform(suite.Logger,
		&containerimagebuilderpusher.ContainerBuilderConfiguration{
			DefaultOnbuildRegistryURL: common.GetEnvOrDefaultString("NUCLIO_DASHBOARD_DEFAULT_ONBUILD_REGISTRY_URL",
				"quay.io"),
		},
		nil)
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
	createFunctionOptions := suite.GetMockDeploymentFunction(fmt.Sprintf("echoer-%s", suite.TestID))
	createdFunction, err := suite.Platform.CreateFunction(createFunctionOptions)
	suite.Require().NoError(err, "Could not create function")
	suite.containerID = createdFunction.ContainerID
	functionName := createdFunction.CreateFunctionBuildResult.UpdatedFunctionConfig.Meta.Name
	namespace := createdFunction.CreateFunctionBuildResult.UpdatedFunctionConfig.Meta.Namespace

	function := suite.getFunction(functionName, namespace)

	// Check its state is ready
	suite.Equal(function.GetStatus().State, functionconfig.FunctionStateReady)

	// Stop the container
	err = suite.DockerClient.StopContainer(createdFunction.ContainerID)
	suite.Require().NoError(err, "Could not stop container")

	// Trigger function containers healthiness validation
	suite.Platform.(*Platform).ValidateFunctionContainersHealthiness()

	// Get functions again
	function = suite.getFunction(functionName, namespace)

	// Now the function state should be error
	suite.Require().Equal(function.GetStatus().State, functionconfig.FunctionStateError)

	// Start the container
	err = suite.DockerClient.StartContainer(createdFunction.ContainerID)
	suite.Require().NoError(err, "Could not start container")

	// Trigger function containers healthiness validation
	suite.Platform.(*Platform).ValidateFunctionContainersHealthiness()

	// Get functions again from local store
	function = suite.getFunction(functionName, namespace)

	// Function is healthy again
	suite.Equal(function.GetStatus().State, functionconfig.FunctionStateReady)
}

// Test function import without deploy and build, then deploy calls build and deploy
func (suite *TestSuite) TestImportFunctionFlow() {

	// Create the function
	createFunctionOptions := suite.GetMockDeploymentFunction(fmt.Sprintf("echoer-%s", suite.TestID))
	createFunctionOptions.FunctionConfig.Meta.Annotations = map[string]string{
		functionconfig.FunctionAnnotationSkipBuild:  "true",
		functionconfig.FunctionAnnotationSkipDeploy: "true",
	}
	createdFunction, err := suite.Platform.CreateFunction(createFunctionOptions)
	suite.NoError(err, "Failed to create function")

	function := suite.getFunction(createdFunction.CreateFunctionBuildResult.UpdatedFunctionConfig.Meta.Name,
		createdFunction.CreateFunctionBuildResult.UpdatedFunctionConfig.Meta.Namespace)

	// Check its state is imported and not deployed
	suite.Equal(function.GetStatus().State, functionconfig.FunctionStateImported)

	// Check that the annotations have been removed
	_, skipBuildExists := function.GetConfig().Meta.Annotations[functionconfig.FunctionAnnotationSkipBuild]
	_, skipDeployExists := function.GetConfig().Meta.Annotations[functionconfig.FunctionAnnotationSkipDeploy]
	suite.Assert().False(skipBuildExists)
	suite.Assert().False(skipDeployExists)

	recreateFunctionOptions := &platform.CreateFunctionOptions{
		Logger: suite.Logger,
		FunctionConfig: functionconfig.Config{
			Meta: function.GetConfig().Meta,
			Spec: function.GetConfig().Spec,
		},
	}
	recreatedFunction, err := suite.Platform.CreateFunction(recreateFunctionOptions)
	suite.NoError(err, "Failed to create function")

	// Get the recreated functions
	function = suite.getFunction(recreatedFunction.CreateFunctionBuildResult.UpdatedFunctionConfig.Meta.Name,
		recreatedFunction.CreateFunctionBuildResult.UpdatedFunctionConfig.Meta.Namespace)

	// Check its state is ready
	suite.Equal(function.GetStatus().State, functionconfig.FunctionStateReady)
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
	createFunctionOptions.FunctionConfig.Meta.Labels = map[string]string{
		"nuclio.io/project-name": platform.DefaultProjectName,
	}
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

func (suite *TestSuite) getFunction(functionName string, namespace string) platform.Function {
	functions, err := suite.Platform.GetFunctions(&platform.GetFunctionsOptions{
		Namespace: namespace,
		Name:      functionName,
	})
	suite.Require().NoError(err, "Failed to get functions")
	suite.Len(functions, 1, "Expected to find one function")
	return functions[0]
}

func TestProjectTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestSuite))
}
