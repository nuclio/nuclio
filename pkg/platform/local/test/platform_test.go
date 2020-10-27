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
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/local"
	processorsuite "github.com/nuclio/nuclio/pkg/processor/test/suite"

	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	processorsuite.TestSuite
	namespace string
}

func (suite *TestSuite) SetupSuite() {
	suite.TestSuite.SetupSuite()
	suite.Runtime = "python:3.6"

	namespaces, err := suite.Platform.GetNamespaces()
	suite.Require().NoError(err, "Failed to get namespaces")

	// we will work on the first one
	suite.namespace = namespaces[0]
}

// Test function containers healthiness validation
func (suite *TestSuite) TestRunFunctionContainerWithCustomRestartPolicy() {
	restartEventsFrom := time.Now()
	functionContainerMaximumRetryCount := 1
	functionPath := []string{suite.GetTestFunctionsDir(), "common", "context-init-fail", "python", "contextinitfail.py"}
	createFunctionOptions := suite.TestSuite.GetDeployOptions("restartable", filepath.Join(functionPath...))
	createFunctionOptions.FunctionConfig.Spec.ReadinessTimeoutSeconds = 10
	createFunctionOptions.FunctionConfig.Meta.Namespace = suite.namespace

	// ensure function is restartable
	createFunctionOptions.FunctionConfig.Spec.Platform.Attributes = map[string]interface{}{}
	createFunctionOptions.FunctionConfig.Spec.Platform.Attributes["restartPolicy"] = dockerclient.RestartPolicy{
		Name:              dockerclient.RestartPolicyNameOnFailure,
		MaximumRetryCount: functionContainerMaximumRetryCount,
	}
	containerName := suite.Platform.(*local.Platform).GetContainerNameByCreateFunctionOptions(createFunctionOptions)
	suite.DeployFunctionExpectError(createFunctionOptions,
		func(deployResult *platform.CreateFunctionResult) bool {

			// give some time to docker to flush its events
			time.Sleep(5 * time.Second)

			// sample container events
			restartEventsUntil := time.Now()
			containerEvents, err := suite.DockerClient.GetContainerEvents(containerName,
				restartEventsFrom.Format(time.RFC3339),
				restartEventsUntil.Format(time.RFC3339))
			suite.Require().NoError(err)
			suite.Require().NotEmpty(containerEvents)

			// count all container die events
			actualTries := 0
			for _, containerEvent := range containerEvents {
				actualTries += strings.Count(containerEvent, "container die")
			}

			// + 1 for first try, rest are just retries
			suite.Require().Equal(functionContainerMaximumRetryCount+1, actualTries)

			return true
		})
}

// Test function containers healthiness validation
func (suite *TestSuite) TestValidateFunctionContainersHealthiness() {
	createFunctionOptions := suite.getDeployOptions("health-validation")
	createFunctionOptions.FunctionConfig.Meta.Namespace = suite.namespace
	suite.DeployFunction(createFunctionOptions,
		func(deployResult *platform.CreateFunctionResult) bool {
			functionName := deployResult.UpdatedFunctionConfig.Meta.Name

			// Check its state is ready
			function := suite.getFunction(functionName)
			suite.Require().Equal(functionconfig.FunctionStateReady, function.GetStatus().State)

			// Stop the container
			err := suite.DockerClient.StopContainer(deployResult.ContainerID)
			suite.Require().NoError(err, "Could not stop container")

			// Trigger function containers healthiness validation
			suite.Platform.(*local.Platform).ValidateFunctionContainersHealthiness()

			// Now the function state should be error
			function = suite.getFunction(functionName)
			suite.Require().Equal(functionconfig.FunctionStateError, function.GetStatus().State)

			// Start the container
			err = suite.DockerClient.StartContainer(deployResult.ContainerID)
			suite.Require().NoError(err, "Failed to start container")

			// Trigger function containers healthiness validation
			suite.Platform.(*local.Platform).ValidateFunctionContainersHealthiness()

			// Function is healthy again
			function = suite.getFunction(functionName)
			suite.Require().Equal(functionconfig.FunctionStateReady, function.GetStatus().State)

			return true
		})
}

// Test function import without deploy and build, then deploy calls build and deploy
func (suite *TestSuite) TestImportFunctionFlow() {

	createFunctionOptions := suite.getDeployOptions("importable")
	createFunctionOptions.FunctionConfig.Meta.Namespace = suite.namespace
	createFunctionOptions.FunctionConfig.Meta.Annotations = map[string]string{
		functionconfig.FunctionAnnotationSkipBuild:  "true",
		functionconfig.FunctionAnnotationSkipDeploy: "true",
	}
	suite.DeployFunctionAndRedeploy(createFunctionOptions,
		func(deployResult *platform.CreateFunctionResult) bool {
			functionName := deployResult.UpdatedFunctionConfig.Meta.Name

			// Check its state is ready
			function := suite.getFunction(functionName)
			functionConfig := function.GetConfig()

			// Check its state is imported and not deployed
			suite.Require().Equal(function.GetStatus().State, functionconfig.FunctionStateImported)

			// Check that the annotations have been removed
			_, skipBuildExists := functionConfig.Meta.Annotations[functionconfig.FunctionAnnotationSkipBuild]
			_, skipDeployExists := functionConfig.Meta.Annotations[functionconfig.FunctionAnnotationSkipDeploy]
			suite.Assert().False(skipBuildExists)
			suite.Assert().False(skipDeployExists)

			createFunctionOptions.FunctionConfig.Meta = functionConfig.Meta
			createFunctionOptions.FunctionConfig.Spec = functionConfig.Spec
			return true
		},
		func(deployResult *platform.CreateFunctionResult) bool {
			functionName := deployResult.UpdatedFunctionConfig.Meta.Name

			// Get the redeployed function
			function := suite.getFunction(functionName)

			// Check its state is ready
			suite.Equal(function.GetStatus().State, functionconfig.FunctionStateReady)
			return true
		})
}

func (suite *TestSuite) TestDeployFunctionVolumeMount() {
	createFunctionOptions := suite.getDeployOptions("volume-mount")
	createFunctionOptions.FunctionConfig.Meta.Namespace = suite.namespace
	createFunctionOptions.FunctionConfig.Spec.Platform.Attributes = map[string]interface{}{
		"processorMountMode": local.ProcessorMountModeVolume,
	}
	localPlatform := suite.Platform.(*local.Platform)
	suite.DeployFunctionAndRedeploy(createFunctionOptions,

		// sanity
		func(deployResult *platform.CreateFunctionResult) bool {
			containers, err := suite.DockerClient.GetContainers(&dockerclient.GetContainerOptions{
				Name: localPlatform.GetContainerNameByCreateFunctionOptions(createFunctionOptions),
			})
			suite.Require().NoError(err, "Failed to get containers")

			containerProcessorMount := containers[0].Mounts[0]
			suite.Require().Equal(string(local.ProcessorMountModeVolume), containerProcessorMount.Type)
			suite.Require().Equal(localPlatform.GetProcessorMountVolumeName(&createFunctionOptions.FunctionConfig), containerProcessorMount.Name)
			suite.Require().Equal(local.FunctionProcessorContainerDirPath, containerProcessorMount.Destination)
			suite.Require().Equal(false, containerProcessorMount.RW)
			return true
		},
		func(deployResult *platform.CreateFunctionResult) bool {
			return true
		})
}

func (suite *TestSuite) getDeployOptions(functionName string) *platform.CreateFunctionOptions {
	functionPath := []string{suite.GetTestFunctionsDir(), "common", "reverser", "python", "reverser.py"}
	createFunctionOptions := suite.TestSuite.GetDeployOptions(functionName, filepath.Join(functionPath...))
	createFunctionOptions.FunctionConfig.Spec.Build.NoBaseImagesPull = true
	return createFunctionOptions

}

func (suite *TestSuite) getFunction(functionName string) platform.Function {
	functions, err := suite.Platform.GetFunctions(&platform.GetFunctionsOptions{
		Name:      functionName,
		Namespace: suite.namespace,
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
