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
	"fmt"
	"path"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/common"

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

	err := suite.ExecuteNutcl([]string{"build", functionName, "--verbose", "--no-pull"},
		map[string]string{
			"path":    path.Join(suite.GetFunctionsDir(), "common", "reverser", "golang"),
			"image":   imageName,
			"runtime": "golang",
		})

	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName)

	// use deploy with the image we just created
	err = suite.ExecuteNutcl([]string{"deploy", functionName, "--verbose"},
		map[string]string{
			"run-image": imageName,
			"runtime":   "golang",
			"handler":   "main:Reverse",
		})

	suite.Require().NoError(err)

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNutcl([]string{"delete", "fu", functionName}, nil)

	// try a few times to invoke, until it succeeds
	err = common.RetryUntilSuccessful(60*time.Second, 1*time.Second, func() bool {

		// invoke the function
		err = suite.ExecuteNutcl([]string{"invoke", functionName},
			map[string]string{
				"method": "POST",
				"body":   "-reverse this string+",
				"via":    "external-ip",
			})

		return err == nil
	})

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

	err := suite.ExecuteNutcl([]string{"deploy", functionName, "--verbose", "--no-pull"}, namedArgs)

	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName)

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNutcl([]string{"delete", "fu", functionName}, nil)

	// try a few times to invoke, until it succeeds
	err = common.RetryUntilSuccessful(60*time.Second, 1*time.Second, func() bool {

		// invoke the function
		err = suite.ExecuteNutcl([]string{"invoke", functionName},
			map[string]string{
				"method": "POST",
				"body":   "-reverse this string+",
				"via":    "external-ip",
			})

		return err == nil
	})

	suite.Require().NoError(err)

	// make sure reverser worked
	suite.Require().Contains(suite.outputBuffer.String(), "+gnirts siht esrever-")
}

func (suite *functionDeployTestSuite) TestDeployWithMetadata() {
	uniqueSuffix := "-" + xid.New().String()
	functionName := "envprinter" + uniqueSuffix
	imageName := "nuclio/deploy-test" + uniqueSuffix

	err := suite.ExecuteNutcl([]string{"deploy", functionName, "--verbose", "--no-pull"},
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
	defer suite.ExecuteNutcl([]string{"delete", "fu", functionName}, nil)

	// try a few times to invoke, until it succeeds
	err = common.RetryUntilSuccessful(60*time.Second, 1*time.Second, func() bool {

		// invoke the function
		err = suite.ExecuteNutcl([]string{"invoke", functionName},
			map[string]string{
				"method": "POST",
				"body":   "-reverse this string+",
				"via":    "external-ip",
			})

		return err == nil
	})

	suite.Require().NoError(err)

	// make sure reverser worked
	suite.Require().Contains(suite.outputBuffer.String(), "11223344")
	suite.Require().Contains(suite.outputBuffer.String(), "0099887766")
}

func (suite *functionDeployTestSuite) TestDeployFromFunctionConfig() {
	randomString := xid.New().String()
	uniqueSuffix := "-" + randomString
	imageName := "nuclio/deploy-test" + uniqueSuffix

	err := suite.ExecuteNutcl([]string{"deploy", "", "--verbose", "--no-pull"},
		map[string]string{
			"path": path.Join(suite.GetFunctionsDir(), "common", "json-parser-with-function-config", "python"),
		})

	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName)

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNutcl([]string{"delete", "fu", "parser"}, nil)

	// try a few times to invoke, until it succeeds
	err = common.RetryUntilSuccessful(60*time.Second, 1*time.Second, func() bool {

		// invoke the function
		err = suite.ExecuteNutcl([]string{"invoke", "parser"},
			map[string]string{
				"method": "POST",
				"body":   fmt.Sprintf(`{"return_this": "%s"}`, randomString),
				"via":    "external-ip",
			})

		return err == nil
	})

	suite.Require().NoError(err)

	// check that invoke printed the value
	suite.Require().Contains(suite.outputBuffer.String(), randomString)
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

	err := suite.ExecuteNutcl([]string{"deploy", functionName, "--verbose", "--no-pull"}, namedArgs)
	suite.Require().NoError(err)

	time.Sleep(2 * time.Second)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName)

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNutcl([]string{"delete", "fu", functionName}, nil)

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
		err = suite.ExecuteNutcl([]string{"invoke", functionName},
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

	err := suite.ExecuteNutcl([]string{"deploy", functionName, "--verbose", "--no-pull"},
		map[string]string{
			"image":   imageName,
			"runtime": "golang",
			"handler": "main:Reverse",
		})

	suite.Require().Error(err, "Function code must be provided either in the path or inline in a spec file; alternatively, an image or handler may be provided")
}

func (suite *functionDeployTestSuite) TestDeployFailsOnShellMissingPathAndHandler() {
	uniqueSuffix := "-" + xid.New().String()
	functionName := "reverser" + uniqueSuffix
	imageName := "nuclio/deploy-test" + uniqueSuffix

	err := suite.ExecuteNutcl([]string{"deploy", functionName, "--verbose", "--no-pull"},
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

	err := suite.ExecuteNutcl([]string{"deploy", functionName, "--verbose", "--no-pull"},
		map[string]string{
			"image":   imageName,
			"runtime": "shell",
			"handler": "rev",
		})

	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName)

	// use nuctl to delete the function when we're done
	defer suite.ExecuteNutcl([]string{"delete", "fu", functionName}, nil)

	// try a few times to invoke, until it succeeds
	err = common.RetryUntilSuccessful(60*time.Second, 1*time.Second, func() bool {

		err = suite.ExecuteNutcl([]string{"invoke", functionName},
			map[string]string{
				"method": "POST",
				"body":   "-reverse this string+",
				"via":    "external-ip",
			})

		return err == nil
	})

	suite.Require().NoError(err)

	// make sure reverser worked
	suite.Require().Contains(suite.outputBuffer.String(), "+gnirts siht esrever-")
}

type functionGetTestSuite struct {
	Suite
}

func (suite *functionGetTestSuite) TestGet() {
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

		err := suite.ExecuteNutcl([]string{
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
			suite.ExecuteNutcl([]string{"delete", "fu", functionName}, nil)
		}(imageName, functionName)
	}

	err := suite.ExecuteNutcl([]string{"get", "function"}, nil)
	suite.Require().NoError(err)

	// find function names in get result
	suite.findPatternsInOutput(functionNames, nil)
}

func TestFunctionTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(functionBuildTestSuite))
	suite.Run(t, new(functionDeployTestSuite))
	suite.Run(t, new(functionGetTestSuite))
}
