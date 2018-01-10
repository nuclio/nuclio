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
	"bufio"
	"fmt"
	"path"
	"strings"
	"testing"

	"github.com/nuclio/nuclio/pkg/version"

	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
)

type GetTestSuite struct {
	Suite
}

func (suite *GetTestSuite) SetupSuite() {
	suite.Suite.SetupSuite()

	// update version so that linker doesn't need to inject it
	version.Set(&version.Info{
		GitCommit: "c",
		Label:     "latest",
		Arch:      "amd64",
		OS:        "linux",
	})
}

func (suite *GetTestSuite) TestGet() {

	numOfFunctions := 3
	var functionNames []string
	var functionImages []string

	for functionIdx := 0; functionIdx < numOfFunctions; functionIdx++ {
		uniqueSuffix := fmt.Sprintf("-%s-%d", xid.New().String(), functionIdx)

		imageName := "nuclio/deploy-test" + uniqueSuffix
		functionName := "reverser" + uniqueSuffix

		// add function name to list
		functionNames = append(functionNames, functionName)
		functionImages = append(functionImages, imageName)

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

	}
	
	// cleanup try delete, use nuctl to delete the function when we're done
	defer func() {

		// make sure to clean up after the test
		suite.dockerClient.RemoveImage(functionImages[0])

		// use nuctl to delete the function when we're done
		suite.ExecuteNutcl([]string{"delete", "fu", functionNames[0]}, nil)

		suite.dockerClient.RemoveImage(functionImages[1])
		suite.dockerClient.RemoveImage(functionImages[2])

		// use nuctl to delete the function when we're done
		suite.ExecuteNutcl([]string{"delete", "fu", functionNames[1], functionNames[2]}, nil)
	}()

	err := suite.ExecuteNutcl([]string{"get", "function"}, nil)

	suite.Require().NoError(err)

	foundFunctionTests := [][]bool{getCheckOutputTest(suite, functionNames)}

	testMultipleGetFunctions := []string{functionNames[0], functionNames[1]}
	err = suite.ExecuteNutcl([]string{"get", "fu", testMultipleGetFunctions[0], testMultipleGetFunctions[1]}, nil)

	suite.Require().NoError(err)

	foundFunctionTests = append(foundFunctionTests, getCheckOutputTest(suite, testMultipleGetFunctions))

	for _, foundFunctions := range foundFunctionTests {
		for _, foundFunction := range foundFunctions {
			suite.Require().True(foundFunction)
		}
	}
}

// Takes suite & slice of Strings contains required names that the suite scanner should output
func getCheckOutputTest(suite *GetTestSuite, requiredNames []string) []bool {

	// functions found/not found in process, return value of the function
	foundFunctions := make([]bool, len(requiredNames))

	// Iterate over all lines in get result. for each function created in this test that we find,
	// set the equivalent boolean in foundFunctions
	scanner := bufio.NewScanner(&suite.outputBuffer)
	for scanner.Scan() {

		for functionIdx, functionName := range requiredNames {

			// if the function name is in the list, remove it
			if strings.Contains(scanner.Text(), functionName) {
				foundFunctions[functionIdx] = true
				break
			}
		}
	}

	return foundFunctions
}

func TestGetTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(GetTestSuite))
}
