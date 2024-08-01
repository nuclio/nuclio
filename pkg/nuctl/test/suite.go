//go:build test_integration && (test_kube || test_local)

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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/nuctl/command"
	nuctlcommon "github.com/nuclio/nuclio/pkg/nuctl/command/common"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	"sigs.k8s.io/yaml"
)

const (
	nuctlPlatformEnvVarName = "NUCTL_PLATFORM"
)

type Suite struct {
	suite.Suite
	platformKindOverride string
	origPlatformKind     string
	logger               logger.Logger
	dockerClient         dockerclient.Client
	shellClient          *cmdrunner.ShellRunner
	outputBuffer         bytes.Buffer
	stdinReader          *strings.Reader
	defaultWaitDuration  time.Duration
	defaultWaitInterval  time.Duration
	tempDir              string
	namespace            string
	ctx                  context.Context
}

func (suite *Suite) SetupSuite() {
	var err error

	common.SetVersionFromEnv()

	// create logger
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	// create shell runner
	suite.shellClient, err = cmdrunner.NewShellRunner(suite.logger)
	suite.Require().NoError(err)

	// create docker client
	suite.dockerClient, err = dockerclient.NewShellClient(suite.logger, suite.shellClient)
	suite.Require().NoError(err)

	// save platform kind before the test
	suite.origPlatformKind = os.Getenv(nuctlPlatformEnvVarName)

	// init default wait values to be used during tests for retries
	suite.defaultWaitDuration = 1 * time.Minute
	suite.defaultWaitInterval = 5 * time.Second

	suite.namespace = common.GetEnvOrDefaultString("NUCTL_NAMESPACE", "nuclio")

	// platform kind has been overridden - use it
	if suite.platformKindOverride != "" {
		suite.logger.DebugWith("Overriding platform kind",
			"platformKindOverride", suite.platformKindOverride)
		suite.origPlatformKind = suite.platformKindOverride
	}

	// default to local platform if platform isn't set
	if os.Getenv(nuctlPlatformEnvVarName) == "" {
		err = os.Setenv(nuctlPlatformEnvVarName, common.LocalPlatformName)
		suite.Require().NoError(err)
	}

	suite.tempDir, _ = os.MkdirTemp("", "nuctl-tests")

	suite.ctx = context.Background()

	// create project
	suite.ExecuteNuctl([]string{"create", "project", platform.DefaultProjectName}, map[string]string{}) // nolint: errcheck
}

func (suite *Suite) SetupTest() {
	suite.outputBuffer.Reset()

	// each test initializes it upon usage
	suite.stdinReader = nil
}

func (suite *Suite) TearDownSuite() {
	suite.logger.Debug("Tearing down suite")

	// restore platform kind
	err := os.Setenv(nuctlPlatformEnvVarName, suite.origPlatformKind)
	suite.Require().NoError(err)

	err = os.RemoveAll(suite.tempDir)
	suite.Require().NoError(err, "Failed to remove temp dir - %s", suite.tempDir)

	suite.logger.Debug("Suite tear down completed")
}

// ExecuteNuctl populates os.Args and executes nuctl as if it were executed from shell
func (suite *Suite) ExecuteNuctl(positionalArgs []string,
	namedArgs map[string]string) error {

	// reset buffer to ensure it contains only last executed command
	suite.outputBuffer.Reset()
	rootCommandeer := command.NewRootCommandeer()

	// set the output, so we can capture it (but also output to stdout)
	rootCommandeer.GetCmd().SetOut(io.MultiWriter(os.Stdout, &suite.outputBuffer))

	// set the input so we can write to stdin
	if suite.stdinReader != nil {
		rootCommandeer.GetCmd().SetIn(suite.stdinReader)
	}

	var argsStringSlice []string

	// add positional arguments
	argsStringSlice = append(argsStringSlice, positionalArgs...)

	for argName, argValue := range namedArgs {
		argsStringSlice = append(argsStringSlice, fmt.Sprintf("--%s", argName), argValue)
	}

	if suite.isNamespaceRequired() && !suite.namespaceInArgs(positionalArgs, namedArgs) {
		// prepend namespace to args
		argsStringSlice = common.PrependStringsToStringSlice(argsStringSlice, "--namespace", suite.namespace)
	}

	// since args[0] is the executable name, just shove the binary there
	argsStringSlice = common.PrependStringToStringSlice(argsStringSlice, "nuctl")

	// override os.Args (this can't go wrong horribly, can it?)
	os.Args = argsStringSlice

	suite.logger.DebugWith("Executing nuctl", "args", argsStringSlice)

	// execute
	return rootCommandeer.Execute()
}

// RetryExecuteNuctlUntilSuccessful executes nuctl until expectFailure is met
func (suite *Suite) RetryExecuteNuctlUntilSuccessful(positionalArgs []string,
	namedArgs map[string]string,
	expectFailure bool) error {

	return common.RetryUntilSuccessful(suite.defaultWaitDuration,
		suite.defaultWaitInterval,
		func() bool {

			defer func() {

				// upon retry, ensure stdin data is readable again
				if suite.stdinReader != nil {
					suite.stdinReader.Seek(0, io.SeekStart) // nolint: errcheck
				}
			}()

			// execute
			err := suite.ExecuteNuctl(positionalArgs, namedArgs)
			if expectFailure {
				return err != nil
			}

			// retry
			if err != nil {
				suite.logger.WarnWithCtx(suite.ctx,
					"Nuctl execution failed, retrying",
					"errStack", errors.GetErrorStackString(err, 10))
				return false
			}
			return true
		})
}

// GetNuclioSourceDir returns path to nuclio source directory
func (suite *Suite) GetNuclioSourceDir() string {
	return common.GetSourceDir()
}

// GetFunctionsDir returns path to nuclio source directory
func (suite *Suite) GetFunctionsDir() string {
	return path.Join(suite.GetNuclioSourceDir(), "test", "_functions")
}

func (suite *Suite) GetFunctionConfigsDir() string {
	return path.Join(suite.GetNuclioSourceDir(), "test", "_function_configs")
}

func (suite *Suite) GetExamples() string {
	return path.Join(suite.GetNuclioSourceDir(), "hack", "examples")
}

func (suite *Suite) GetImportsDir() string {
	return path.Join(suite.GetNuclioSourceDir(), "test", "_imports")
}

func (suite *Suite) findPatternsInOutput(patternsMustExist []string, patternsMustNotExist []string) {
	foundPatternsMustExist := make([]bool, len(patternsMustExist))
	foundPatternsMustNotExist := make([]bool, len(patternsMustNotExist))

	// iterate over all lines in result
	scanner := bufio.NewScanner(&suite.outputBuffer)
	for scanner.Scan() {

		for patternIdx, patternName := range patternsMustExist {

			// one line may match more than one pattern
			if strings.Contains(scanner.Text(), patternName) {
				foundPatternsMustExist[patternIdx] = true
			}
		}

		for patternIdx, patternName := range patternsMustNotExist {
			if strings.Contains(scanner.Text(), patternName) {
				foundPatternsMustNotExist[patternIdx] = true
				break
			}
		}
	}

	// all patterns that must exist must exist
	for _, foundPattern := range foundPatternsMustExist {
		suite.Require().True(foundPattern)
	}

	// all patterns that must not exist must not exist
	for _, foundPattern := range foundPatternsMustNotExist {
		suite.Require().False(foundPattern)
	}
}

func (suite *Suite) verifyAPIGatewayExists(apiGatewayName, primaryFunctionName string) {

	// reset output buffer for reading the nex output cleanly
	suite.outputBuffer.Reset()
	err := suite.RetryExecuteNuctlUntilSuccessful([]string{"get", "agw", apiGatewayName}, map[string]string{
		"output": nuctlcommon.OutputFormatYAML,
	}, false)
	suite.Require().NoError(err)

	apiGateway := platform.APIGatewayConfig{}
	apiGatewayBodyBytes := suite.outputBuffer.Bytes()
	err = yaml.Unmarshal(apiGatewayBodyBytes, &apiGateway)
	suite.Require().NoError(err)

	suite.Assert().Equal(apiGatewayName, apiGateway.Meta.Name)
	suite.Assert().Equal(primaryFunctionName, apiGateway.Spec.Upstreams[0].NuclioFunction.Name)
}

func (suite *Suite) assertFunctionImported(functionName string, imported bool) {

	// reset output buffer for reading the nex output cleanly
	suite.outputBuffer.Reset()
	err := suite.RetryExecuteNuctlUntilSuccessful([]string{"get", "function", functionName}, map[string]string{
		"output": nuctlcommon.OutputFormatYAML,
	}, false)
	suite.Require().NoError(err)

	function := functionconfig.Config{}
	functionBodyBytes := suite.outputBuffer.Bytes()
	err = yaml.Unmarshal(functionBodyBytes, &function)
	suite.Require().NoError(err)

	suite.Require().Equal(functionName, function.Meta.Name)
	if imported {

		// get imported functions
		err = suite.ExecuteNuctl([]string{"get", "function", functionName}, nil)
		suite.Require().NoError(err)

		// ensure function state is imported
		suite.findPatternsInOutput([]string{"imported"}, nil)
	}
}

func (suite *Suite) getFunctionInFormat(functionName string,
	outputFormat string) (*functionconfig.ConfigWithStatus, error) {
	suite.outputBuffer.Reset()
	var err error

	suite.Require().NotEmpty(outputFormat, "Output format must not be empty")
	suite.Require().NotEmpty(functionName, "Function name must not be empty")

	// get function in format
	if err = suite.ExecuteNuctl([]string{"get", "function", functionName},
		map[string]string{
			"output": outputFormat,
		}); err != nil {
		return nil, errors.Wrapf(err, "Failed to get function %s", functionName)
	}

	parsedFunction := functionconfig.ConfigWithStatus{}

	// unmarshal response correspondingly to output format
	switch outputFormat {
	case nuctlcommon.OutputFormatJSON:
		err = json.Unmarshal(suite.outputBuffer.Bytes(), &parsedFunction)
	case nuctlcommon.OutputFormatYAML:
		err = yaml.Unmarshal(suite.outputBuffer.Bytes(), &parsedFunction)
	default:
		return nil, errors.Errorf("Invalid output format %s", outputFormat)
	}

	return &parsedFunction, err
}

func (suite *Suite) waitForFunctionState(functionName string, expectedState functionconfig.FunctionState) {
	err := common.RetryUntilSuccessful(1*time.Minute, 5*time.Second, func() bool {
		functionConfigWithStatus, err := suite.getFunctionInFormat(functionName, nuctlcommon.OutputFormatYAML)
		if err != nil {
			suite.logger.ErrorWith("Waiting for function readiness failed", "err", err)
			return false
		}
		if functionConfigWithStatus.Status.State != expectedState {
			suite.logger.DebugWith("Function state is not ready yet",
				"expectedState", expectedState,
				"currentState", functionConfigWithStatus.Status.State)
			return false
		}
		return true
	})
	suite.Require().NoErrorf(err,
		"Failed to wait for function '%s' with expected state '%s'",
		functionName,
		expectedState)
}

func (suite *Suite) writeFunctionConfigToTempFile(functionConfig *functionconfig.Config,
	tempFilePattern string) string {

	// create a temp function yaml to be used with test modified values
	functionConfigPath, err := os.CreateTemp("", tempFilePattern)
	suite.Require().NoError(err)

	// close when done writing
	defer functionConfigPath.Close() // nolint: errcheck

	// dump modified function config to temp function configuration file
	marshaledFunctionConfig, err := yaml.Marshal(functionConfig)
	suite.Require().NoError(err)

	_, err = functionConfigPath.Write(marshaledFunctionConfig)
	suite.Require().NoError(err)

	// ensure file is written to disk
	err = functionConfigPath.Sync()
	suite.Require().NoError(err)

	return functionConfigPath.Name()
}

func (suite *Suite) ensureRunningOnPlatform(expectedPlatformKind string) {
	if suite.origPlatformKind != expectedPlatformKind {
		suite.T().Skipf("Skipping test, unmatched platform kind (%s != %s)",
			expectedPlatformKind,
			suite.origPlatformKind)
	}
}

func (suite *Suite) namespaceInArgs(positionalArgs []string, namedArgs map[string]string) bool {
	if common.StringSliceContainsString(positionalArgs, "--namespace") || common.StringSliceContainsString(positionalArgs, "-n") {
		return true
	}

	if _, ok := namedArgs["namespace"]; ok {
		return true
	}

	return false
}

func (suite *Suite) isNamespaceRequired() bool {
	return suite.namespace != "" && common.GetKubeconfigPath("") != ""
}
