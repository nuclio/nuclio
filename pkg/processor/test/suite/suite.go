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
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/factory"
	"github.com/nuclio/nuclio/pkg/platformconfig"

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

type OnAfterContainerRun func(deployResult *platform.CreateFunctionResult) bool

// TestSuite is a base test suite that offers its children the ability to build
// and run a function, after which the child test can communicate with the
// function container (through an trigger of some sort)
type TestSuite struct {
	suite.Suite
	Logger                logger.Logger
	DockerClient          dockerclient.Client
	Platform              platform.Platform
	TestID                string
	Runtime               string
	RuntimeDir            string
	FunctionDir           string
	PlatformType          string
	Namespace             string
	PlatformConfiguration *platformconfig.Config
	FunctionNameUniquify  bool

	containerID            string
	createdTempDirs        []string
	cleanupCreatedTempDirs bool
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
	var err error

	if suite.RuntimeDir == "" {
		suite.RuntimeDir = suite.Runtime
	}

	if suite.PlatformType == "" {
		suite.PlatformType = "local"
	}

	if suite.Namespace == "" {
		suite.Namespace = "default"
	}

	// this will preserve the current behavior where function names are renamed to be unique upon deployment
	suite.FunctionNameUniquify = true

	suite.Logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	suite.DockerClient, err = dockerclient.NewShellClient(suite.Logger, nil)
	suite.Require().NoError(err)

	suite.Platform, err = factory.CreatePlatform(suite.Logger,
		suite.PlatformType,
		suite.PlatformConfiguration,
		suite.Namespace)
	suite.Require().NoError(err)
	suite.Require().NotNil(suite.Platform)
}

// SetupTest is called before each test in the suite
func (suite *TestSuite) SetupTest() {
	suite.TestID = xid.New().String()
}

// BlastHTTP is a stress test suite
func (suite *TestSuite) BlastHTTP(configuration BlastConfiguration) {
	var totalResults vegeta.Metrics

	// get createFunctionOptions from given blastConfiguration
	createFunctionOptions, err := suite.blastConfigurationToDeployOptions(&configuration)
	suite.Require().NoError(err)

	// deploy the function
	suite.deployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		configuration.URL = fmt.Sprintf("http://%s:%d", suite.GetTestHost(), deployResult.Port)

		err := suite.probeAndWaitForFunctionReadiness(&configuration)
		suite.Require().NoError(err, "Failed to probe and wait for function readiness")

		// blast the function
		totalResults, err = suite.blastFunction(&configuration)
		suite.Require().NoError(err)
		return true
	}, false)

	// delete the function
	if os.Getenv(keepDockerEnvKey) == "" {
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
	return BlastConfiguration{
		Method:        http.MethodGet,
		Workers:       32,
		RatePerWorker: 5,
		Duration:      10 * time.Second,
		URL:           suite.GetTestHost(),
		FunctionName:  "outputter",
		FunctionPath:  "outputter",
		TimeOut:       600 * time.Second,
	}
}

// TearDownTest is called after each test in the suite
func (suite *TestSuite) TearDownTest() {
	suite.Logger.InfoWith("Tearing down test", "testName", suite.T().Name())

	// if we managed to get a container up, dump logs if we failed and remove the container either way
	if suite.containerID != "" {

		if suite.T().Failed() {

			// wait a bit for things to flush
			time.Sleep(2 * time.Second)

			if logs, err := suite.DockerClient.GetContainerLogs(suite.containerID); err == nil {
				suite.Logger.WarnWith("Test failed, retrieved logs", "logs", logs)
			} else {
				suite.Logger.WarnWith("Failed to get docker logs on failure", "err", err)
			}
		}

		if os.Getenv(keepDockerEnvKey) == "" {
			err := suite.DockerClient.RemoveContainer(suite.containerID)
			suite.Require().NoError(err)
		}
	}

	if !suite.T().Skipped() && suite.cleanupCreatedTempDirs {
		for _, tempDir := range suite.createdTempDirs {
			if common.FileExists(tempDir) {
				suite.Failf("", "Temporary dir %s was not cleaned", tempDir)
			}
		}
	}
}

// CreateFunction builds a docker image, runs a container from it and then
// runs onAfterContainerRun
func (suite *TestSuite) DeployFunction(createFunctionOptions *platform.CreateFunctionOptions,
	onAfterContainerRun OnAfterContainerRun) *platform.CreateFunctionResult {
	return suite.deployFunctionPopulateMissingFields(createFunctionOptions,
		onAfterContainerRun,
		false)
}

func (suite *TestSuite) DeployFunctionExpectError(createFunctionOptions *platform.CreateFunctionOptions,
	onAfterContainerRun OnAfterContainerRun) *platform.CreateFunctionResult {
	return suite.deployFunctionPopulateMissingFields(createFunctionOptions,
		onAfterContainerRun,
		true)
}

func (suite *TestSuite) DeployFunctionAndRedeploy(createFunctionOptions *platform.CreateFunctionOptions,
	onAfterFirstContainerRun OnAfterContainerRun,
	onAfterSecondContainerRun OnAfterContainerRun) {

	suite.PopulateDeployOptions(createFunctionOptions)

	// delete the function when done
	defer suite.Platform.DeleteFunction(&platform.DeleteFunctionOptions{ // nolint: errcheck
		FunctionConfig: createFunctionOptions.FunctionConfig,
	})

	suite.deployFunction(createFunctionOptions, onAfterFirstContainerRun, false)
	suite.deployFunction(createFunctionOptions, onAfterSecondContainerRun, false)
}

// GetNuclioSourceDir returns path to nuclio source directory
func (suite *TestSuite) GetNuclioSourceDir() string {
	return common.GetSourceDir()
}

// GetTestFunctionsDir returns the test function dir
func (suite *TestSuite) GetTestFunctionsDir() string {
	return path.Join(suite.GetNuclioSourceDir(), "test", "_functions")
}

// GetTestHost returns the host on which a remote testing entity resides (e.g. brokers, functions)
func (suite *TestSuite) GetTestHost() string {

	// If an env var is set, use that. otherwise localhost
	return common.GetEnvOrDefaultString("NUCLIO_TEST_HOST", "localhost")
}

// GetDeployOptions populates a platform.CreateFunctionOptions structure from function name and path
func (suite *TestSuite) GetDeployOptions(functionName string, functionPath string) *platform.CreateFunctionOptions {
	functionConfig := *functionconfig.NewConfig()
	functionConfig.Spec.ReadinessTimeoutSeconds = 60

	createFunctionOptions := &platform.CreateFunctionOptions{
		Logger:         suite.Logger,
		FunctionConfig: functionConfig,
	}

	createFunctionOptions.FunctionConfig.Meta.Name = functionName
	createFunctionOptions.FunctionConfig.Spec.Runtime = suite.Runtime
	createFunctionOptions.FunctionConfig.Spec.Build.Path = functionPath
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{}

	createFunctionOptions.FunctionConfig.Spec.Build.TempDir = suite.CreateTempDir()

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
	if suite.FunctionNameUniquify && createFunctionOptions.FunctionConfig.Meta.Name != "" {
		createFunctionOptions.FunctionConfig.Meta.Name = suite.GetUniqueFunctionName(createFunctionOptions.FunctionConfig.Meta.Name)
	}

	// don't explicitly pull base images before building
	createFunctionOptions.FunctionConfig.Spec.Build.NoBaseImagesPull = true

	// Does the test call for cleaning up the temp dir, and thus needs to check this on teardown
	suite.cleanupCreatedTempDirs = !createFunctionOptions.FunctionConfig.Spec.Build.NoCleanup
}

func (suite *TestSuite) GetUniqueFunctionName(name string) string {
	uniqueFunctionName := fmt.Sprintf("%s-%s", name, suite.TestID)

	// k8s maximum name limit
	k8sMaxNameLength := 63
	if len(uniqueFunctionName) > k8sMaxNameLength {

		// trims
		uniqueFunctionName = uniqueFunctionName[:k8sMaxNameLength-len(uniqueFunctionName)]
	}

	// to not reach
	return uniqueFunctionName
}

func (suite *TestSuite) GetRuntimeDir() string {
	if suite.RuntimeDir != "" {
		return suite.RuntimeDir
	}

	return suite.Runtime
}

func (suite *TestSuite) CreateTempDir() string {
	tempDir, err := ioutil.TempDir("", "build-test-*")
	suite.Require().NoError(err, "Failed to create temporary dir")
	suite.createdTempDirs = append(suite.createdTempDirs, tempDir)
	return tempDir
}

// return appropriate CreateFunctionOptions for given blast configuration
func (suite *TestSuite) blastConfigurationToDeployOptions(request *BlastConfiguration) (*platform.CreateFunctionOptions, error) {

	// Set createFunctionOptions of example function "outputter"
	createFunctionOptions := suite.GetDeployOptions(request.FunctionName,
		suite.GetFunctionPath(request.FunctionPath))

	// Configure deployOptions properties, number of MaxWorkers like in the default stress request
	createFunctionOptions.FunctionConfig.Meta.Name =
		fmt.Sprintf("%s-%s",
			createFunctionOptions.FunctionConfig.Meta.Name,
			suite.TestID)
	createFunctionOptions.FunctionConfig.Spec.Build.NoBaseImagesPull = true
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		"httpTrigger": {
			Kind:       "http",
			MaxWorkers: request.Workers,
		},
	}
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

func (suite *TestSuite) deployFunctionPopulateMissingFields(createFunctionOptions *platform.CreateFunctionOptions,
	onAfterContainerRun OnAfterContainerRun,
	expectFailure bool) *platform.CreateFunctionResult {

	var deployResult *platform.CreateFunctionResult

	// add some commonly used options to createFunctionOptions
	suite.PopulateDeployOptions(createFunctionOptions)

	// delete the function when done
	defer func() {

		// if we didnt deploy yet, try remove what is created
		if deployResult == nil {
			suite.Platform.DeleteFunction(&platform.DeleteFunctionOptions{ // nolint: errcheck
				FunctionConfig: createFunctionOptions.FunctionConfig,
			})
		} else {

			// if we did deploy, remove what was deployed
			suite.Platform.DeleteFunction(&platform.DeleteFunctionOptions{ // nolint: errcheck
				FunctionConfig: deployResult.UpdatedFunctionConfig,
			})
		}
	}()
	deployResult = suite.deployFunction(createFunctionOptions, onAfterContainerRun, expectFailure)
	return deployResult
}

func (suite *TestSuite) deployFunction(createFunctionOptions *platform.CreateFunctionOptions,
	onAfterContainerRun OnAfterContainerRun,
	expectError bool) *platform.CreateFunctionResult {

	// deploy the function
	deployResult, err := suite.Platform.CreateFunction(createFunctionOptions)

	if !expectError {
		suite.Require().NoError(err)
	} else {
		suite.Require().Error(err)
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

func (suite *TestSuite) probeAndWaitForFunctionReadiness(configuration *BlastConfiguration) error {

	// sending some probe requests to function, to ensure it responses before blasting it
	return common.RetryUntilSuccessful(30*time.Second, 1*time.Second, func() bool {

		// create a request
		httpRequest, err := http.NewRequest(configuration.Method, configuration.URL, nil)
		suite.Require().NoError(err)

		suite.Logger.DebugWith("Sending function probe request",
			"configurationMethod", configuration.Method,
			"configurationURL", configuration.URL)
		httpResponse, responseErr := http.DefaultClient.Do(httpRequest)

		// if we fail to connect, fail
		if responseErr != nil {
			if strings.Contains(responseErr.Error(), "EOF") ||
				strings.Contains(responseErr.Error(), "connection reset by peer") {

				// function isn't ready yet, give it another try
				suite.Logger.DebugWith("Function is not ready yet, retrying")
				return false
			}

			// if we got here, we failed on something more fatal, fail test
			suite.Fail("Function probing failed",
				"responseErr", responseErr)
		}

		// we need at least one good answer in the range of [200, 500)
		return http.StatusOK <= httpResponse.StatusCode && httpResponse.StatusCode < http.StatusInternalServerError
	})
}
