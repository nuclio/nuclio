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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/nuctl/command/common"
	"github.com/nuclio/nuclio/pkg/processor/build"
	"github.com/nuclio/nuclio/pkg/processor/trigger/test"

	"github.com/ghodss/yaml"
	"github.com/nuclio/errors"
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
	defer suite.dockerClient.RemoveImage(imageName)

	// use deploy with the image we just created
	err = suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose"},
		map[string]string{
			"run-image": imageName,
			"runtime":   "golang",
			"handler":   "main:Reverse",
		})

	suite.Require().NoError(err)

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)

	// try a few times to invoke, until it succeeds
	err = suite.ExecuteNuctlAndWait([]string{"invoke", functionName},
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
	uniqueSuffix := "-" + xid.New().String()
	functionName := "deploy-reverser" + uniqueSuffix
	imageName := "nuclio/processor-" + functionName

	namedArgs := map[string]string{
		"path":    path.Join(suite.GetFunctionsDir(), "common", "reverser", "golang"),
		"runtime": "golang",
		"handler": "main:Reverse",
	}

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"}, namedArgs)

	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName)

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)

	// try a few times to invoke, until it succeeds
	err = suite.ExecuteNuctlAndWait([]string{"invoke", functionName},
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
	defer suite.dockerClient.RemoveImage(imageName)

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)

	suite.inputBuffer = bytes.Buffer{}
	suite.inputBuffer.WriteString("-reverse this string+")

	// try a few times to invoke, until it succeeds
	err = suite.ExecuteNuctlAndWait([]string{"invoke", functionName},
		map[string]string{
			"method": "POST",
			"via":    "external-ip",
		},
		false)
	suite.Require().NoError(err)

	// make sure reverser worked
	suite.Require().Contains(suite.outputBuffer.String(), "+gnirts siht esrever-")
}

func (suite *functionDeployTestSuite) TestDeployWithMetadata() {
	uniqueSuffix := "-" + xid.New().String()
	functionName := "envprinter" + uniqueSuffix
	imageName := "nuclio/processor-" + functionName

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"},
		map[string]string{
			"path":    path.Join(suite.GetFunctionsDir(), "common", "envprinter", "python"),
			"env":     "FIRST_ENV=11223344,SECOND_ENV=0099887766",
			"labels":  "label1=first,label2=second",
			"runtime": "python",
			"handler": "envprinter:handler",
		})

	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName)

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)

	// try a few times to invoke, until it succeeds
	err = suite.ExecuteNuctlAndWait([]string{"invoke", functionName},
		map[string]string{
			"method": "POST",
			"body":   "-reverse this string+",
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
	defer suite.dockerClient.RemoveImage(imageName)

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)

	// try a few times to invoke, until it succeeds
	err = suite.ExecuteNuctlAndWait([]string{"invoke", functionName},
		map[string]string{
			"method": "POST",
			"body":   fmt.Sprintf(`{"return_this": "%s"}`, randomString),
			"via":    "external-ip",
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
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName)

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
	defer suite.dockerClient.RemoveImage(imageName)

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)

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
		err = suite.ExecuteNuctlAndWait([]string{"invoke", functionName},
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
	defer suite.dockerClient.RemoveImage(imageName)

	// use nuctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)

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
	defer suite.dockerClient.RemoveImage(imageName)

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)

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
	defer suite.dockerClient.RemoveImage(imageName)

	// use nuctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)

	// try a few times to invoke, until it succeeds
	err = suite.ExecuteNuctlAndWait([]string{"invoke", functionName},
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
	err = suite.ExecuteNuctlAndWait([]string{"get", "function", functionName}, nil, false)
	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName)

	// create a function event using nuctl
	err = suite.ExecuteNuctl([]string{"create", "functionevent", functionEventName},
		map[string]string{
			"function": functionName,
		})
	suite.Require().NoError(err)

	// check to see we have created the function event
	err = suite.ExecuteNuctlAndWait([]string{"get", "functionevent", functionEventName}, nil, false)
	suite.Require().NoError(err)

	// delete the function
	err = suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)
	suite.Require().NoError(err)

	// ensure function created
	err = suite.ExecuteNuctlAndWait([]string{"get", "function", functionName}, nil, true)
	suite.Require().NoError(err)

	// check to see the function event was deleted as well
	err = suite.ExecuteNuctlAndWait([]string{"get", "functionevent", functionEventName}, nil, true)
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
	suite.dockerClient.RemoveImage(imageName)

	//  make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName)
	defer suite.shellClient.Run(nil, "rm %s", tarName)

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
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)

	// try a few times to invoke, until it succeeds
	err = suite.ExecuteNuctlAndWait([]string{"invoke", functionName},
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

// Expecting the Code Entry Type to be modified to image
func (suite *functionDeployTestSuite) TestDeployFromLocalDirPath() {
	uniqueSuffix := "-" + xid.New().String()
	functionName := "local-dir" + uniqueSuffix
	imageName := "nuclio/processor-" + functionName

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"},
		map[string]string{
			"path":    path.Join(suite.GetFunctionsDir(), "common", "reverser", "python"),
			"runtime": "python:3.6",
			"handler": "reverser:handler",
		})
	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName)

	// use nuctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)

	// check that the function's CET was modified to 'image'
	err = suite.ExecuteNuctlAndWait([]string{"get", "function", functionName},
		map[string]string{
			"output": common.OutputFormatYAML,
		},
		false)
	suite.Require().NoError(err)
	suite.Require().Contains(suite.outputBuffer.String(), "codeEntryType: image")

	// try a few times to invoke, until it succeeds
	err = suite.ExecuteNuctlAndWait([]string{"invoke", functionName},
		map[string]string{
			"method": "POST",
			"body":   "-reverse this string+",
			"via":    "external-ip",
		}, false)
	suite.Require().NoError(err)

	// check that invoke printed the value
	suite.Require().Contains(suite.outputBuffer.String(), "+gnirts siht esrever-")
}

func (suite *functionDeployTestSuite) TestDeployCronTriggersK8s() {

	// relevant only for kube platform
	if suite.origPlatformType != "kube" {
		suite.T().Skipf("Not on kube platform")
	}

	uniqueSuffix := "-" + xid.New().String()
	functionName := "event-recorder" + uniqueSuffix
	imageName := "nuclio/processor-" + functionName

	namedArgs := map[string]string{
		"path":    path.Join(suite.GetFunctionsDir(), "common", "event-recorder", "python"),
		"runtime": "python",
		"handler": "event_recorder:handler",
		"triggers": `{
    "crontrig": {
        "kind": "cron",
        "attributes": {
            "interval": "3s",
            "event": {
                "body": "somebody",
                "headers": {
                    "Extra-Header-1": "value1",
                    "Extra-Header-2": "value2"
                }
            }
        }
    }
}`,
	}

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"}, namedArgs)
	suite.Require().NoError(err)

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName)

	// try a few times to invoke, until it succeeds (validate function deployment finished)
	err = suite.ExecuteNuctlAndWait([]string{"invoke", functionName},
		map[string]string{
			"method": "POST",
			"via":    "external-ip",
		},
		false)
	suite.Require().NoError(err)

	// wait 15 seconds so at least 1 interval will pass
	suite.logger.InfoWith("Sleeping for 15 sec (so at least 1 interval will pass)")
	time.Sleep(15 * time.Second)
	suite.logger.InfoWith("Done sleeping")

	suite.outputBuffer.Reset()

	// try a few times to invoke, until it succeeds
	// the output buffer should contain a response body with the function's called events from the cron trigger
	err = suite.ExecuteNuctlAndWait([]string{"invoke", functionName},
		map[string]string{
			"method": "POST",
			"via":    "external-ip",
		},
		false)
	suite.Require().NoError(err)

	events := suite.parseEventsRecorderOutput(suite.outputBuffer.String())

	// validate at least 1 cron job ran
	suite.Require().GreaterOrEqual(len(events), 1)

	// validate the body was sent
	suite.Require().Equal(events[0].Body, "somebody")

	// validate headers were attached properly
	suite.Require().Contains(events[0].Headers, "X-Nuclio-Invoke-Trigger")
	suite.Require().Contains(events[0].Headers, "Extra-Header-1")
	suite.Require().Contains(events[0].Headers, "Extra-Header-2")
}

func (suite *functionDeployTestSuite) parseEventsRecorderOutput(outputBufferString string) []triggertest.Event {
	var foundResponseBody bool
	var responseBody string
	var events []triggertest.Event

	suite.logger.InfoWith("Parsing event recorder output", "outputBufferString", outputBufferString)

	// try unquote response from output buffer (continue normally if it's not a quoted string)
	response, err := strconv.Unquote(outputBufferString)
	if err != nil {
		response = outputBufferString
	}

	// find the response body in the output buffer
	responseLines := strings.Split(response, "\n")
	for _, line := range responseLines {
		if foundResponseBody {
			responseBody = line
			break
		}
		if strings.Contains(line, "Response body") {
			foundResponseBody = true
			continue
		}
	}

	suite.logger.InfoWith("Parsing events from response body", "responseBody", responseBody)
	err = json.Unmarshal([]byte(responseBody), &events)
	suite.Require().NoError(err)

	return events
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
			suite.dockerClient.RemoveImage(imageName)

			// use nutctl to delete the function when we're done
			suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)
		}(imageName, functionName)

		err := suite.ExecuteNuctl([]string{
			"deploy",
			functionName,
			"--verbose",
			"--no-pull",
		}, namedArgs)

		suite.Require().NoError(err)

		// wait for function to ensure deployed successfully
		err = suite.ExecuteNuctlAndWait([]string{"get", "function", functionName}, nil, false)
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
			OutputFormat: common.OutputFormatJSON,
		},
		{
			FunctionName: functionNames[0],
			OutputFormat: common.OutputFormatYAML,
		},
	} {
		// reset buffer
		suite.outputBuffer.Reset()

		parsedFunction := functionconfig.Config{}

		// get function in format
		err = suite.ExecuteNuctl([]string{"get", "function", testCase.FunctionName},
			map[string]string{
				"output": testCase.OutputFormat,
			})
		suite.Require().NoError(err)

		// unmarshal response correspondingly to output format
		switch testCase.OutputFormat {
		case common.OutputFormatJSON:
			err = json.Unmarshal(suite.outputBuffer.Bytes(), &parsedFunction)
		case common.OutputFormatYAML:
			err = yaml.Unmarshal(suite.outputBuffer.Bytes(), &parsedFunction)
		}

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
	defer suite.dockerClient.RemoveImage(imageName)

	// try a few times to invoke, until it succeeds
	err = suite.ExecuteNuctlAndWait([]string{"invoke", functionName},
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
	err = suite.ExecuteNuctlAndWait([]string{"invoke", functionName},
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

	defer suite.ExecuteNuctl([]string{"delete", "fu", function1Name}, nil)
	defer suite.ExecuteNuctl([]string{"delete", "fu", function2Name}, nil)

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

	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)

	// import the project
	err := suite.ExecuteNuctl([]string{"import", "fu", functionConfigPath, "--verbose"}, nil)
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
	defer suite.dockerClient.RemoveImage(imageName)

	// use nuctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)

	// reset output buffer for reading the next output cleanly
	suite.outputBuffer.Reset()
	suite.inputBuffer.Reset()

	// export the function
	err = suite.ExecuteNuctlAndWait([]string{"export", "fu", functionName}, nil, false)
	suite.Require().NoError(err)

	exportedFunctionBody := suite.outputBuffer.Bytes()

	// delete original function in order to resolve conflict while importing the function
	err = suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)
	suite.Require().NoError(err)

	// wait until function is deleted
	err = suite.ExecuteNuctlAndWait([]string{"get", "function", functionName}, nil, true)
	suite.Require().NoError(err)

	// import the function from stdin
	suite.inputBuffer.Write(exportedFunctionBody)
	err = suite.ExecuteNuctl([]string{"import", "fu"}, nil)
	suite.Require().NoError(err)

	// wait until able to get the function
	err = suite.ExecuteNuctlAndWait([]string{"get", "function", functionName}, nil, false)
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
	err = suite.ExecuteNuctlAndWait([]string{"invoke", functionName},
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
	defer suite.dockerClient.RemoveImage(imageName)

	// reset output buffer for reading the nex output cleanly
	suite.outputBuffer.Reset()

	// export the function
	err = suite.ExecuteNuctlAndWait([]string{"export", "fu", functionName}, nil, false)
	suite.Require().NoError(err)

	exportedFunctionConfig := functionconfig.Config{}
	err = yaml.Unmarshal(suite.outputBuffer.Bytes(), &exportedFunctionConfig)
	suite.Require().NoError(err)

	// assert skip annotations
	suite.Assert().True(functionconfig.ShouldSkipBuild(exportedFunctionConfig.Meta.Annotations))
	suite.Assert().True(functionconfig.ShouldSkipDeploy(exportedFunctionConfig.Meta.Annotations))

	exportedFunctionConfigJson, err := json.Marshal(exportedFunctionConfig)
	suite.Require().NoError(err)

	// write exported function config to temp file
	exportTempFile, err := ioutil.TempFile("", "reverser.*.json")
	suite.Require().NoError(err)
	defer os.Remove(exportTempFile.Name())

	_, err = exportTempFile.Write(exportedFunctionConfigJson)
	suite.Require().NoError(err)

	// delete original function in order to resolve conflict while importing the function
	err = suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)
	suite.Require().NoError(err)

	// wait until function is deleted
	err = suite.ExecuteNuctlAndWait([]string{"get", "function", functionName}, nil, true)
	suite.Require().NoError(err)

	// import the function
	err = suite.ExecuteNuctl([]string{"import", "fu", exportTempFile.Name()}, nil)
	suite.Require().NoError(err)

	// use nuctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)

	// wait until able to get the function
	err = suite.ExecuteNuctlAndWait([]string{"get", "function", functionName}, nil, false)
	suite.Require().NoError(err)

	// try to invoke, and ensure it fails - because it is imported and not deployed
	err = suite.ExecuteNuctlAndWait([]string{"invoke", functionName},
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
	err = suite.ExecuteNuctlAndWait([]string{"invoke", functionName},
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
	defer suite.dockerClient.RemoveImage(imageName)

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
	err = suite.ExecuteNuctlAndWait([]string{"export", "fu", functionName}, nil, false)
	suite.Require().NoError(err)

	exportedFunctionConfig := functionconfig.Config{}
	err = yaml.Unmarshal(suite.outputBuffer.Bytes(), &exportedFunctionConfig)
	suite.Require().NoError(err)

	// assert skip annotations
	suite.Assert().True(functionconfig.ShouldSkipBuild(exportedFunctionConfig.Meta.Annotations))
	suite.Assert().True(functionconfig.ShouldSkipDeploy(exportedFunctionConfig.Meta.Annotations))

	exportedFunctionConfigJson, err := json.Marshal(exportedFunctionConfig)
	suite.Require().NoError(err)

	// write exported function config to temp file
	exportTempFile, err := ioutil.TempFile("", "reverser.*.json")
	suite.Require().NoError(err)
	defer os.Remove(exportTempFile.Name())

	_, err = exportTempFile.Write(exportedFunctionConfigJson)
	suite.Require().NoError(err)

	// delete original function in order to resolve conflict while importing the function
	err = suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)
	suite.Require().NoError(err)

	// wait until function is deleted
	err = suite.ExecuteNuctlAndWait([]string{"get", "function", functionName}, nil, true)
	suite.Require().NoError(err)

	// import the function
	err = suite.ExecuteNuctl([]string{"import", "fu", exportTempFile.Name()}, nil)
	suite.Require().NoError(err)

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)

	// wait until able to get the function
	err = suite.ExecuteNuctlAndWait([]string{"get", "function", functionName}, nil, false)
	suite.Require().NoError(err)

	// try to invoke, and ensure it fails - because it is imported and not deployed
	err = suite.ExecuteNuctlAndWait([]string{"invoke", functionName},
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
