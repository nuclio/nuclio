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

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
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
	Logger       logger.Logger
	DockerClient dockerclient.Client
	Platform     platform.Platform
	TestID       string
	Runtime      string
	RuntimeDir   string
	FunctionDir  string
	containerID  string
	TempDir      string
	CleanupTemp  bool
}

// BlastRequest holds information for BlastHTTP function
type BlastConfiguration struct {
	Duration      time.Duration
	TimeOut       time.Duration
	URL           string
	Method        string
	FunctionName  string
	FunctionPath  string
	Handler       string
	RatePerWorker int
	Workers       int
}

// SetupSuite is called for suite setup
func (suite *TestSuite) SetupSuite() {
	if suite.RuntimeDir == "" {
		suite.RuntimeDir = suite.Runtime
	}

	// update version so that linker doesn't need to inject it
	err := version.Set(&version.Info{
		GitCommit: "c",
		Label:     "latest",
		Arch:      "amd64",
		OS:        "linux",
	})
	suite.Require().NoError(err)

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

// BlastHTTP is a stress test suite
func (suite *TestSuite) BlastHTTP(configuration BlastConfiguration) {

	// get createFunctionOptions from given blastConfiguration
	createFunctionOptions, err := suite.blastConfigurationToDeployOptions(&configuration)
	suite.Require().NoError(err)

	// deploy the function
	_, err = suite.Platform.CreateFunction(createFunctionOptions)
	suite.Require().NoError(err)

	// blast the function
	totalResults, err := suite.blastFunction(&configuration)
	suite.Require().NoError(err)

	// delete the function
	if os.Getenv("NUCLIO_TEST_KEEP_DOCKER") == "" {
		err = suite.Platform.DeleteFunction(&platform.DeleteFunctionOptions{
			FunctionConfig: createFunctionOptions.FunctionConfig,
		})
		suite.Require().NoError(err)
	}

	// debug with test results
	suite.Logger.DebugWith("BlastHTTP results",
		"totalResults.Success", totalResults.Success*100.00,
		"totalResults.Errors", totalResults.Errors)

	// totalResults.Success is the success percentage in float64 (0.9 -> 90%), require that it's above a threshold
	suite.Require().True(totalResults.Success >= 0.95)
}

// NewBlastConfiguration populates BlastRequest struct with default values
func (suite *TestSuite) NewBlastConfiguration() BlastConfiguration {

	// default host configuration
	host := "localhost"

	// Check if situation is dockerized, if so set url to host
	if os.Getenv("NUCLIO_TEST_HOST") != "" {
		host = os.Getenv("NUCLIO_TEST_HOST")
	}

	request := BlastConfiguration{Method: "GET", Workers: 32, RatePerWorker: 5,
		Duration: 10 * time.Second, URL: fmt.Sprintf("http://%s:8080", host),
		FunctionName: "outputter", FunctionPath: "outputter", TimeOut: time.Second * 600}

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
			err := suite.DockerClient.RemoveContainer(suite.containerID)
			suite.Require().NoError(err)
		}
	}

	if !suite.T().Skipped() && suite.CleanupTemp && common.FileExists(suite.TempDir) {
		suite.Failf("", "Temporary dir %s was not cleaned", suite.TempDir)
	}
}

// CreateFunction builds a docker image, runs a container from it and then
// runs onAfterContainerRun
func (suite *TestSuite) DeployFunction(createFunctionOptions *platform.CreateFunctionOptions,
	onAfterContainerRun func(deployResult *platform.CreateFunctionResult) bool) *platform.CreateFunctionResult {

	// add some commonly used options to createFunctionOptions
	suite.PopulateDeployOptions(createFunctionOptions)

	// deploy the function
	deployResult, err := suite.Platform.CreateFunction(createFunctionOptions)
	suite.Require().NoError(err)

	// delete the function when done
	defer suite.Platform.DeleteFunction(&platform.DeleteFunctionOptions{ // nolint: errcheck
		FunctionConfig: createFunctionOptions.FunctionConfig,
	})

	// remove the image when we're done
	if os.Getenv(keepDockerEnvKey) == "" {
		defer suite.DockerClient.RemoveImage(deployResult.Image) // nolint: errcheck
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

	return deployResult
}

// GetNuclioSourceDir returns path to nuclio source directory
func (suite *TestSuite) GetNuclioSourceDir() string {
	return path.Join(os.Getenv("GOPATH"), "src", "github.com", "nuclio", "nuclio")
}

// GetTestFunctionsDir returns the test function dir
func (suite *TestSuite) GetTestFunctionsDir() string {
	return path.Join(suite.GetNuclioSourceDir(), "test", "_functions")
}

// GetTestHost returns the host on which a remote testing entity resides (e.g. brokers, functions)
func (suite *TestSuite) GetTestHost() string {

	// If an env var is set, use that. otherwise localhost
	if os.Getenv("NUCLIO_TEST_HOST") != "" {
		return os.Getenv("NUCLIO_TEST_HOST")
	}

	return "localhost"
}

// GetDeployOptions populates a platform.CreateFunctionOptions structure from function name and path
func (suite *TestSuite) GetDeployOptions(functionName string, functionPath string) *platform.CreateFunctionOptions {
	readinessTimeout := 60 * time.Second

	createFunctionOptions := &platform.CreateFunctionOptions{
		Logger:           suite.Logger,
		FunctionConfig:   *functionconfig.NewConfig(),
		ReadinessTimeout: &readinessTimeout,
	}

	createFunctionOptions.FunctionConfig.Meta.Name = functionName
	createFunctionOptions.FunctionConfig.Spec.Runtime = suite.Runtime
	createFunctionOptions.FunctionConfig.Spec.Build.Path = functionPath

	suite.TempDir = suite.createTempDir()
	createFunctionOptions.FunctionConfig.Spec.Build.TempDir = suite.TempDir

	return createFunctionOptions
}

// GetFunctionPath returns the non-relative function path (given a relative path)
func (suite *TestSuite) GetFunctionPath(functionRelativePath ...string) string {

	// functionPath = FunctionDir + functionRelativePath
	functionPath := []string{suite.FunctionDir}
	functionPath = append(functionPath, functionRelativePath...)

	return path.Join(functionPath...)
}

// adds some commonly-used fields to the given CreateFunctionOptions
func (suite *TestSuite) PopulateDeployOptions(createFunctionOptions *platform.CreateFunctionOptions) {

	// give the name a unique prefix, except if name isn't set
	// TODO: will affect concurrent runs
	if createFunctionOptions.FunctionConfig.Meta.Name != "" {
		createFunctionOptions.FunctionConfig.Meta.Name = fmt.Sprintf("%s-%s", createFunctionOptions.FunctionConfig.Meta.Name, suite.TestID)
	}

	// don't explicitly pull base images before building
	createFunctionOptions.FunctionConfig.Spec.Build.NoBaseImagesPull = true

	// Does the test call for cleaning up the temp dir, and thus needs to check this on teardown
	suite.CleanupTemp = !createFunctionOptions.FunctionConfig.Spec.Build.NoCleanup
}

func (suite *TestSuite) GetRuntimeDir() string {
	if suite.RuntimeDir != "" {
		return suite.RuntimeDir
	}

	return suite.Runtime
}

func (suite *TestSuite) createTempDir() string {
	tempDir, err := ioutil.TempDir("", "build-test-"+suite.TestID)
	if err != nil {
		suite.FailNowf("Failed to create temporary dir %s for test %s", suite.TempDir, suite.TestID)
	}

	return tempDir
}

// return appropriate CreateFunctionOptions for given blast configuration
func (suite *TestSuite) blastConfigurationToDeployOptions(request *BlastConfiguration) (*platform.CreateFunctionOptions, error) {

	// Set createFunctionOptions of example function "outputter"
	createFunctionOptions := suite.GetDeployOptions(request.FunctionName,
		suite.GetFunctionPath(request.FunctionPath))

	// Configure deployOptipns properties, number of MaxWorkers like in the default stress request - 32
	createFunctionOptions.FunctionConfig.Meta.Name = fmt.Sprintf("%s-%s", createFunctionOptions.FunctionConfig.Meta.Name, suite.TestID)
	createFunctionOptions.FunctionConfig.Spec.Build.NoBaseImagesPull = true
	defaultHTTPTriggerConfiguration := functionconfig.Trigger{
		Kind:       "http",
		MaxWorkers: 32,
		URL:        ":8080",
		Attributes: map[string]interface{}{
			"port": 8080,
		},
	}
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{"trigger": defaultHTTPTriggerConfiguration}
	createFunctionOptions.FunctionConfig.Spec.Handler = request.Handler

	return createFunctionOptions, nil
}

// Blast function using vegeta's attacker & given BlastConfiguration
func (suite *TestSuite) blastFunction(configuration *BlastConfiguration) (vegeta.Metrics, error) {

	// The variable that will store connection result
	totalResults := vegeta.Metrics{}

	// Initialize target according to request
	target := vegeta.NewStaticTargeter(vegeta.Target{
		Method: configuration.Method,
		URL:    configuration.URL,
	})

	// Initialize attacker with given number of workers, timeout about 1 minute
	attacker := vegeta.NewAttacker(vegeta.Workers(uint64(configuration.Workers)), vegeta.Timeout(configuration.TimeOut))

	// Attack + add connection result to results, make rate -> rate by worker by multiplication
	for res := range attacker.Attack(target, uint64(configuration.Workers*configuration.RatePerWorker), configuration.Duration) {
		totalResults.Add(res)
	}

	// Close vegeta's metrics, no longer needed
	totalResults.Close()

	suite.Logger.InfoWith("attack results", "results", totalResults.Errors, "target", target)

	return totalResults, nil
}
