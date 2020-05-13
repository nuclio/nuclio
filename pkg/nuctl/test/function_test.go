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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/nuctl/command/common"

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
	functionName := "reverser" + uniqueSuffix
	imageName := "nuclio/build-test" + uniqueSuffix

	err := suite.ExecuteNuctl([]string{"build", functionName, "--verbose", "--no-pull"},
		map[string]string{
			"path":    path.Join(suite.GetFunctionsDir(), "common", "reverser", "golang"),
			"image":   imageName,
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
	functionName := "reverser" + uniqueSuffix
	imageName := "nuclio/deploy-test" + uniqueSuffix

	namedArgs := map[string]string{
		"path":    path.Join(suite.GetFunctionsDir(), "common", "reverser", "golang"),
		"image":   imageName,
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

func (suite *functionDeployTestSuite) TestDeployWithMetadata() {
	uniqueSuffix := "-" + xid.New().String()
	functionName := "envprinter" + uniqueSuffix
	imageName := "nuclio/deploy-test" + uniqueSuffix

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"},
		map[string]string{
			"path":    path.Join(suite.GetFunctionsDir(), "common", "envprinter", "python"),
			"env":     "FIRST_ENV=11223344,SECOND_ENV=0099887766",
			"labels":  "label1=first,label2=second",
			"runtime": "python",
			"handler": "envprinter:handler",
			"image":   imageName,
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
	uniqueSuffix := "-" + randomString
	imageName := "nuclio/deploy-test" + uniqueSuffix
	functionName := "parser"

	err := suite.ExecuteNuctl([]string{"deploy", "", "--verbose", "--no-pull"},
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

	// deploy function with invalid s3 values
	err := suite.ExecuteNuctl([]string{"deploy", "s3-fast-failure", "--verbose", "--no-pull"},
		map[string]string{
			"file":            path.Join(suite.GetFunctionConfigsDir(), "error", "s3_codeentry/function.yaml"),
			"code-entry-type": "s3",
		})
	suite.Require().Error(err)
	suite.Contains(errors.GetErrorStackString(err, 5), "Failed to download file from s3")
}

func (suite *functionDeployTestSuite) TestInvokeWithLogging() {
	uniqueSuffix := "-" + xid.New().String()
	functionName := "reverser" + uniqueSuffix
	imageName := "nuclio/deploy-test" + uniqueSuffix

	namedArgs := map[string]string{
		"path":    path.Join(suite.GetFunctionsDir(), "common", "logging", "golang"),
		"image":   imageName,
		"runtime": "golang",
		"handler": "main:Logging",
	}

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"}, namedArgs)
	suite.Require().NoError(err)

	time.Sleep(2 * time.Second)

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
		err = suite.ExecuteNuctl([]string{"invoke", functionName},
			map[string]string{
				"method":    "POST",
				"log-level": testCase.logLevel,
				"via":       "external-ip",
			})

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
	functionName := "reverser" + uniqueSuffix
	imageName := "nuclio/deploy-test" + uniqueSuffix

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName)

	// use nuctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"},
		map[string]string{
			"image":   imageName,
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
	functionName := "reverser" + uniqueSuffix
	imageName := "nuclio/deploy-test" + uniqueSuffix

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"},
		map[string]string{
			"image":   imageName,
			"runtime": "shell",
		})

	suite.Require().Error(err, "Function code must be provided either in the path or inline in a spec file; alternatively, an image or handler may be provided")
}

func (suite *functionDeployTestSuite) TestDeployShellViaHandler() {
	uniqueSuffix := "-" + xid.New().String()
	functionName := "reverser" + uniqueSuffix
	imageName := "nuclio/deploy-test" + uniqueSuffix

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
	functionName := "reverser" + uniqueSuffix
	functionEventName := "reverser-event" + uniqueSuffix
	imageName := "nuclio/deploy-test" + uniqueSuffix

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
	randomString := xid.New().String()
	uniqueSuffix := "-" + randomString
	imageName := "nuclio/deploy-local-dir" + uniqueSuffix
	functionName := "local-dir-reverser"

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"},
		map[string]string{
			"path":    path.Join(suite.GetFunctionsDir(), "common", "reverser", "python"),
			"runtime": "python:3.6",
			"handler": "reverser:handler",
			"image":   imageName,
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

type functionGetTestSuite struct {
	Suite
}

func (suite *functionGetTestSuite) TestGet() {
	var err error
	numOfFunctions := 3
	var functionNames []string

	for functionIdx := 0; functionIdx < numOfFunctions; functionIdx++ {
		uniqueSuffix := fmt.Sprintf("-%s-%d", xid.New().String(), functionIdx)

		imageName := "nuclio/deploy-test" + uniqueSuffix
		functionName := "reverser" + uniqueSuffix

		// add function name to list
		functionNames = append(functionNames, functionName)

		namedArgs := map[string]string{
			"path":    path.Join(suite.GetFunctionsDir(), "common", "reverser", "golang"),
			"image":   imageName,
			"runtime": "golang",
			"handler": "main:Reverse",
		}

		err := suite.ExecuteNuctl([]string{
			"deploy",
			functionName,
			"--verbose",
			"--no-pull",
		}, namedArgs)

		suite.Require().NoError(err)

		// cleanup
		defer func(imageName string, functionName string) {

			// make sure to clean up after the test
			suite.dockerClient.RemoveImage(imageName)

			// use nutctl to delete the function when we're done
			suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)
		}(imageName, functionName)

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

	uniqueSuffix := xid.New().String()
	imageName := "nuclio/deploy-test" + uniqueSuffix
	functionName := "reverser" + uniqueSuffix

	namedArgs := map[string]string{
		"path":    path.Join(suite.GetFunctionsDir(), "common", "reverser", "golang"),
		"image":   imageName,
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
	err = suite.ExecuteNuctlAndWait([]string{"export", "fu", functionName}, map[string]string{}, false)
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
	err = suite.ExecuteNuctlAndWait([]string{"get", "function", functionName}, map[string]string{}, true)
	suite.Require().NoError(err)

	// import the function
	err = suite.ExecuteNuctl([]string{"import", "fu", exportTempFile.Name()}, map[string]string{})
	suite.Require().NoError(err)

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)

	// wait until able to get the function
	err = suite.ExecuteNuctlAndWait([]string{"get", "function", functionName}, map[string]string{}, false)
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
	err = suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose"}, map[string]string{})
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
	functionName := "reverser" + uniqueSuffix
	imageName := "nuclio/processor-" + functionName

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName)

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"},
		map[string]string{
			"image":   imageName,
			"runtime": "golang",
			"handler": "main:Reverse",
		})

	suite.Require().Error(err, "Function code must be provided either in the path or inline in a spec file; alternatively, an image or handler may be provided")

	// reset output buffer for reading the nex output cleanly
	suite.outputBuffer.Reset()

	// export the function
	err = suite.ExecuteNuctlAndWait([]string{"export", "fu", functionName}, map[string]string{}, false)
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
	err = suite.ExecuteNuctlAndWait([]string{"get", "function", functionName}, map[string]string{}, true)
	suite.Require().NoError(err)

	// import the function
	err = suite.ExecuteNuctl([]string{"import", "fu", exportTempFile.Name()}, map[string]string{})
	suite.Require().NoError(err)

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)

	// wait until able to get the function
	err = suite.ExecuteNuctlAndWait([]string{"get", "function", functionName}, map[string]string{}, false)
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
	err = suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose"}, map[string]string{})

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
