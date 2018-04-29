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

	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
)

type projectGetTestSuite struct {
	Suite
}

func (suite *projectGetTestSuite) TestGet() {
	numOfProjects := 3
	var projectNames []string

	// get with nothing created - should pass
	err := suite.ExecuteNutcl([]string{"get", "project"}, nil)
	suite.Require().NoError(err)

	for projectIdx := 0; projectIdx < numOfProjects; projectIdx++ {
		uniqueSuffix := fmt.Sprintf("-%s-%d", xid.New().String(), projectIdx)

		projectName := "get-test-project" + uniqueSuffix

		// add project name to list
		projectNames = append(projectNames, projectName)

		namedArgs := map[string]string{
			"display-name": fmt.Sprintf("display-name-%d", projectIdx),
			"description":  fmt.Sprintf("description-%d", projectIdx),
		}

		err := suite.ExecuteNutcl([]string{
			"create",
			"project",
			projectName,
			"--verbose",
		}, namedArgs)

		suite.Require().NoError(err)

		// cleanup
		defer func(projectName string) {

			// use nutctl to delete the project when we're done
			suite.ExecuteNutcl([]string{"delete", "proj", projectName}, nil)

		}(projectName)
	}

	err = suite.ExecuteNutcl([]string{"get", "project"}, nil)
	suite.Require().NoError(err)

	// find function names in get result
	suite.findPatternsInOutput(projectNames, nil)

	// delete the second project
	err = suite.ExecuteNutcl([]string{"delete", "proj", projectNames[1], "--verbose"}, nil)
	suite.Require().NoError(err)

	// get again
	err = suite.ExecuteNutcl([]string{"get", "project"}, nil)
	suite.Require().NoError(err)

	// verify second project deleted
	suite.findPatternsInOutput([]string{
		projectNames[0], projectNames[2],
	}, []string{
		projectNames[1],
	})
}

type projectDeleteTestSuite struct {
	Suite
}

func (suite *projectGetTestSuite) TestDeleteWithFunctions() {
	uniqueSuffix := fmt.Sprintf("-%s", xid.New().String())
	functionName := "reverser" + uniqueSuffix
	projectName := "get-test-project" + uniqueSuffix

	// create a project
	err := suite.ExecuteNutcl([]string{
		"create",
		"project",
		projectName,
	}, nil)

	suite.Require().NoError(err)

	// cleanup
	defer func(projectName string) {

		// use nutctl to delete the project when we're done
		suite.ExecuteNutcl([]string{"delete", "proj", projectName}, nil)

	}(projectName)

	// deploy a function
	imageName := fmt.Sprintf("nuclio/deploy-test-%s", xid.New().String())
	namedArgs := map[string]string{
		"path":         path.Join(suite.GetFunctionsDir(), "common", "reverser", "golang"),
		"image":        imageName,
		"runtime":      "golang",
		"handler":      "main:Reverse",
		"project-name": projectName,
	}

	err = suite.ExecuteNutcl([]string{"deploy", functionName, "--verbose", "--no-pull"}, namedArgs)
	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName)

	// make sure the function is deleted
	defer suite.ExecuteNutcl([]string{"delete", "fu", functionName}, nil)

	// try to delete the project - should fail
	err = suite.ExecuteNutcl([]string{"delete", "proj", projectName}, nil)
	suite.Require().Error(err)

	// delete the function
	err = suite.ExecuteNutcl([]string{"delete", "fu", functionName}, nil)
	suite.Require().NoError(err)

	// now delete the project again - should succeed
	err = suite.ExecuteNutcl([]string{"delete", "proj", projectName}, nil)
	suite.Require().NoError(err)
}

func TestProjectTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(projectGetTestSuite))
	suite.Run(t, new(projectDeleteTestSuite))
}
