//go:build test_unit

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

package dockerclient

import (
	"strings"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/cmdrunner"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ShellClientTestSuite struct {
	suite.Suite
	logger          logger.Logger
	mockedCmdRunner *cmdrunner.MockRunner
	shellClient     *ShellClient
}

func (suite *ShellClientTestSuite) SetupTest() {
	var err error

	// create logger
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err, "Failed to create logger")

	// create mocked cmd runner
	suite.mockedCmdRunner = cmdrunner.NewMockRunner()
	suite.mockedCmdRunner.
		On("Run", mock.Anything, `docker version --format "{{json .}}"`, mock.Anything).
		Return(cmdrunner.RunResult{
			Output: "test",
		}, nil)

	// create docker shell client
	suite.shellClient, err = NewShellClient(suite.logger, suite.mockedCmdRunner)
	suite.Require().NoError(err, "Failed to create shell client")

	suite.shellClient.buildRetryInterval = 1 * time.Millisecond
}

func (suite *ShellClientTestSuite) TestShellClientRunContainerReturnsStdout() {
	testPhrase := "testing"
	suite.mockedCmdRunner.
		On("Run", mock.Anything, "docker run %s %s %s", mock.Anything).
		Return(cmdrunner.RunResult{
			Output: testPhrase,
		}, nil).
		Once()
	output, err := suite.shellClient.RunContainer("alpine",
		&RunOptions{
			ContainerName: "somename",
			Ports:         map[int]int{7779: 7779},
		})
	suite.Require().NoError(err)

	suite.Equal(testPhrase, output)
}

func (suite *ShellClientTestSuite) TestShellClientRunContainerReturnsMultilineStdout() {
	suite.mockedCmdRunner.
		On("Run", mock.Anything, "docker run %s %s %s", mock.Anything).
		Return(cmdrunner.RunResult{
			Output: `
hello world
this is another line
and another
andthisistheid
`,
		}, nil).
		Once()

	containerID, err := suite.shellClient.RunContainer("alpine",
		&RunOptions{
			ContainerName: "somename",
			Ports:         map[int]int{7779: 7779},
		})

	suite.Require().NoError(err)
	suite.Require().Equal("andthisistheid", containerID)
}

func (suite *ShellClientTestSuite) TestShellClientRunContainerReturnsStderr() {
	suite.mockedCmdRunner.
		On("Run", mock.Anything, "docker run %s %s %s", mock.Anything).
		Return(cmdrunner.RunResult{
			Stderr: "foo",
		}, nil).
		Once()
	_, err := suite.shellClient.RunContainer("alpine",
		&RunOptions{
			ContainerName: "somename",
			Ports:         map[int]int{7779: 7779},
		})

	suite.Require().NoError(err)
}

func (suite *ShellClientTestSuite) TestShellClientRunContainerFailsOnNonSingleStdout() {
	suite.mockedCmdRunner.
		On("Run", mock.Anything, "docker run %s %s %s", mock.Anything).
		Return(cmdrunner.RunResult{
			Output: `
hello world
this is another line
and another
andthisistheid with a space`,
		}, nil).
		Once()
	_, err := suite.shellClient.RunContainer("alpine",
		&RunOptions{
			Ports: map[int]int{7779: 7779},
		})

	suite.Require().Error(err, "Output from docker command includes more than just ID")
}

func (suite *ShellClientTestSuite) TestShellClientRunContainerWhenImageMayNotExist() {
	suite.mockedCmdRunner.
		On("Run", mock.Anything, "docker run %s %s %s", mock.Anything).
		Return(cmdrunner.RunResult{
			Output: `
hello world
this is another line
and another
therealidishere
and this a line informing a new version of alpine was pulled. with a space`,
		}, nil).
		Once()

	containerID, err := suite.shellClient.RunContainer("alpine",
		&RunOptions{
			ContainerName:    "somename",
			Ports:            map[int]int{7779: 7779},
			ImageMayNotExist: true,
		})

	suite.Require().NoError(err)
	suite.Require().Equal("therealidishere", containerID)
}

func (suite *ShellClientTestSuite) TestShellClientRunContainerRedactsOutput() {
	suite.mockedCmdRunner.
		On("Run", mock.Anything, "docker run %s %s %s", mock.Anything).
		Return(cmdrunner.RunResult{
			Output: "helloworldsecret",
		}, nil).
		Once()

	suite.shellClient.redactedValues = append(suite.shellClient.redactedValues, "secret")
	output, err := suite.shellClient.RunContainer("alpine",
		&RunOptions{
			ContainerName: "cont",
			Ports:         map[int]int{7779: 7779},
		})

	suite.Require().NoError(err)
	suite.Require().Equal("helloworld[redacted]", output)
}

func (suite *ShellClientTestSuite) TestBuildBailOnUnknownError() {

	// mock failing
	suite.mockedCmdRunner.
		On("Run",
			mock.Anything,
			mock.Anything,
			mock.MatchedBy(func(vars []interface{}) bool {
				command := vars[0].(string)
				return strings.Contains(command, "docker build")
			})).
		Return(cmdrunner.RunResult{
			Stderr: `some bad happened`,
		}, errors.New("unexpected error")).
		Once()

	err := suite.shellClient.Build(&BuildOptions{
		Image:      "image",
		ContextDir: "",
	})
	suite.Require().Error(err)

	// 1 for docker version + 1 unknown build error
	suite.mockedCmdRunner.AssertNumberOfCalls(suite.T(), "Run", 2)
}

func (suite *ShellClientTestSuite) TestBuildRetryOnErrors() {

	// mock failing
	suite.mockedCmdRunner.
		On("Run",
			mock.Anything,
			mock.Anything,
			mock.MatchedBy(func(vars []interface{}) bool {
				command := vars[0].(string)
				return strings.Contains(command, "docker build")
			})).
		Return(cmdrunner.RunResult{
			Stderr: `Unable to find image 'nuclio-onbuild-someid:sometag' locally`,
		}, errors.New("execution error")).
		Twice()

	// success build
	suite.mockedCmdRunner.
		On("Run", mock.Anything, mock.Anything, mock.Anything).
		Return(cmdrunner.RunResult{}, nil)

	err := suite.shellClient.Build(&BuildOptions{
		Image:      "nuclio-onbuild-someid:sometag",
		ContextDir: "",
	})
	suite.Require().Nil(err)

	// 1 for docker version + 2 failing builds + 1 success build
	suite.mockedCmdRunner.AssertNumberOfCalls(suite.T(), "Run", 4)
}

func (suite *ShellClientTestSuite) TestBuildFailValidation() {

	for _, buildOptions := range []BuildOptions{
		{Image: "notValid:1.2.3 | bash 'hi'"},
		{Image: "repo/image:v1.0.0;xyz&netstat"},
		{Image: "repo/image:v1.0.0", BuildArgs: map[string]string{"mm m": "value"}},
	} {
		suite.mockedCmdRunner.
			On("Run",
				mock.Anything,
				mock.MatchedBy(func(command string) bool {
					return strings.Contains(command, "docker build %s")
				}),
				mock.Anything).
			Return(cmdrunner.RunResult{}, nil)

		err := suite.shellClient.Build(&buildOptions)
		suite.logger.DebugWith("Command expectedly failed", "err", err)
		suite.Require().Error(err)
		suite.Require().Contains(err.Error(), "Invalid build options")
		suite.mockedCmdRunner.AssertNumberOfCalls(suite.T(), "Run", 1)
	}
}

func (suite *ShellClientTestSuite) TestRunFailValidation() {

	for _, testCase := range []struct {
		name       string
		imageName  string
		runOptions RunOptions
	}{
		{
			name:       "InvalidContainerName",
			imageName:  "someimage",
			runOptions: RunOptions{ContainerName: "invalid|%#$"},
		},
		{
			name:       "InvalidContainerName2",
			imageName:  "image",
			runOptions: RunOptions{ContainerName: "/nuclio/nuclio-port-change-bvpv1hm0inddvfped4ag"},
		},
		{
			name:       "InvalidEnv",
			imageName:  "image",
			runOptions: RunOptions{ContainerName: "cont", Env: map[string]string{"sdfsd=sdf": "val"}},
		},
		{
			name:       "InvalidImageName",
			imageName:  "bad|name%",
			runOptions: RunOptions{ContainerName: "cont"},
		},
	} {
		suite.Run(testCase.name, func() {
			suite.mockedCmdRunner.
				On("Run",
					mock.Anything,
					mock.MatchedBy(func(command string) bool {
						return strings.Contains(command, "docker run %s")
					}),
					mock.Anything).
				Return(cmdrunner.RunResult{}, nil)

			_, err := suite.shellClient.RunContainer(testCase.imageName, &testCase.runOptions)
			suite.logger.DebugWith("Command expectedly failed", "err", err)
			suite.Require().Error(err)
			suite.Require().True(strings.Contains(err.Error(), "Invalid run options"))
			suite.mockedCmdRunner.AssertNumberOfCalls(suite.T(), "Run", 1)
		})
	}
}

func (suite *ShellClientTestSuite) TestGetContainersEscapesLabelFilterInjection() {

	// on the local platform a remote, unauthenticated client controls the function namespace,
	// which flows into the `nuclio.io/namespace` label value passed here. it must be shell-escaped
	// before interpolation into `docker ps --filter`, otherwise a double quote breaks out of the
	// filter token and injects arbitrary shell run as root in the dashboard container (GHSA-2893-rq73-w22x).
	var capturedArgs []interface{}
	suite.mockedCmdRunner.
		On("Run",
			mock.Anything,
			"docker ps --quiet %s %s %s %s",
			mock.MatchedBy(func(vars []interface{}) bool {
				capturedArgs = vars
				return true
			})).
		Return(cmdrunner.RunResult{
			Output: "",
		}, nil).
		Once()

	_, err := suite.shellClient.GetContainers(&GetContainerOptions{
		Labels: map[string]string{"nuclio.io/namespace": `n";touch /tmp/pwned;echo "`},
	})
	suite.Require().NoError(err)

	// the whole label filter must be a single shell-quoted token; the metacharacters stay
	// literal inside the quotes instead of terminating the docker ps command
	labelFilterArgument := capturedArgs[3].(string)
	suite.Require().Contains(labelFilterArgument,
		`--filter 'label=nuclio.io/namespace=n";touch /tmp/pwned;echo "'`)

	suite.mockedCmdRunner.AssertExpectations(suite.T())
}

// TestRunContainerEnvValue tests both valid env values and injection attempts
func (suite *ShellClientTestSuite) TestRunContainerEnvValue() {
	testCases := []struct {
		name       string
		envName    string
		envValue   string
		shouldFail bool
	}{
		// Happy path - valid env values
		{"valid safe value", "VAR1", "safe_value", false},
		{"valid alphanumeric", "MY_VAR", "value123", false},
		{"valid boolean", "DEBUG", "true", false},
		{"valid port", "PORT", "8080", false},
		{"valid path", "PATH_VAR", "/usr/local/bin:/usr/bin", false},
		{"valid key=value", "CONFIG", "key=value", false},
		{"valid empty", "EMPTY", "", false},
		{"valid dots", "WITH_DOTS", "1.2.3.4", false},
		{"valid dash", "WITH_DASH", "app-name", false},
		{"valid underscore", "WITH_UNDERSCORE", "my_var_name", false},
		// Malicious payloads - shell injection attempts (GHSA-87r5-jx94-x93m, GHSA-r6fg-6j3g-8g5q)
		{"injection single quote touch", "X", "'; touch /tmp/pwned #", true},
		{"injection single quote id", "X", "'; id; echo '", true},
		{"injection double quote touch", "X", `"; touch /tmp/pwned; echo "`, true},
		{"injection double quote id", "X", `"; id; echo "`, true},
		{"injection command substitution touch", "X", "$(touch /tmp/pwned)", true},
		{"injection command substitution redirect", "X", "$(id > /tmp/pwned)", true},
		{"injection embedded command substitution", "X", "value$(whoami)value", true},
		{"injection backtick touch", "X", "`touch /tmp/pwned`", true},
		{"injection backtick id", "X", "`id`", true},
		{"injection semicolon touch", "X", "; touch /tmp/pwned", true},
		{"injection semicolon commands", "X", "value; id; echo", true},
		{"injection AND operator", "X", "value && id", true},
		{"injection OR operator", "X", "value || cat /etc/passwd", true},
		{"injection pipe operator", "X", "value | id", true},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.mockedCmdRunner.
				On("Run", mock.Anything, mock.Anything, mock.Anything).
				Return(cmdrunner.RunResult{Output: "container123"}, nil).
				Once()

			result, err := suite.shellClient.RunContainer("alpine:latest",
				&RunOptions{
					ContainerName: "test",
					Env:           map[string]string{tc.envName: tc.envValue},
				})

			if tc.shouldFail {
				suite.Require().Error(err, "Env injection should be rejected: %s=%s", tc.envName, tc.envValue)
			} else {
				suite.Require().NoError(err, "Valid env should be accepted: %s=%s", tc.envName, tc.envValue)
				suite.Require().Equal(result, "container123", "Expected container ID to be returned for valid env")
			}
			suite.mockedCmdRunner.AssertExpectations(suite.T())
		})
	}
}

// TestRunContainerEnvName tests both valid env names and injection attempts
func (suite *ShellClientTestSuite) TestRunContainerEnvName() {
	testCases := []struct {
		name       string
		envName    string
		shouldFail bool
	}{
		// Happy path
		{"valid simple VAR", "VAR", false},
		{"valid with underscore MY_VAR", "MY_VAR", false},
		{"valid with numbers VAR_123", "VAR_123", false},
		{"valid leading underscore _UNDERSCORE", "_UNDERSCORE", false},
		{"valid all caps CAPS", "CAPS", false},
		{"valid single letter X", "X", false},
		// Malicious payloads
		{"injection single quote touch", "X'; touch /tmp/pwned #", true},
		{"injection double quote id", `X"; id; echo "`, true},
		{"injection command substitution", "X$(whoami)", true},
		{"injection backtick id", "X`id`", true},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.mockedCmdRunner.
				On("Run", mock.Anything, mock.Anything, mock.Anything).
				Return(cmdrunner.RunResult{Output: "container123"}, nil).
				Once()

			result, err := suite.shellClient.RunContainer("alpine:latest",
				&RunOptions{
					ContainerName: "test",
					Env:           map[string]string{tc.envName: "value"},
				})

			if tc.shouldFail {
				suite.Require().Error(err, "Env name injection should be rejected: %s", tc.envName)
			} else {
				suite.Require().NoError(err, "Valid env name should be accepted: %s", tc.envName)
				suite.Require().Equal(result, "container123", "Expected container ID to be returned for valid env name")
			}
			suite.mockedCmdRunner.AssertExpectations(suite.T())
		})
	}
}

// TestRunContainerLabelKey tests both valid label keys and injection attempts
func (suite *ShellClientTestSuite) TestRunContainerLabelKey() {
	testCases := []struct {
		name       string
		labelKey   string
		shouldFail bool
	}{
		// Happy path
		{"happy flow - app", "app", false},
		{"happy flow - app.version", "app.version", false},
		{"happy flow - app/name", "app/name", false},
		{"happy flow - valid url", "com.example.app", false},
		{"happy flow - valid label", "valid-label-123", false},
		{"happy flow - label with underscore", "label_with_underscore", false},
		{"happy flow - name with dash", "my-app-v1", false},
		// Malicious payloads
		{"Malicious payloads- echo", "x'; { echo 'INJECTED'; id; } #", true},
		{"Malicious payloads - touch", "x'; touch /tmp/pwned #", true},
		{"Malicious payloads - touch 2", `x"; touch /tmp/pwned; echo "`, true},
		{"Malicious payloads - echo", `x"; id; echo "`, true},
		{"Malicious payloads - touch3", "x$(touch /tmp/pwned)", true},
		{"Malicious payloads - whoami", "x$(whoami)", true},
		{"Malicious payloads - touch4", "x`touch /tmp/pwned`", true},
		{"Malicious payloads - backquote", "x`id`", true},
		{"Malicious payloads - &&", "x&& id", true},
		{"Malicious payloads - pipeline", "x| cat /etc/passwd", true},
		{"Malicious payloads - touch5", "x; touch /tmp/pwned", true},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.mockedCmdRunner.
				On("Run", mock.Anything, mock.Anything, mock.Anything).
				Return(cmdrunner.RunResult{Output: "container123"}, nil).
				Once()

			result, err := suite.shellClient.RunContainer("alpine:latest",
				&RunOptions{
					ContainerName: "test",
					Labels:        map[string]string{tc.labelKey: "value"},
				})

			if tc.shouldFail {
				suite.Require().Error(err, "Label key injection should be rejected: %s", tc.labelKey)
			} else {
				suite.Require().NoError(err, "Valid label key should be accepted: %s", tc.labelKey)
				suite.Require().Equal(result, "container123", "Expected container ID to be returned for valid label key")
			}
			suite.mockedCmdRunner.AssertExpectations(suite.T())
		})
	}
}

// TestRunContainerVolumePath tests both valid volume paths and injection attempts
func (suite *ShellClientTestSuite) TestRunContainerVolumePath() {
	testCases := []struct {
		name          string
		hostPath      string
		containerPath string
		shouldFail    bool
	}{
		// Happy path
		{"valid /data to /data", "/data", "/data", false},
		{"valid /home/user/project to /app", "/home/user/project", "/app", false},
		{"valid /var/log to /logs", "/var/log", "/logs", false},
		{"valid /etc/config to /config", "/etc/config", "/config", false},
		{"valid /usr/share/app to /opt/app", "/usr/share/app", "/opt/app", false},
		// Malicious payloads - host path injection (no whitespace, so the payload can't be
		// caught incidentally by the loose "no whitespace allowed" volumeNameRegex check)
		{"injection host path single quote touch", "/tmp';touch;#", "/data", true},
		{"injection host path double quote id", `"/tmp";id;echo"`, "/data", true},
		{"injection host path command substitution", "/tmp$(id)", "/data", true},
		{"injection host path backtick id", "/tmp`id`", "/data", true},
		{"injection host path semicolon rm", "/tmp;id", "/data", true},
		// Malicious payloads - container path injection
		{"injection container path single quote touch", "/data", "/tmp';touch;#", true},
		{"injection container path double quote id", "/data", `"/tmp";id;echo"`, true},
		{"injection container path command substitution", "/data", "/tmp$(whoami)", true},
		{"injection container path backtick id", "/data", "/tmp`id`", true},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.mockedCmdRunner.
				On("Run", mock.Anything, mock.Anything, mock.Anything).
				Return(cmdrunner.RunResult{Output: "container123"}, nil).
				Once()

			result, err := suite.shellClient.RunContainer("alpine:latest",
				&RunOptions{
					ContainerName: "test",
					Volumes:       map[string]string{tc.hostPath: tc.containerPath},
				})

			if tc.shouldFail {
				suite.Require().Error(err, "Volume path injection should be rejected")
			} else {
				suite.Require().NoError(err, "Valid volume paths should be accepted")
				suite.Require().Equal(result, "container123", "Expected container ID to be returned for valid volume paths")
			}
			suite.mockedCmdRunner.AssertExpectations(suite.T())
		})
	}
}

// TestBuildArg tests both valid build arguments and injection attempts
func (suite *ShellClientTestSuite) TestBuildArg() {
	testCases := []struct {
		name       string
		argName    string
		argValue   string
		shouldFail bool
	}{
		// Happy path
		{"happy flow - base image", "BASE_IMAGE", "ubuntu:20.04", false},
		{"happy flow - version", "VERSION", "1.2.3", false},
		{"happy flow - build date", "BUILD_DATE", "2026-08-16", false},
		{"happy flow - commit sha", "COMMIT_SHA", "abc123def456", false},
		{"happy flow - app name", "APP_NAME", "myapp", false},
		// Malicious payloads - values (no whitespace, so the payload can't be caught
		// incidentally by the loose "no whitespace allowed" restrictedBuildArgRegex check)
		{"Malicious payloads - values - touch", "ARG", "';touch;#", true},
		{"Malicious payloads - values - echo", "ARG", `";id;echo"`, true},
		{"Malicious payloads - values - touch2", "ARG", "$(id)", true},
		{"Malicious payloads - values - whoami", "ARG", "`whoami`", true},
		{"Malicious payloads - values - rm", "ARG", ";id", true},
		// Malicious payloads - names
		{"Malicious payloads - names - touch", "ARG';touch;#", "value", true},
		{"Malicious payloads - names - echo", `ARG";id;echo"`, "value", true},
		{"Malicious payloads - names - whoami", "ARG$(whoami)", "value", true},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.mockedCmdRunner.
				On("Run", mock.Anything, mock.Anything, mock.MatchedBy(func(vars []interface{}) bool {
					return true
				})).
				Return(cmdrunner.RunResult{}, nil).
				Once()

			err := suite.shellClient.Build(&BuildOptions{
				Image:      "test:latest",
				ContextDir: "/tmp",
				BuildArgs:  map[string]string{tc.argName: tc.argValue},
			})

			if tc.shouldFail {
				suite.Require().Error(err, "Build arg injection should be rejected")
			} else {
				suite.Require().NoError(err, "Valid build arg should be accepted: %s=%s", tc.argName, tc.argValue)
			}
			suite.mockedCmdRunner.AssertExpectations(suite.T())
		})
	}
}

// TestExecInContainerEnv tests both valid ExecInContainer env values and injection attempts
func (suite *ShellClientTestSuite) TestExecInContainerEnv() {
	testCases := []struct {
		name       string
		envName    string
		envValue   string
		shouldFail bool
	}{
		// Happy path
		{"happy flow - exec", "EXEC_VAR", "exec_value", false},
		{"happy flow - debug", "DEBUG", "true", false},
		{"happy flow - log level", "LOG_LEVEL", "INFO", false},
		{"happy flow - timeout", "TIMEOUT", "30", false},
		// Malicious payloads
		{"Malicious payloads - touch", "X", "'; touch /tmp/pwned #", true},
		{"Malicious payloads - echo", "X", `"; id; echo "`, true},
		{"Malicious payloads - touch2", "X", "$(touch /tmp/pwned)", true},
		{"Malicious payloads - whoami", "X", "`whoami`", true},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.mockedCmdRunner.
				On("Run", mock.Anything, mock.Anything, mock.Anything).
				Return(cmdrunner.RunResult{}, nil).
				Once()

			err := suite.shellClient.ExecInContainer("container123",
				&ExecOptions{
					Command: "ls",
					Env:     map[string]string{tc.envName: tc.envValue},
				})

			if tc.shouldFail {
				suite.Require().Error(err, "ExecInContainer env injection should be rejected")
			} else {
				suite.Require().NoError(err, "Valid ExecInContainer env should be accepted")
			}
			suite.mockedCmdRunner.AssertExpectations(suite.T())
		})
	}
}

func TestShellRunnerTestSuite(t *testing.T) {
	suite.Run(t, new(ShellClientTestSuite))
}
