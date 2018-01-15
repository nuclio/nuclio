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
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/local"
	"github.com/nuclio/nuclio/pkg/version"
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/nuclio/nuclio-sdk"
	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
	"github.com/tsenart/vegeta/lib"
)

const (
	keepDockerEnvKey = "NUCLIO_TEST_KEEP_DOCKER"
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
	Platform     platform.Platform
	TestID       string
	Runtime      string
	FunctionDir  string
	containerID  string
	TempDir      string
	CleanupTemp  bool
}

// StressRequest holds information for blastHTTP function
type StressRequest struct {
	Url         string
	Method      string
	RatePerWorker        uint64
	Duration    time.Duration
	Workers uint64
	TimeOut int32
	FunctionName string
	FunctionPath string
	Handler string
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

	suite.DockerClient, err = dockerclient.NewShellClient(suite.Logger, nil)
	suite.Require().NoError(err)

	suite.Platform, err = local.NewPlatform(suite.Logger)
	suite.Require().NoError(err)
}

// SetupTest is called before each test in the suite
func (suite *TestSuite) SetupTest() {
	suite.TestID = xid.New().String()
}

// BlastHTTP is a stress test suite, that checks
func (suite *TestSuite) BlastHTTP(request StressRequest) bool {

	// set deployOptions of example function "outputter"
	deployOptions := suite.GetDeployOptions(request.FunctionName,
		suite.GetFunctionPath(request.FunctionPath))

	// configure deployOptipns properties, number of MaxWorkers like in the default stress request - 32
	deployOptions.FunctionConfig.Meta.Name = fmt.Sprintf("%s-%s", deployOptions.FunctionConfig.Meta.Name, suite.TestID)
	deployOptions.FunctionConfig.Spec.Build.NuclioSourceDir = suite.GetNuclioSourceDir()
	deployOptions.FunctionConfig.Spec.Build.NoBaseImagesPull = true
	deployOptions.FunctionConfig.Spec.HTTPPort = 8080

	// check for specific Handler
	if request.Handler != ""{
		deployOptions.FunctionConfig.Spec.Handler = request.Handler
	}

	defaultHTTPTriggerConfiguration := functionconfig.Trigger{
		Kind:       "http",
		MaxWorkers: 32,
		URL:        ":8080",
	}
	deployOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{"trigger":defaultHTTPTriggerConfiguration}

	// deploy the function
	_, err := suite.Platform.DeployFunction(deployOptions)
	suite.Require().NoError(err)

	// the variable that will store connection result
	var totalResults vegeta.Metrics

	// Initialize target according to request
	target := vegeta.NewStaticTargeter(vegeta.Target{
		Method: request.Method,
		URL:    request.Url,
	})

	// Initialize attacker with given number of workers, timeout proportionally to Duration
	attacker := vegeta.NewAttacker(vegeta.Workers(request.Workers), vegeta.Timeout(request.Duration * 4 * time.Second))

	// Attack + add connection result to results, make rate -> rate by worker by multiplication
	for res := range attacker.Attack(target, request.RatePerWorker * request.Workers, request.Duration) {
		totalResults.Add(res)
	}

	// Close vegeta's metrics, no longer needed.
	totalResults.Close()

	// delete the function
	err = suite.Platform.DeleteFunction(&platform.DeleteOptions{
		FunctionConfig: deployOptions.FunctionConfig,
	})

	suite.Require().NoError(err)

	// Debug with test results
	suite.Logger.Debug("Total tests success percentage %v", float32(totalResults.Success*100))
	suite.Logger.DebugWith("error received are", "errors", totalResults.Errors)

	// totalResults.Success is the success percentage in float64 (0.9 -> 90%), return true if all tests succeeded
	return int(totalResults.Success) != 0
}

// GetDefaultStressRequest populate StressRequest struct with default values
func (suite *TestSuite) GetDefaultStressRequest() StressRequest {

	// Initialize default request
	request := StressRequest{Method: "GET", Workers: 10, RatePerWorker: 1,
		Duration: 10 * time.Second, Url: "http://localhost:8080",
		FunctionName: "outputter", FunctionPath: "outputter"}

	return request
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

		if os.Getenv(keepDockerEnvKey) == "" {
			suite.DockerClient.RemoveContainer(suite.containerID)
		}
	}

	if suite.CleanupTemp && common.FileExists(suite.TempDir) {
		suite.Failf("", "Temporary dir %s was not cleaned", suite.TempDir)
	}
}

// DeployFunction builds a docker image, runs a container from it and then
// runs onAfterContainerRun
func (suite *TestSuite) DeployFunction(deployOptions *platform.DeployOptions,
	onAfterContainerRun func(deployResult *platform.DeployResult) bool) *platform.DeployResult {

	deployOptions.FunctionConfig.Meta.Name = fmt.Sprintf("%s-%s", deployOptions.FunctionConfig.Meta.Name, suite.TestID)
	deployOptions.FunctionConfig.Spec.Build.NuclioSourceDir = suite.GetNuclioSourceDir()
	deployOptions.FunctionConfig.Spec.Build.NoBaseImagesPull = true

	// Does the test call for cleaning up the temp dir, and thus needs to check this on teardown
	suite.CleanupTemp = !deployOptions.FunctionConfig.Spec.Build.NoCleanup

	// deploy the function
	deployResult, err := suite.Platform.DeployFunction(deployOptions)
	suite.Require().NoError(err)

	// remove the image when we're done
	if os.Getenv(keepDockerEnvKey) == "" {
		defer suite.DockerClient.RemoveImage(deployResult.ImageName)
	}

	// give the container some time - after 10 seconds, give up
	deadline := time.Now().Add(10 * time.Second)

	for {

		// stop after 10 seconds
		if time.Now().After(deadline) {
			var dockerLogs string

			dockerLogs, err = suite.DockerClient.GetContainerLogs(deployResult.ContainerID)
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

	suite.TempDir = suite.createTempDir()
	deployOptions.FunctionConfig.Spec.Build.TempDir = suite.TempDir

	return deployOptions
}

// GetFunctionPath returns the non-relative function path (given a relative path)
func (suite *TestSuite) GetFunctionPath(functionRelativePath ...string) string {

	// functionPath = FunctionDir + functionRelativePath
	functionPath := []string{suite.FunctionDir}
	functionPath = append(functionPath, functionRelativePath...)

	return path.Join(functionPath...)
}

func (suite *TestSuite) createTempDir() string {
	tempDir, err := ioutil.TempDir("", "build-test-"+suite.TestID)
	if err != nil {
		suite.FailNowf("Failed to create temporary dir %s for test %s", suite.TempDir, suite.TestID)
	}

	return tempDir
}
