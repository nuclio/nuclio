//go:build test_integration && (test_kube || test_local)

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
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuctlcommon "github.com/nuclio/nuclio/pkg/nuctl/command/common"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube"
	"github.com/nuclio/nuclio/pkg/platform/kube/client"
	"github.com/nuclio/nuclio/pkg/processor/build"

	"github.com/ghodss/yaml"
	"github.com/gobuffalo/flect"
	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
)

type functionBuildTestSuite struct {
	Suite
}

func (suite *functionBuildTestSuite) TestBuild() {
	uniqueSuffix := "-" + xid.New().String()
	functionName := "reverser-build" + uniqueSuffix
	imageName := "nuclio/processor-" + functionName

	err := suite.ExecuteNuctl([]string{"build", functionName, "--verbose", "--no-pull"},
		map[string]string{
			"path":    path.Join(suite.GetFunctionsDir(), "common", "reverser", "golang"),
			"runtime": "golang",
		})

	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName) // nolint: errcheck

	// use deploy with the image we just created
	err = suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose"},
		map[string]string{
			"run-image": imageName,
			"runtime":   "golang",
			"handler":   "main:Reverse",
		})

	suite.Require().NoError(err)

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

	// try a few times to invoke, until it succeeds
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"invoke", functionName},
		map[string]string{
			"method": "POST",
			"body":   "-reverse this string+",
			"via":    "external-ip",
		},
		false)
	suite.Require().NoError(err)

	// make sure reverser worked
	suite.Require().Contains(suite.outputBuffer.String(), "+gnirts siht esrever-")
}

type functionDeployTestSuite struct {
	Suite
}

func (suite *functionDeployTestSuite) TestDeploy() {
	for _, runtimeInfo := range []struct {
		runtime  string
		handler  string
		filename string
	}{
		{
			runtime:  "golang",
			handler:  "empty:Handler",
			filename: "empty.go",
		},
		{
			runtime:  "java",
			handler:  "EmptyHandler",
			filename: "EmptyHandler.java",
		},
		{
			runtime:  "nodejs",
			handler:  "empty:handler",
			filename: "empty.js",
		},
		{
			runtime:  "dotnetcore",
			handler:  "nuclio:empty",
			filename: "empty.cs",
		},
		{
			runtime:  "python:3.6",
			handler:  "empty:handler",
			filename: "empty.py",
		},
		{
			runtime:  "python:3.7",
			handler:  "empty:handler",
			filename: "empty.py",
		},
		{
			runtime:  "python:3.8",
			handler:  "empty:handler",
			filename: "empty.py",
		},
		{
			runtime:  "python:3.9",
			handler:  "empty:handler",
			filename: "empty.py",
		},
		{
			runtime:  "ruby",
			handler:  "empty:main",
			filename: "empty.rb",
		},
		{
			runtime:  "shell",
			handler:  "empty.sh:main",
			filename: "empty.sh",
		},
	} {
		suite.Run(runtimeInfo.runtime, func() {
			runtimeName, _ := common.GetRuntimeNameAndVersion(runtimeInfo.runtime)
			functionName := fmt.Sprintf("test-%s-%s",
				flect.Dasherize(runtimeInfo.runtime),
				xid.New().String())
			namedArgs := map[string]string{
				"path":    path.Join(suite.GetExamples(), runtimeName, "empty", runtimeInfo.filename),
				"runtime": runtimeInfo.runtime,
				"handler": runtimeInfo.handler,
			}
			suite.logger.DebugWith("Deploying function",
				"functionName", functionName,
				"namedArgs", namedArgs,
			)
			err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"}, namedArgs)
			suite.Require().NoError(err)

			// use nutctl to delete the function when we're done
			defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

			// try a few times to invoke, until it succeeds
			err = suite.RetryExecuteNuctlUntilSuccessful([]string{"invoke", functionName},
				map[string]string{
					"via": "external-ip",
				},
				false)
			suite.Require().NoError(err)
		})
	}
}

func (suite *functionDeployTestSuite) TestInvokeWithBodyFromStdin() {
	uniqueSuffix := "-" + xid.New().String()
	functionName := "invoke-body-stdin-reverser" + uniqueSuffix
	imageName := "nuclio/processor-" + functionName
	namedArgs := map[string]string{
		"path":    path.Join(suite.GetFunctionsDir(), "common", "reverser", "python"),
		"runtime": "python",
		"handler": "reverser:handler",
	}

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"}, namedArgs)

	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName) // nolint: errcheck

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

	suite.inputBuffer = bytes.Buffer{}
	suite.inputBuffer.WriteString("-reverse this string+")

	// try a few times to invoke, until it succeeds
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"invoke", functionName},
		map[string]string{
			"method": "POST",
			"via":    "external-ip",
		},
		false)
	suite.Require().NoError(err)

	// make sure reverser worked
	suite.Require().Contains(suite.outputBuffer.String(), "+gnirts siht esrever-")
}

func (suite *functionDeployTestSuite) TestInvokeWithTimeout() {
	uniqueSuffix := "-" + xid.New().String()
	functionName := "invoke-with-timeout" + uniqueSuffix
	imageName := "nuclio/processor-" + functionName
	timeoutSeconds := 3
	timeoutSourceCode := fmt.Sprintf("sleep %d\necho done", timeoutSeconds)
	namedArgs := map[string]string{
		"image":   imageName,
		"runtime": "shell",
		"handler": "main.sh",
		"source":  base64.StdEncoding.EncodeToString([]byte(timeoutSourceCode)),
	}

	// deploy function
	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"}, namedArgs)
	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName) // nolint: errcheck

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

	// try a few times to invoke, until it succeeds
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"invoke", functionName},
		map[string]string{
			"method":  "POST",
			"via":     "external-ip",
			"timeout": platform.FunctionInvocationDefaultTimeout.String(),
		},
		false)
	suite.Require().NoError(err)

	// fail to invoke due to timeout error
	err = suite.ExecuteNuctl([]string{"invoke", functionName},
		map[string]string{
			"method":  "POST",
			"via":     "external-ip",
			"timeout": (time.Duration(timeoutSeconds-1) * time.Second).String(),
		})
	suite.Require().Error(err)
}

func (suite *functionDeployTestSuite) TestDeployWithMetadata() {
	uniqueSuffix := "-" + xid.New().String()
	functionName := "envprinter" + uniqueSuffix
	imageName := "nuclio/processor-" + functionName

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"},
		map[string]string{
			"path":        path.Join(suite.GetFunctionsDir(), "common", "envprinter", "python"),
			"env":         "FIRST_ENV=11223344,SECOND_ENV=0099887766",
			"labels":      "label1=first,label2=second",
			"annotations": "annotation1=third,annotation2=fourth",
			"runtime":     "python",
			"handler":     "envprinter:handler",
		})

	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName) // nolint: errcheck

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

	// try a few times to invoke, until it succeeds
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"invoke", functionName},
		map[string]string{
			"method": http.MethodPost,
			"via":    "external-ip",
		},
		false)
	suite.Require().NoError(err)

	// make sure reverser worked
	suite.Require().Contains(suite.outputBuffer.String(), "11223344")
	suite.Require().Contains(suite.outputBuffer.String(), "0099887766")
}

func (suite *functionDeployTestSuite) TestDeployFromFunctionConfig() {
	randomString := xid.New().String()

	functionPath := path.Join(suite.GetFunctionsDir(), "common", "json-parser-with-function-config", "python")
	functionConfig := functionconfig.Config{}
	functionBody, err := ioutil.ReadFile(filepath.Join(functionPath, build.FunctionConfigFileName))
	suite.Require().NoError(err)
	err = yaml.Unmarshal(functionBody, &functionConfig)
	suite.Require().NoError(err)
	functionName := functionConfig.Meta.Name
	imageName := "nuclio/processor-" + functionName

	err = suite.ExecuteNuctl([]string{"deploy", "", "--verbose", "--no-pull"},
		map[string]string{
			"path":  path.Join(suite.GetFunctionsDir(), "common", "json-parser-with-function-config", "python"),
			"image": imageName,
		})

	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName) // nolint: errcheck

	// clear output buffer from last invocation
	suite.outputBuffer.Reset()

	// get the function
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"get", "fu", functionName}, map[string]string{
		"output": nuctlcommon.OutputFormatYAML,
	}, false)
	suite.Require().NoError(err)

	deployedFunctionConfig := functionconfig.Config{}
	err = yaml.Unmarshal(suite.outputBuffer.Bytes(), &deployedFunctionConfig)
	suite.Require().NoError(err)

	// the function has 1 http trigger - api. here we are verifying the default HTTP trigger wasn't added
	suite.Require().Equal(1, len(functionconfig.GetTriggersByKind(deployedFunctionConfig.Spec.Triggers, "http")))
	suite.Require().Equal("http", deployedFunctionConfig.Spec.Triggers["api"].Kind)

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

	// try a few times to invoke, until it succeeds
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"invoke", functionName},
		map[string]string{
			"method":       "POST",
			"body":         fmt.Sprintf(`{"return_this": "%s"}`, randomString),
			"content-type": "application/json",
			"via":          "external-ip",
		},
		false)
	suite.Require().NoError(err)

	// check that invoke printed the value
	suite.Require().Contains(suite.outputBuffer.String(), randomString)
}

func (suite *functionDeployTestSuite) TestDeployFromCodeEntryTypeS3InvalidValues() {
	uniqueSuffix := "-" + xid.New().String()
	functionName := "s3-fast-failure" + uniqueSuffix
	imageName := "nuclio/processor-" + functionName

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName) // nolint: errcheck

	// deploy function with invalid s3 values
	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"},
		map[string]string{
			"file":            path.Join(suite.GetFunctionConfigsDir(), "error", "s3_codeentry/function.yaml"),
			"image":           imageName,
			"code-entry-type": "s3",
		})
	suite.Require().Error(err)
	suite.Contains(errors.GetErrorStackString(err, 5), "Failed to download file from s3")
}

func (suite *functionDeployTestSuite) TestInvokeWithLogging() {
	uniqueSuffix := "-" + xid.New().String()
	functionName := "invoke-logging" + uniqueSuffix
	imageName := "nuclio/processor-" + functionName

	namedArgs := map[string]string{
		"path":    path.Join(suite.GetFunctionsDir(), "common", "logging", "golang"),
		"runtime": "golang",
		"handler": "main:Logging",
	}

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"}, namedArgs)
	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName) // nolint: errcheck

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

	for _, testCase := range []struct {
		logLevel           string
		expectedMessages   []string
		unexpectedMessages []string
	}{
		{
			logLevel: "none",
			unexpectedMessages: []string{
				"Debug message",
				"Info message",
				"Warn message",
				"Error message",
			},
		},
		{
			logLevel: "debug",
			expectedMessages: []string{
				"Debug message",
				"Info message",
				"Warn message",
				"Error message",
			},
		},
		{
			logLevel: "info",
			expectedMessages: []string{
				"Info message",
				"Warn message",
				"Error message",
			},
			unexpectedMessages: []string{
				"Debug message",
			},
		},
		{
			logLevel: "warn",
			expectedMessages: []string{
				"Warn message",
				"Error message",
			},
			unexpectedMessages: []string{
				"Debug message",
				"Info message",
			},
		},
		{
			logLevel: "error",
			expectedMessages: []string{
				"Error message",
			},
			unexpectedMessages: []string{
				"Debug message",
				"Info message",
				"Warn message",
			},
		},
	} {
		// clear output buffer from last invocation
		suite.outputBuffer.Reset()

		// invoke the function
		err = suite.RetryExecuteNuctlUntilSuccessful([]string{"invoke", functionName},
			map[string]string{
				"method":    "POST",
				"log-level": testCase.logLevel,
				"via":       "external-ip",
			}, false)
		suite.Require().NoError(err)

		// make sure expected strings are in output
		for _, expectedMessage := range testCase.expectedMessages {
			suite.Require().Contains(suite.outputBuffer.String(), expectedMessage)
		}

		// make sure unexpected strings are NOT in output
		for _, unexpectedMessage := range testCase.unexpectedMessages {
			suite.Require().NotContains(suite.outputBuffer.String(), unexpectedMessage)
		}
	}
}

func (suite *functionDeployTestSuite) TestDeployFailsOnMissingPath() {
	uniqueSuffix := "-" + xid.New().String()
	functionName := "missing-path-reverser" + uniqueSuffix
	imageName := "nuclio/processor-" + functionName

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName) // nolint: errcheck

	// use nuctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"},
		map[string]string{
			"runtime": "golang",
			"handler": "main:Reverse",
		})

	suite.Require().Error(err, "Function code must be provided either in the path or inline in a spec file; alternatively, an image or handler may be provided")

	// ensure get functions succeeded for failing functions
	err = suite.ExecuteNuctl([]string{"get", "function"}, nil)
	suite.Require().NoError(err)
}

func (suite *functionDeployTestSuite) TestDeployFailsOnShellMissingPathAndHandler() {
	uniqueSuffix := "-" + xid.New().String()
	functionName := "missing-path" + uniqueSuffix
	imageName := "nuclio/processor-" + functionName

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName) // nolint: errcheck

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"},
		map[string]string{
			"runtime": "shell",
		})

	suite.Require().Error(err, "Function code must be provided either in the path or inline in a spec file; alternatively, an image or handler may be provided")
}

func (suite *functionDeployTestSuite) TestDeployShellViaHandler() {
	uniqueSuffix := "-" + xid.New().String()
	functionName := "shell-handler" + uniqueSuffix
	imageName := "nuclio/processor-" + functionName

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"},
		map[string]string{
			"image":   imageName,
			"runtime": "shell",
			"handler": "rev",
		})

	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName) // nolint: errcheck

	// use nuctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

	// try a few times to invoke, until it succeeds
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"invoke", functionName},
		map[string]string{
			"method": "POST",
			"body":   "-reverse this string+",
			"via":    "external-ip",
		},
		false)
	suite.Require().NoError(err)

	// make sure reverser worked
	suite.Require().Contains(suite.outputBuffer.String(), "+gnirts siht esrever-")
}

func (suite *functionDeployTestSuite) TestDeployWithFunctionEvent() {
	uniqueSuffix := "-" + xid.New().String()
	functionName := "function-event" + uniqueSuffix
	imageName := "nuclio/processor-" + functionName
	functionEventName := "reverser-event" + uniqueSuffix

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"},
		map[string]string{
			"image":   imageName,
			"runtime": "shell",
			"handler": "rev",
		})

	suite.Require().NoError(err)

	// ensure function created
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"get", "function", functionName}, nil, false)
	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName) // nolint: errcheck

	// create a function event using nuctl
	err = suite.ExecuteNuctl([]string{"create", "functionevent", functionEventName},
		map[string]string{
			"function": functionName,
		})
	suite.Require().NoError(err)

	// check to see we have created the function event
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"get", "functionevent", functionEventName}, nil, false)
	suite.Require().NoError(err)

	// delete the function
	err = suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)
	suite.Require().NoError(err)

	// ensure function created
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"get", "function", functionName}, nil, true)
	suite.Require().NoError(err)

	// check to see the function event was deleted as well
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"get", "functionevent", functionEventName}, nil, true)
	suite.Require().NoError(err)

	// reset buffer
	suite.outputBuffer.Reset()

	// get function events
	err = suite.ExecuteNuctl([]string{"get", "functionevent"}, nil)
	suite.Require().NoError(err)

	// make sure function event names is not in get result
	suite.findPatternsInOutput(nil, []string{functionEventName})
}

func (suite *functionDeployTestSuite) TestBuildWithSaveDeployWithLoad() {
	uniqueSuffix := "-" + xid.New().String()
	functionName := "reverser" + uniqueSuffix
	imageName := "nuclio/build-test" + uniqueSuffix
	tarName := functionName + ".tar"

	err := suite.ExecuteNuctl([]string{"build", functionName, "--verbose", "--no-pull"},
		map[string]string{
			"path":              path.Join(suite.GetFunctionsDir(), "common", "reverser", "golang"),
			"image":             imageName,
			"runtime":           "golang",
			"output-image-file": tarName,
		})

	suite.Require().NoError(err)

	// delete the current image to see that load works
	err = suite.dockerClient.RemoveImage(imageName)
	suite.Require().NoError(err)

	//  make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName)    // nolint: errcheck
	defer suite.shellClient.Run(nil, "rm %s", tarName) // nolint: errcheck

	// use deploy with the image we just created
	err = suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose"},
		map[string]string{
			"run-image":        imageName,
			"runtime":          "golang",
			"handler":          "main:Reverse",
			"input-image-file": tarName,
		})

	suite.Require().NoError(err)

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

	// try a few times to invoke, until it succeeds
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"invoke", functionName},
		map[string]string{
			"method": "POST",
			"body":   "-reverse this string+",
			"via":    "external-ip",
		},
		false)
	suite.Require().NoError(err)

	// make sure reverser worked
	suite.Require().Contains(suite.outputBuffer.String(), "+gnirts siht esrever-")
}

func (suite *functionDeployTestSuite) TestBuildAndDeployFromFile() {
	randomString := xid.New().String()
	uniqueSuffix := "-" + xid.New().String()
	functionPath := path.Join(suite.GetFunctionsDir(), "common", "json-parser-with-function-config", "python")
	functionConfig := functionconfig.Config{}
	functionBody, err := ioutil.ReadFile(filepath.Join(functionPath, build.FunctionConfigFileName))
	suite.Require().NoError(err)
	err = yaml.Unmarshal(functionBody, &functionConfig)
	suite.Require().NoError(err)
	functionName := functionConfig.Meta.Name + uniqueSuffix
	imageName := "nuclio/processor-" + functionName + uniqueSuffix

	err = suite.ExecuteNuctl([]string{"build", functionName, "--verbose", "--no-pull"},
		map[string]string{
			"path":  path.Join(suite.GetFunctionsDir(), "common", "json-parser-with-function-config", "python"),
			"image": imageName,
		})

	suite.Require().NoError(err)

	//  make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName) // nolint: errcheck

	// use deploy with the image we just created
	err = suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose"},
		map[string]string{
			"run-image": imageName,
			"file":      path.Join(suite.GetFunctionsDir(), "common", "json-parser-with-function-config", "python", "function-different-spec.yaml"),
		})

	suite.Require().NoError(err)

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

	// reset output buffer for reading the nex output cleanly
	suite.outputBuffer.Reset()

	// export the function
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"export", "fu", functionName}, nil, false)
	suite.Require().NoError(err)

	deployedFunctionConfig := functionconfig.Config{}
	err = yaml.Unmarshal(suite.outputBuffer.Bytes(), &deployedFunctionConfig)
	suite.Require().NoError(err)

	// assert data from different spec and not original spec
	suite.Assert().Equal(2, *deployedFunctionConfig.Spec.MinReplicas)
	suite.Assert().Equal(6, *deployedFunctionConfig.Spec.MaxReplicas)

	// check that created trigger is default trigger and not the original one
	suite.Assert().Equal(1, len(deployedFunctionConfig.Spec.Triggers))
	suite.Assert().Contains(deployedFunctionConfig.Spec.Triggers, "default-http")

	// try a few times to invoke, until it succeeds
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"invoke", functionName},
		map[string]string{
			"method":       "POST",
			"body":         fmt.Sprintf(`{"return_this": "%s"}`, randomString),
			"content-type": "application/json",
			"via":          "external-ip",
		},
		false)
	suite.Require().NoError(err)

	// check that invoke printed the value
	suite.Require().Contains(suite.outputBuffer.String(), randomString)
}

func (suite *functionDeployTestSuite) TestBuildAndDeployFromFileWithOverriddenArgs() {
	randomString := xid.New().String()
	uniqueSuffix := "-" + xid.New().String()
	functionPath := path.Join(suite.GetFunctionsDir(), "common", "json-parser-with-function-config", "python")
	functionConfig := functionconfig.Config{}
	functionBody, err := ioutil.ReadFile(filepath.Join(functionPath, build.FunctionConfigFileName))
	suite.Require().NoError(err)
	err = yaml.Unmarshal(functionBody, &functionConfig)
	suite.Require().NoError(err)
	functionName := functionConfig.Meta.Name + uniqueSuffix
	imageName := "nuclio/processor-" + functionName + uniqueSuffix

	minReplicas := 3

	err = suite.ExecuteNuctl([]string{"build", functionName, "--verbose", "--no-pull"},
		map[string]string{
			"path":  functionPath,
			"image": imageName,
		})

	suite.Require().NoError(err)

	//  make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName) // nolint: errcheck

	// use deploy with the image we just created
	err = suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose"},
		map[string]string{
			"run-image":    imageName,
			"file":         path.Join(suite.GetFunctionsDir(), "common", "json-parser-with-function-config", "python", "function-different-spec.yaml"),
			"min-replicas": fmt.Sprintf("%d", minReplicas),
		})

	suite.Require().NoError(err)

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

	// reset output buffer for reading the nex output cleanly
	suite.outputBuffer.Reset()

	// export the function
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"export", "fu", functionName}, nil, false)
	suite.Require().NoError(err)

	deployedFunctionConfig := functionconfig.Config{}
	err = yaml.Unmarshal(suite.outputBuffer.Bytes(), &deployedFunctionConfig)
	suite.Require().NoError(err)

	// assert data from different spec and not original spec
	suite.Assert().Equal(6, *deployedFunctionConfig.Spec.MaxReplicas)

	// check that created trigger is default trigger and not the original one
	suite.Assert().Equal(1, len(deployedFunctionConfig.Spec.Triggers))
	suite.Assert().Contains(deployedFunctionConfig.Spec.Triggers, "default-http")

	// assert from args and not from either spec
	suite.Assert().Equal(minReplicas, *deployedFunctionConfig.Spec.MinReplicas)

	// try a few times to invoke, until it succeeds
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"invoke", functionName},
		map[string]string{
			"method":       "POST",
			"body":         fmt.Sprintf(`{"return_this": "%s"}`, randomString),
			"content-type": "application/json",
			"via":          "external-ip",
		},
		false)
	suite.Require().NoError(err)

	// check that invoke printed the value
	suite.Require().Contains(suite.outputBuffer.String(), randomString)
}

func (suite *functionDeployTestSuite) TestDeployWithResourceVersion() {

	// TODO: when we enable some sort of resource validation on other platforms, allow this to run on those as well
	suite.ensureRunningOnPlatform("kube")

	// read and parse the function we're gonna test
	functionConfig := functionconfig.Config{}
	uniqueSuffix := "-" + xid.New().String()
	functionPath := path.Join(suite.GetFunctionsDir(), "common", "json-parser-with-function-config", "python")
	functionBody, err := ioutil.ReadFile(filepath.Join(functionPath, build.FunctionConfigFileName))
	suite.Require().NoError(err)
	err = yaml.Unmarshal(functionBody, &functionConfig)
	suite.Require().NoError(err)

	// name uniqueness
	functionConfig.Meta.Name += uniqueSuffix

	// ensure no resource version
	functionConfig.Meta.ResourceVersion = ""

	// --- step 1 ---
	// deploy first time, should successfully create the function

	// write function config to file
	functionConfigPath := suite.writeFunctionConfigToTempFile(&functionConfig, "resource-version-*.yaml")

	// remove when done
	defer os.RemoveAll(functionConfigPath) // nolint: errcheck

	// deploy with temp, expect to pass
	err = suite.ExecuteNuctl([]string{"deploy", functionConfig.Meta.Name, "--verbose", "--no-pull"},
		map[string]string{
			"path": functionPath,
			"file": functionConfigPath,
		})

	// delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "function", functionConfig.Meta.Name}, nil) // nolint: errcheck
	suite.Require().NoError(err, "Function creation expected to pass")

	// try a few times to invoke, until it succeeds
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"invoke", functionConfig.Meta.Name},
		map[string]string{
			"method":       "POST",
			"body":         `{"return_this": "abc"}`,
			"content-type": "application/json",
			"via":          "external-ip",
		}, false)
	suite.Require().NoError(err)

	// --- step 2 ---
	// redeploy the function with a small change, to ensure the resource version is changed

	// get the current deployed function, save its resource version
	deployedFunction, err := suite.getFunctionInFormat(functionConfig.Meta.Name, nuctlcommon.OutputFormatYAML)
	suite.Require().NoError(err)

	// save it for next step, to be used as a "stale" resource vresion
	functionResourceVersion := deployedFunction.Meta.ResourceVersion

	// sanity, ensure it is not an empty string
	suite.Require().NotEmpty(functionResourceVersion)

	// redeploy the function, let it change its resource vresion
	err = suite.ExecuteNuctl([]string{"deploy", functionConfig.Meta.Name, "--verbose", "--no-pull"},
		map[string]string{
			"path":       functionPath,
			"file":       functionConfigPath,
			"target-cpu": "80", // some change
		})
	suite.Require().NoError(err)

	// wait for function to be deployed
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"invoke", functionConfig.Meta.Name},
		map[string]string{
			"method":       "POST",
			"body":         `{"return_this": "abc"}`,
			"content-type": "application/json",
			"via":          "external-ip",
		}, false)
	suite.Require().NoError(err)

	// get the redeployed function, extract its latest resource version
	redeployedFunction, err := suite.getFunctionInFormat(functionConfig.Meta.Name, nuctlcommon.OutputFormatYAML)
	suite.Require().NoError(err)

	// sanity, ensure retrieved redeployed function resource version is not empty
	suite.Require().NotEmpty(redeployedFunction.Meta.ResourceVersion)

	// --- step 3 ---
	// at this stage, deployFunction instance is considered stale, while redeployedFunction instance is the latest copy.
	// at this step, we are going to deploy with the stale resource version and expect a conflict failure

	// change it to anything but empty/equal to the current resource version
	deployedFunction.Meta.ResourceVersion = functionResourceVersion

	// write function config to file
	staleFunctionConfigPath := suite.writeFunctionConfigToTempFile(&deployedFunction.Config,
		"stale-resource-version-*.yaml")

	// remove when done
	defer os.RemoveAll(staleFunctionConfigPath) // nolint: errcheck

	// deployment should fail, resource schema conflict
	err = suite.ExecuteNuctl([]string{"deploy", functionConfig.Meta.Name, "--verbose"},
		map[string]string{
			"file": staleFunctionConfigPath,
		})
	suite.Require().Error(err)
	suite.Require().Equal(http.StatusConflict, errors.Cause(err).(*nuclio.ErrorWithStatusCode).StatusCode())

	// --- step 4 ----
	// at this step, we expect deployment to pass again, as we removed the resource version
	// to allow overriding of the current function regardless of its resource version

	// empty resource version
	deployedFunction.Meta.ResourceVersion = ""

	// write function config to file
	overriddenFunctionConfigPath := suite.writeFunctionConfigToTempFile(&deployedFunction.Config,
		"overridden-resource-version-*.yaml")

	// remove when done
	defer os.RemoveAll(overriddenFunctionConfigPath) // nolint: errcheck

	// deployment should pass, resource version should be overridden
	err = suite.ExecuteNuctl([]string{"deploy", functionConfig.Meta.Name, "--verbose"},
		map[string]string{
			"file": overriddenFunctionConfigPath,
		})
	suite.Require().NoError(err)

	// retry get function until its resource version changes
	err = common.RetryUntilSuccessful(1*time.Minute, 3*time.Second, func() bool {

		// get the deployed function, we're gonna inspect its resource version
		deployedFunction, err = suite.getFunctionInFormat(functionConfig.Meta.Name, nuctlcommon.OutputFormatYAML)
		return err == nil && deployedFunction.Meta.ResourceVersion != functionResourceVersion
	})
	suite.Require().NoErrorf(err, "Resource version should have been changed (real: %s, expected: %s)",
		deployedFunction.Meta.ResourceVersion,
		functionResourceVersion)
}

func (suite *functionDeployTestSuite) TestDeployAndRedeployHTTPTriggerPortChange() {
	uniqueSuffix := "-" + xid.New().String()
	functionName := "port-change" + uniqueSuffix
	imageName := "nuclio/processor-" + functionName

	namedArgs := map[string]string{
		"path":    path.Join(suite.GetFunctionsDir(), "common", "event-recorder", "python"),
		"runtime": "python",
		"handler": "event_recorder:handler",
	}

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"}, namedArgs)
	suite.Require().NoError(err)

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName) // nolint: errcheck

	// wait for function to become ready
	suite.waitForFunctionState(functionName, functionconfig.FunctionStateReady)

	deployedFunctionConfig, err := suite.getFunctionInFormat(functionName, nuctlcommon.OutputFormatYAML)
	suite.Require().NoError(err)

	// ensure allocated http port is returned
	suite.Require().NotZero(deployedFunctionConfig.Status.HTTPPort)

	desiredHTTPPort := 30555
	namedArgs = map[string]string{
		"path":    path.Join(suite.GetFunctionsDir(), "common", "event-recorder", "python"),
		"runtime": "python",
		"handler": "event_recorder:handler",
		"triggers": fmt.Sprintf(`{
	   "http": {
	       "kind": "http",
	       "attributes": {
	           "port": %d
	       }
	   }
	}`, desiredHTTPPort),
	}

	// redeploy function with a specific port
	err = suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"}, namedArgs)
	suite.Require().NoError(err)

	// wait for function to become ready again
	suite.waitForFunctionState(functionName, functionconfig.FunctionStateReady)

	suite.outputBuffer.Reset()

	deployedFunctionConfig, err = suite.getFunctionInFormat(functionName, nuctlcommon.OutputFormatYAML)
	suite.Require().NoError(err)

	suite.Require().Equal(desiredHTTPPort, deployedFunctionConfig.Status.HTTPPort)
}

func (suite *functionDeployTestSuite) TestDeployFailsOnReservedFunctionName() {
	functionName := "dashboard"
	imageName := "nuclio/processor-" + functionName

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName) // nolint: errcheck

	// use nuctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"},
		map[string]string{
			"runtime": "golang",
			"handler": "main:Reverse",
		})
	suite.Require().Error(err, "Deploy should have been failed with precondition error.")
	suite.Require().IsType(&nuclio.ErrPreconditionFailed, errors.RootCause(err))
}

// Expecting the Code Entry Type to be modified to image
func (suite *functionDeployTestSuite) TestDeployFromLocalDirPath() {
	uniqueSuffix := "-" + xid.New().String()
	functionName := "local-dir" + uniqueSuffix
	imageName := "nuclio/processor-" + functionName

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"},
		map[string]string{
			"path":    path.Join(suite.GetFunctionsDir(), "common", "reverser", "python"),
			"runtime": "python:3.8",
			"handler": "reverser:handler",
		})
	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName) // nolint: errcheck

	// use nuctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

	// check that the function's CET was modified to 'image'
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"get", "function", functionName},
		map[string]string{
			"output": nuctlcommon.OutputFormatYAML,
		},
		false)
	suite.Require().NoError(err)
	suite.Require().Contains(suite.outputBuffer.String(), "codeEntryType: image")

	// try a few times to invoke, until it succeeds
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"invoke", functionName},
		map[string]string{
			"method": "POST",
			"body":   "-reverse this string+",
			"via":    "external-ip",
		}, false)
	suite.Require().NoError(err)

	// check that invoke printed the value
	suite.Require().Contains(suite.outputBuffer.String(), "+gnirts siht esrever-")
}

// Expect the deployment to fail fast (instead of waiting for readiness timeout to pass)
func (suite *functionDeployTestSuite) TestDeployWaitReadinessTimeoutBeforeFailureDisabled() {
	uniqueSuffix := "-" + xid.New().String()
	functionName := "reverser" + uniqueSuffix
	imageName := "nuclio/processor-" + functionName

	// set a bad handler name - so that the deployment will fail
	namedArgs := map[string]string{
		"path":              path.Join(suite.GetFunctionsDir(), "common", "reverser", "python"),
		"runtime":           "python",
		"handler":           "reverser:bad-handler-name",
		"readiness-timeout": "60",
	}

	deployingTimestamp := time.Now()

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"}, namedArgs)
	suite.Require().Error(err)

	// validate the deployment "failed fast" - it didn't wait for the whole readiness timeout to pass
	failedFast := time.Now().Before(deployingTimestamp.Add(60 * time.Second))
	suite.Require().True(failedFast)

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName) // nolint: errcheck
}

func (suite *functionDeployTestSuite) TestDeployWithSecurityContext() {

	runAsUserID := "1000"
	runAsGroupID := "2000"
	fsGroup := "3000"

	// with executable handler
	uniqueSuffix := "-" + xid.New().String()
	functionName := "security-context" + uniqueSuffix
	imageName := "nuclio/processor-" + functionName

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"},
		map[string]string{
			"image":        imageName,
			"runtime":      "shell",
			"handler":      "id",
			"run-as-user":  runAsUserID,
			"run-as-group": runAsGroupID,
			"fsgroup":      fsGroup,
		})

	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName) // nolint: errcheck

	// use nuctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

	// try a few times to invoke, until it succeeds
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"invoke", functionName},
		map[string]string{"method": "POST"},
		false)
	suite.Require().NoError(err)

	// make sure the id command from the handler, returns the correct uid and gids
	suite.Require().Contains(suite.outputBuffer.String(), fmt.Sprintf(`uid=%s gid=%s groups=%s`,
		runAsUserID,
		runAsGroupID,
		fsGroup))

	// with script handler
	uniqueSuffix = "-" + xid.New().String()
	functionName = "security-context" + uniqueSuffix
	imageName = "nuclio/processor-" + functionName

	err = suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"},
		map[string]string{
			"image":   imageName,
			"runtime": "shell",
			"handler": "main.sh",

			// the `id` command
			"source":       "aWQ=",
			"run-as-user":  runAsUserID,
			"run-as-group": runAsGroupID,
			"fsgroup":      fsGroup,
		})

	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName) // nolint: errcheck

	// use nuctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

	// try a few times to invoke, until it succeeds
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"invoke", functionName},
		map[string]string{"method": "POST"},
		false)
	suite.Require().NoError(err)

	// make sure the id command from the handler, returns the correct uid and gids
	suite.Require().Contains(suite.outputBuffer.String(), fmt.Sprintf(`uid=%s gid=%s groups=%s`,
		runAsUserID,
		runAsGroupID,
		fsGroup))
}

func (suite *functionDeployTestSuite) TestDeployServiceTypeClusterIPWithInvocation() {

	// TODO: remove this if we ever implement "ServiceType" for local platform
	suite.ensureRunningOnPlatform("kube")

	uniqueSuffix := "-" + xid.New().String()
	functionName := "deploy-reverser" + uniqueSuffix
	imageName := "nuclio/processor-" + functionName
	serviceName := kube.ServiceNameFromFunctionName(functionName)
	url, port := client.GetDomainNameInvokeURL(serviceName, suite.namespace)
	functionClusterURL := fmt.Sprintf("http://%s:%d", url, port)

	namedArgs := map[string]string{
		"path":    path.Join(suite.GetFunctionsDir(), "common", "reverser", "golang"),
		"runtime": "golang",
		"handler": "main:Reverse",
		"triggers": `{
	    "http": {
	        "kind": "http",
	        "attributes": {
	            "serviceType": "ClusterIP"
	        }
	    }
	}`,
	}

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"}, namedArgs)

	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName) // nolint: errcheck

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

	wgetFunctionName := "wget-function" + uniqueSuffix
	wgetImageName := "nuclio/processor-" + wgetFunctionName

	// wgets the url given in the `x-nuclio-arguments` with the POST body from the body
	wgetSourceCode := `url=$1
read body

wget -O - --post-data "$body" $url 2> /dev/null
`

	err = suite.ExecuteNuctl([]string{"deploy", wgetFunctionName, "--verbose", "--no-pull"},
		map[string]string{
			"image":   wgetImageName,
			"runtime": "shell",
			"handler": "main.sh",
			"source":  base64.StdEncoding.EncodeToString([]byte(wgetSourceCode)),
		})

	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(wgetImageName) // nolint: errcheck

	// use nuctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", wgetFunctionName}, nil) // nolint: errcheck

	// try a few times to invoke, until it succeeds
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"invoke", wgetFunctionName},
		map[string]string{
			"method":  "POST",
			"headers": fmt.Sprintf("x-nuclio-arguments=%s", functionClusterURL),
			"body":    "-reverse this string+",
			"via":     "external-ip",
		},
		false)
	suite.Require().NoError(err)

	// make sure reverser worked
	suite.Require().Contains(suite.outputBuffer.String(), "+gnirts siht esrever-")
}

func (suite *functionDeployTestSuite) TestDeployWithOverrideServiceTypeFlag() {

	// TODO: remove this if we ever implement "ServiceType" for local platform
	suite.ensureRunningOnPlatform("kube")

	uniqueSuffix := "-" + xid.New().String()
	functionName := "reverser-cluster-ip" + uniqueSuffix
	imageName := "nuclio/processor-" + functionName

	namedArgs := map[string]string{
		"path":                      path.Join(suite.GetFunctionsDir(), "common", "reverser", "golang"),
		"runtime":                   "golang",
		"handler":                   "main:Reverse",
		"http-trigger-service-type": "ClusterIP",
	}

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"}, namedArgs)

	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName) // nolint: errcheck

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

	functionName2 := "reverser-node-port" + uniqueSuffix
	imageName2 := "nuclio/processor-" + functionName2

	namedArgs = map[string]string{
		"path":                      path.Join(suite.GetFunctionsDir(), "common", "reverser", "golang"),
		"runtime":                   "golang",
		"handler":                   "main:Reverse",
		"http-trigger-service-type": "NodePort",
	}

	err = suite.ExecuteNuctl([]string{"deploy", functionName2, "--verbose", "--no-pull"}, namedArgs)

	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName2) // nolint: errcheck

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName2}, nil) // nolint: errcheck

	// get the function
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"get", "fu", functionName}, map[string]string{
		"output": nuctlcommon.OutputFormatYAML,
	}, false)
	suite.Require().NoError(err)

	functionConfig := functionconfig.Config{}
	err = yaml.Unmarshal(suite.outputBuffer.Bytes(), &functionConfig)
	suite.Require().NoError(err)

	suite.Assert().Equal("ClusterIP", functionConfig.Spec.Triggers["default-http"].Attributes["serviceType"])

	// get the function
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"get", "fu", functionName2}, map[string]string{
		"output": nuctlcommon.OutputFormatYAML,
	}, false)
	suite.Require().NoError(err)

	function2Config := functionconfig.Config{}
	err = yaml.Unmarshal(suite.outputBuffer.Bytes(), &function2Config)
	suite.Require().NoError(err)

	suite.Assert().Equal("NodePort", function2Config.Spec.Triggers["default-http"].Attributes["serviceType"])
}

type functionGetTestSuite struct {
	Suite
}

func (suite *functionGetTestSuite) TestGet() {
	var err error
	numOfFunctions := 3
	var functionNames []string

	for functionIdx := 0; functionIdx < numOfFunctions; functionIdx++ {
		uniqueSuffix := fmt.Sprintf("-%s-%d", xid.New().String(), functionIdx)
		functionName := "reverser" + uniqueSuffix
		imageName := "nuclio/processor-" + functionName

		// add function name to list
		functionNames = append(functionNames, functionName)

		namedArgs := map[string]string{
			"path":    path.Join(suite.GetFunctionsDir(), "common", "reverser", "golang"),
			"runtime": "golang",
			"handler": "main:Reverse",
		}

		// cleanup
		defer func(imageName string, functionName string) {

			// make sure to clean up after the test
			suite.dockerClient.RemoveImage(imageName) // nolint: errcheck

			// use nutctl to delete the function when we're done
			suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck
		}(imageName, functionName)

		err := suite.ExecuteNuctl([]string{
			"deploy",
			functionName,
			"--verbose",
			"--no-pull",
		}, namedArgs)

		suite.Require().NoError(err)

		// wait for function to ensure deployed successfully
		err = suite.RetryExecuteNuctlUntilSuccessful([]string{"get", "function", functionName}, nil, false)
		suite.Require().NoError(err)
	}

	// get deployed functions
	err = suite.ExecuteNuctl([]string{"get", "function"}, nil)
	suite.Require().NoError(err)

	// find function names in get result
	suite.findPatternsInOutput(functionNames, nil)

	for _, testCase := range []struct {
		FunctionName string
		OutputFormat string
	}{
		{
			FunctionName: functionNames[0],
			OutputFormat: nuctlcommon.OutputFormatJSON,
		},
		{
			FunctionName: functionNames[0],
			OutputFormat: nuctlcommon.OutputFormatYAML,
		},
	} {
		// reset buffer
		suite.outputBuffer.Reset()

		parsedFunction, err := suite.getFunctionInFormat(testCase.FunctionName, testCase.OutputFormat)

		// ensure parsing went well, and response is valid (json/yaml)
		suite.Require().NoError(err, "Failed to unmarshal function")

		// sanity, we got the function we asked for
		suite.Assert().Equal(testCase.FunctionName, parsedFunction.Meta.Name)
	}
}

type functionDeleteTestSuite struct {
	Suite
}

func (suite *functionGetTestSuite) TestDelete() {
	var err error

	uniqueSuffix := "-" + xid.New().String()
	functionName := "reverser" + uniqueSuffix
	imageName := "nuclio/processor-" + functionName

	namedArgs := map[string]string{
		"path":    path.Join(suite.GetFunctionsDir(), "common", "reverser", "golang"),
		"runtime": "golang",
		"handler": "main:Reverse",
	}

	err = suite.ExecuteNuctl([]string{
		"deploy",
		functionName,
		"--verbose",
		"--no-pull",
	}, namedArgs)
	suite.Require().NoError(err)

	// cleanup
	defer suite.dockerClient.RemoveImage(imageName) // nolint: errcheck

	// try a few times to invoke, until it succeeds
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"invoke", functionName},
		map[string]string{
			"method": "POST",
			"body":   "-reverse this string+",
			"via":    "external-ip",
		},
		false)
	suite.Require().NoError(err)

	// function removed
	err = suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)
	suite.Require().NoError(err)

	// ensure delete is idempotent
	err = suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)
	suite.Require().NoError(err)

	// try invoke, it should failed
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"invoke", functionName},
		map[string]string{
			"method": "POST",
			"body":   "-reverse this string+",
			"via":    "external-ip",
		},
		true)
	suite.Require().NoError(err, "Function was suppose to be deleted!")
}

type functionExportImportTestSuite struct {
	Suite
}

func (suite *functionExportImportTestSuite) TestFailToImportFunctionNoInput() {

	// import function without input
	err := suite.ExecuteNuctl([]string{"import", "fu", "--verbose"}, nil)
	suite.Require().Error(err)

}

func (suite *functionExportImportTestSuite) TestImportMultiFunctions() {
	functionsConfigPath := path.Join(suite.GetImportsDir(), "functions.yaml")

	// these names are defined within functions.yaml
	function1Name := "test-function-1"
	function2Name := "test-function-2"

	defer suite.ExecuteNuctl([]string{"delete", "fu", function1Name}, nil) // nolint: errcheck
	defer suite.ExecuteNuctl([]string{"delete", "fu", function2Name}, nil) // nolint: errcheck

	// import the project
	err := suite.ExecuteNuctl([]string{"import", "fu", functionsConfigPath, "--verbose"}, nil)
	suite.Require().NoError(err)

	suite.assertFunctionImported(function1Name, true)
	suite.assertFunctionImported(function2Name, true)
}

func (suite *functionExportImportTestSuite) TestImportFunction() {
	functionConfigPath := path.Join(suite.GetImportsDir(), "function.yaml")

	// this name is defined within function.yaml
	functionName := "test-function"

	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

	// import the project
	err := suite.ExecuteNuctl([]string{"import", "fu", functionConfigPath, "--verbose"}, nil) // nolint: errcheck
	suite.Require().NoError(err)

	suite.assertFunctionImported(functionName, true)
}

func (suite *functionExportImportTestSuite) TestExportImportRoundTripFromStdin() {
	uniqueSuffix := "-" + xid.New().String()
	functionName := "export-import-stdin" + uniqueSuffix
	imageName := "nuclio/processor-" + functionName
	namedArgs := map[string]string{
		"path":    path.Join(suite.GetFunctionsDir(), "common", "reverser", "python"),
		"runtime": "python:3.6",
		"handler": "reverser:handler",
	}

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"}, namedArgs)
	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName) // nolint: errcheck

	// use nuctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

	// reset output buffer for reading the next output cleanly
	suite.outputBuffer.Reset()
	suite.inputBuffer.Reset()

	// export the function
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"export", "fu", functionName}, nil, false)
	suite.Require().NoError(err)

	exportedFunctionBody := suite.outputBuffer.Bytes()

	// delete original function in order to resolve conflict while importing the function
	err = suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)
	suite.Require().NoError(err)

	// wait until function is deleted
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"get", "function", functionName}, nil, true)
	suite.Require().NoError(err)

	// import the function from stdin
	suite.inputBuffer.Write(exportedFunctionBody)
	err = suite.ExecuteNuctl([]string{"import", "fu"}, nil)
	suite.Require().NoError(err)

	// wait until able to get the function
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"get", "function", functionName}, nil, false)
	suite.Require().NoError(err)
	suite.Require().Contains(suite.outputBuffer.String(), "imported")

	// try to invoke, and ensure it fails - because it is imported and not deployed
	err = suite.ExecuteNuctl([]string{"invoke", functionName},
		map[string]string{
			"method": "POST",
			"body":   "-reverse this string+",
			"via":    "external-ip",
		})
	suite.Require().Error(err)

	// deploy imported function
	err = suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose"}, nil)
	suite.Require().NoError(err)

	// try a few times to invoke, until it succeeds
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"invoke", functionName},
		map[string]string{
			"method": "POST",
			"body":   "-reverse this string+",
			"via":    "external-ip",
		},
		false)
	suite.Require().NoError(err)

	// make sure reverser worked
	suite.Require().Contains(suite.outputBuffer.String(), "+gnirts siht esrever-")
}

func (suite *functionExportImportTestSuite) TestExportImportRoundTrip() {
	uniqueSuffix := "-" + xid.New().String()
	functionName := "reverser" + uniqueSuffix
	imageName := "nuclio/processor-" + functionName

	namedArgs := map[string]string{
		"path":    path.Join(suite.GetFunctionsDir(), "common", "reverser", "golang"),
		"runtime": "golang",
		"handler": "main:Reverse",
	}

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"}, namedArgs)
	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName) // nolint: errcheck

	// reset output buffer for reading the nex output cleanly
	suite.outputBuffer.Reset()

	// export the function
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"export", "fu", functionName}, nil, false)
	suite.Require().NoError(err)

	exportedFunctionConfig := functionconfig.Config{}
	err = yaml.Unmarshal(suite.outputBuffer.Bytes(), &exportedFunctionConfig)
	suite.Require().NoError(err)

	// assert skip annotations
	suite.Assert().True(functionconfig.ShouldSkipBuild(exportedFunctionConfig.Meta.Annotations))
	suite.Assert().True(functionconfig.ShouldSkipDeploy(exportedFunctionConfig.Meta.Annotations))

	exportedFunctionConfigJSON, err := json.Marshal(exportedFunctionConfig)
	suite.Require().NoError(err)

	// write exported function config to temp file
	exportTempFile, err := ioutil.TempFile("", "reverser.*.json")
	suite.Require().NoError(err)
	defer os.Remove(exportTempFile.Name()) // nolint: errcheck

	_, err = exportTempFile.Write(exportedFunctionConfigJSON)
	suite.Require().NoError(err)

	// delete original function in order to resolve conflict while importing the function
	err = suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)
	suite.Require().NoError(err)

	// wait until function is deleted
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"get", "function", functionName}, nil, true)
	suite.Require().NoError(err)

	// import the function
	err = suite.ExecuteNuctl([]string{"import", "fu", exportTempFile.Name()}, nil)
	suite.Require().NoError(err)

	// use nuctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

	// wait until able to get the function
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"get", "function", functionName}, nil, false)
	suite.Require().NoError(err)

	// try to invoke, and ensure it fails - because it is imported and not deployed
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"invoke", functionName},
		map[string]string{
			"method": "POST",
			"body":   "-reverse this string+",
			"via":    "external-ip",
		},
		true)
	suite.Require().NoError(err)

	// deploy imported function
	err = suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose"}, nil)
	suite.Require().NoError(err)

	// try a few times to invoke, until it succeeds
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"invoke", functionName},
		map[string]string{
			"method": "POST",
			"body":   "-reverse this string+",
			"via":    "external-ip",
		},
		false)
	suite.Require().NoError(err)

	// make sure reverser worked
	suite.Require().Contains(suite.outputBuffer.String(), "+gnirts siht esrever-")
}

func (suite *functionExportImportTestSuite) TestExportImportRoundTripFailingFunction() {
	uniqueSuffix := "-" + xid.New().String()
	functionName := "export-import-failing-function" + uniqueSuffix
	imageName := "nuclio/processor-" + functionName

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName) // nolint: errcheck

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"},
		map[string]string{
			"file":            path.Join(suite.GetFunctionConfigsDir(), "error", "s3_codeentry/function.yaml"),
			"code-entry-type": "s3",
			"runtime":         "golang",
			"handler":         "main:Reverse",
		})
	suite.Require().Error(err)

	// reset output buffer for reading the nex output cleanly
	suite.outputBuffer.Reset()

	// export the function
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"export", "fu", functionName}, nil, false)
	suite.Require().NoError(err)

	exportedFunctionConfig := functionconfig.Config{}
	err = yaml.Unmarshal(suite.outputBuffer.Bytes(), &exportedFunctionConfig)
	suite.Require().NoError(err)

	// assert skip annotations
	suite.Assert().True(functionconfig.ShouldSkipBuild(exportedFunctionConfig.Meta.Annotations))
	suite.Assert().True(functionconfig.ShouldSkipDeploy(exportedFunctionConfig.Meta.Annotations))

	exportedFunctionConfigJSON, err := json.Marshal(exportedFunctionConfig)
	suite.Require().NoError(err)

	// write exported function config to temp file
	exportTempFile, err := ioutil.TempFile("", "reverser.*.json")
	suite.Require().NoError(err)
	defer os.Remove(exportTempFile.Name()) // nolint: errcheck

	_, err = exportTempFile.Write(exportedFunctionConfigJSON)
	suite.Require().NoError(err)

	// delete original function in order to resolve conflict while importing the function
	err = suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)
	suite.Require().NoError(err)

	// wait until function is deleted
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"get", "function", functionName}, nil, true)
	suite.Require().NoError(err)

	// import the function
	err = suite.ExecuteNuctl([]string{"import", "fu", exportTempFile.Name()}, nil)
	suite.Require().NoError(err)

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

	// wait until able to get the function
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"get", "function", functionName}, nil, false)
	suite.Require().NoError(err)

	// try to invoke, and ensure it fails - because it is imported and not deployed
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"invoke", functionName},
		map[string]string{
			"method": "POST",
			"body":   "-reverse this string+",
			"via":    "external-ip",
		},
		true)
	suite.Require().NoError(err)

	// deploy imported function
	err = suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose"}, nil)

	suite.Require().Error(err, "Function code must be provided either in the path or inline in a spec file; alternatively, an image or handler may be provided")
}

func TestFunctionTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(functionBuildTestSuite))
	suite.Run(t, new(functionDeployTestSuite))
	suite.Run(t, new(functionGetTestSuite))
	suite.Run(t, new(functionDeleteTestSuite))
	suite.Run(t, new(functionExportImportTestSuite))
}
