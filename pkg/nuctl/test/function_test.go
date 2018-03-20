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
	"strconv"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

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
			})

		return err == nil
	})

	suite.Require().NoError(err)

	// make sure reverser worked
	suite.Require().Contains(suite.outputBuffer.String(), "11223344")
	suite.Require().Contains(suite.outputBuffer.String(), "0099887766")
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
			})

		return err == nil
	})

	suite.Require().NoError(err)

	// make sure reverser worked
	suite.Require().Contains(suite.outputBuffer.String(), "+gnirts siht esrever-")
}

type functionGetTestSuite struct {
	httpsuite.TestSuite
}

func (suite *functionGetTestSuite) TestGet() {
	// Prepare options for function
	deployOptions := suite.GetDeployOptions("reverser",
		suite.GetFunctionPath(functionPath(), "common", "reverser", "golang"))

	deployOptions.FunctionConfig.Spec.Handler = "main:Reverse"
	deployOptions.FunctionConfig.Spec.Runtime = "golang"

	// Initialize variables
	functionNamespace := "function_test_namespace"
	numOfFunctions := 3
	var functionNames []string
	var expectedFunctionPorts []int
	var containerIds []string
	var err error
	var functionName string

	// Create loop for function deployment
	for functionIdx := 0; functionIdx < numOfFunctions; functionIdx++ {
		uniqueSuffix := fmt.Sprintf("-%s-%d", xid.New().String(), functionIdx)

		functionName = "reverser" + uniqueSuffix

		deployOptions.FunctionConfig.Meta.Name = functionName
		deployOptions.FunctionConfig.Meta.Namespace = functionNamespace

		// Deploy function
		functionNames = append(functionNames, functionName)

		// Get results
		funcResult := suite.DeployContainer(deployOptions)
		expectedFunctionPorts = append(expectedFunctionPorts, funcResult.Port)
		containerIds = append(containerIds, funcResult.ContainerID)

	}

	// Use nutctl to delete the function when we're done
	defer func(containerIds []string) {
		for functionIdx := 0; functionIdx < len(containerIds); functionIdx++ {

			err := suite.DockerClient.RemoveContainer(containerIds[functionIdx])

			suite.Require().NoError(err)
		}
	}(containerIds)

	// Get results of deployed function for verification
	functionsResult, err := suite.Platform.GetFunctions(&platform.GetFunctionsOptions{
		Namespace: functionNamespace,
	})
	if err != nil {
		errors.Wrap(err, "Failed to get functions")
	}

	if len(functionsResult) == 0 {
		fmt.Errorf("Function not found: %s @ %s", functionNames[0])
	}

	// Get actual ports
	actualFunctionPorts := printFunction(functionsResult)

	// Verify that number of actual and expected functions is the same
	suite.Require().True(len(expectedFunctionPorts) == len(actualFunctionPorts))

	// Convert slice of int into string
	actualFunctionPortsString := convertArrayOfIntIntoString(actualFunctionPorts)

	// Verify that ports are equals
	for functionIdx := 0; functionIdx < len(expectedFunctionPorts); functionIdx++ {
		suite.Require().Contains(actualFunctionPortsString, strconv.Itoa(expectedFunctionPorts[functionIdx]))
	}
}

func TestFunctionTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(functionBuildTestSuite))
	suite.Run(t, new(functionDeployTestSuite))
	suite.Run(t, new(functionGetTestSuite))
}

func printFunction(function []platform.Function) []int {
	var functionsPorts []int
	for functionIdx := 0; functionIdx < len(function); functionIdx++ {
		functionsPorts = append(functionsPorts, function[functionIdx].GetConfig().Spec.HTTPPort)
	}
	return functionsPorts
}

func convertArrayOfIntIntoString(arrayOfInt []int) string {
	var arrayInString string
	for functionIdx := 0; functionIdx < len(arrayOfInt); functionIdx++ {
		arrayInString = arrayInString + strconv.Itoa(arrayOfInt[functionIdx]) + " "
	}
	return arrayInString
}
