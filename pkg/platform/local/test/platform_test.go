//go:build test_integration && test_local

/*
Copyright 2023 The Nuclio Authors.

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
	"context"
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
	ctx       context.Context
}

func (suite *TestSuite) SetupSuite() {
	suite.TestSuite.SetupSuite()
	suite.ctx = context.Background()
	suite.Runtime = "python"

	namespaces, err := suite.Platform.GetNamespaces(suite.ctx)
	suite.Require().NoError(err, "Failed to get namespaces")

	// we will work on the first one
	suite.namespace = namespaces[0]

	getProjectsOptions := &platform.CreateProjectOptions{
		ProjectConfig: &platform.ProjectConfig{Meta: platform.ProjectMeta{Name: platform.DefaultProjectName, Namespace: suite.namespace}, Spec: platform.ProjectSpec{
			Description: "just a description",
		}},
	}
	err = suite.Platform.CreateProject(suite.ctx, getProjectsOptions)
	suite.Require().NoError(err, "Failed to create project")
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
	containerName := suite.Platform.(*local.Platform).
		GetFunctionContainerName(&createFunctionOptions.FunctionConfig)
	suite.DeployFunctionExpectError(createFunctionOptions, // nolint: errcheck
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
			suite.NotEmpty(deployResult, "Function hasn't been deployed")
			functionName := deployResult.UpdatedFunctionConfig.Meta.Name

			// Ensure function state is ready
			suite.WaitForFunctionState(&platform.GetFunctionsOptions{
				Name:      functionName,
				Namespace: suite.namespace,
			}, functionconfig.FunctionStateReady, time.Second)

			// Stop the container
			err := suite.DockerClient.StopContainer(deployResult.ContainerID)
			suite.Require().NoError(err, "Could not stop container")

			// Trigger function containers healthiness validation
			go suite.Platform.(*local.Platform).ValidateFunctionContainersHealthiness(suite.ctx)

			// Wait for function to become unhealthy
			suite.WaitForFunctionState(&platform.GetFunctionsOptions{
				Name:      functionName,
				Namespace: suite.namespace,
			}, functionconfig.FunctionStateUnhealthy, time.Minute)

			// Start the container
			err = suite.DockerClient.StartContainer(deployResult.ContainerID)
			suite.Require().NoError(err, "Failed to start container")

			// Trigger function containers healthiness validation
			go suite.Platform.(*local.Platform).ValidateFunctionContainersHealthiness(suite.ctx)

			suite.WaitForFunctionState(&platform.GetFunctionsOptions{
				Name:      functionName,
				Namespace: suite.namespace,
			}, functionconfig.FunctionStateReady, time.Minute)

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

			suite.WaitForFunctionState(&platform.GetFunctionsOptions{
				Name:      deployResult.UpdatedFunctionConfig.Meta.Name,
				Namespace: suite.namespace,
			}, functionconfig.FunctionStateImported, time.Second)

			// Check its state is ready
			function := suite.GetFunction(&platform.GetFunctionsOptions{
				Name:      functionName,
				Namespace: suite.namespace,
			})
			functionConfig := function.GetConfig()

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
			suite.WaitForFunctionState(&platform.GetFunctionsOptions{
				Name:      deployResult.UpdatedFunctionConfig.Meta.Name,
				Namespace: suite.namespace,
			}, functionconfig.FunctionStateReady, time.Second)
			return true
		})
}

// Test deploying a function using volume mount
func (suite *TestSuite) TestDeployFunctionVolumeMount() {
	createFunctionOptions := suite.getDeployOptions("volume-mount")
	createFunctionOptions.FunctionConfig.Meta.Namespace = suite.namespace
	localPlatform := suite.Platform.(*local.Platform)
	suite.DeployFunctionAndRedeploy(createFunctionOptions,

		// sanity
		func(deployResult *platform.CreateFunctionResult) bool {
			containers, err := suite.DockerClient.GetContainers(&dockerclient.GetContainerOptions{
				Name: localPlatform.GetFunctionContainerName(&createFunctionOptions.FunctionConfig),
			})
			suite.Require().NoError(err, "Failed to get containers")

			containerMount := containers[0].Mounts[0]

			// processor is mounted by a volume mount
			suite.Require().Equal("volume", containerMount.Type)
			suite.Require().Equal(localPlatform.GetFunctionVolumeMountName(&createFunctionOptions.FunctionConfig), containerMount.Name)
			suite.Require().Equal(local.FunctionProcessorContainerDirPath, containerMount.Destination)
			suite.Require().Equal(false, containerMount.RW)
			return true
		},

		// Re-deploy to ensure even if its volume mount exists - it would be used.
		func(deployResult *platform.CreateFunctionResult) bool {
			return true
		})
}

// Test deleting a function while its volume mount is missing
func (suite *TestSuite) TestDeleteFunctionMissingVolumeMount() {
	createFunctionOptions := suite.getDeployOptions("missing-volume-mount")
	createFunctionOptions.FunctionConfig.Meta.Namespace = suite.namespace
	localPlatform := suite.Platform.(*local.Platform)
	suite.DeployFunction(createFunctionOptions,

		// sanity
		func(deployResult *platform.CreateFunctionResult) bool {
			containers, err := suite.DockerClient.GetContainers(&dockerclient.GetContainerOptions{
				Name: localPlatform.GetFunctionContainerName(&createFunctionOptions.FunctionConfig),
			})
			suite.Require().NoError(err, "Failed to get containers")

			functionVolumeMountName := localPlatform.GetFunctionVolumeMountName(&createFunctionOptions.FunctionConfig)

			// stop container
			err = suite.DockerClient.RemoveContainer(containers[0].ID)
			suite.Require().NoError(err)

			// delete its volume
			err = suite.DockerClient.DeleteVolume(functionVolumeMountName)
			suite.Require().NoError(err)

			// ensure delete function succeeded
			err = suite.Platform.DeleteFunction(suite.ctx,
				&platform.DeleteFunctionOptions{
					FunctionConfig: functionconfig.Config{
						Meta: createFunctionOptions.FunctionConfig.Meta,
					},
				})
			suite.Require().NoError(err)
			return true
		})
}

func (suite *TestSuite) TestRedeployFunction() {
	createFunctionOptions := suite.getDeployOptions("redeployed-func")
	createFunctionOptions.FunctionConfig.Meta.Namespace = suite.namespace
	localPlatform := suite.Platform.(*local.Platform)

	isFunctionDeployed := make(chan bool)
	isRedeploySuccessful := make(chan bool)

	go func() {
		// redeploy the function once it is deployed
		<-isFunctionDeployed
		err := localPlatform.RedeployFunction(suite.ctx, &platform.RedeployFunctionOptions{
			FunctionSpec: &createFunctionOptions.FunctionConfig.Spec,
			FunctionMeta: &createFunctionOptions.FunctionConfig.Meta,
		})
		suite.Require().NoError(err)
		isRedeploySuccessful <- true
	}()

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		suite.Require().NotNil(deployResult, "Expected deploy result not to be nil")

		// get container id
		containerId := suite.getFunctionContainerId(localPlatform, &createFunctionOptions.FunctionConfig)

		// signal that function is deployed
		isFunctionDeployed <- true

		// wait for redeploy to complete
		<-isRedeploySuccessful

		// get container id again and check that it is changed
		newContainerId := suite.getFunctionContainerId(localPlatform, &createFunctionOptions.FunctionConfig)
		suite.Require().NotEqual(newContainerId, containerId)

		return true
	})
}

func (suite *TestSuite) TestDeployFunctionDisabledDefaultHttpTrigger() {
	createFunctionOptions := suite.getDeployOptions("disable-default-http")
	createFunctionOptions.FunctionConfig.Meta.Namespace = suite.namespace
	trueValue := true
	createFunctionOptions.FunctionConfig.Spec.DisableDefaultHTTPTrigger = &trueValue
	localPlatform := suite.Platform.(*local.Platform)
	suite.DeployFunction(createFunctionOptions,

		// sanity
		func(deployResult *platform.CreateFunctionResult) bool {
			containerId := suite.getFunctionContainerId(localPlatform, &createFunctionOptions.FunctionConfig)
			suite.Require().NotEqual("", containerId)
			return true
		})
}

func (suite *TestSuite) TestDeployFunctionDisablePublishingPorts() {
	createFunctionOptions := suite.getDeployOptions("no-publish-ports")
	createFunctionOptions.FunctionConfig.Meta.Namespace = suite.namespace
	localPlatform := suite.Platform.(*local.Platform)

	for _, testCase := range []struct {
		name          string
		useAttributes bool
	}{
		{
			name:          "DisableWithTriggerAttributes",
			useAttributes: true,
		},
		{
			name: "DisableWithAnnotation",
		},
	} {
		suite.Run(testCase.name, func() {

			if testCase.useAttributes {
				// use trigger attributes to disable publishing ports
				createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
					"http": {
						Kind: "http",
						Attributes: map[string]interface{}{
							"disablePortPublishing": true,
						},
					},
				}
			} else {
				// use trigger annotation to disable publishing ports
				createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
					"http": {
						Kind: "http",
						Annotations: map[string]string{
							"nuclio.io/disable-port-publishing": "true",
						},
					},
				}
			}

			suite.DeployFunction(createFunctionOptions,
				func(deployResult *platform.CreateFunctionResult) bool {
					containers, err := suite.DockerClient.GetContainers(&dockerclient.GetContainerOptions{
						Name: localPlatform.GetFunctionContainerName(&createFunctionOptions.FunctionConfig),
					})
					suite.Require().NoError(err, "Failed to get containers")
					suite.Require().Len(containers, 1, "Expected to get one container")

					// check that the container is not published
					suite.Require().Empty(containers[0].NetworkSettings.Ports)

					return true
				})
		})
	}
}

func (suite *TestSuite) getDeployOptions(functionName string) *platform.CreateFunctionOptions {
	functionPath := []string{suite.GetTestFunctionsDir(), "common", "reverser", "python", "reverser.py"}
	createFunctionOptions := suite.TestSuite.GetDeployOptions(functionName, filepath.Join(functionPath...))
	createFunctionOptions.FunctionConfig.Spec.Build.NoBaseImagesPull = true
	return createFunctionOptions
}

func (suite *TestSuite) getFunctionContainerId(localPlatform *local.Platform, config *functionconfig.Config) string {
	containers, err := suite.DockerClient.GetContainers(&dockerclient.GetContainerOptions{
		Name: localPlatform.GetFunctionContainerName(config),
	})
	suite.Require().NoError(err, "Failed to get containers")
	suite.Require().Len(containers, 1, "Expected to get one container")
	return containers[0].ID
}

func TestProjectTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestSuite))
}
