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

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/nuctl/command"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/ghodss/yaml"
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
	err := suite.ExecuteNuctl([]string{"get", "project"}, nil)
	suite.Require().NoError(err)

	for projectIdx := 0; projectIdx < numOfProjects; projectIdx++ {
		uniqueSuffix := fmt.Sprintf("-%s-%d", xid.New().String(), projectIdx)

		projectName := "get-test-project" + uniqueSuffix

		// add project name to list
		projectNames = append(projectNames, projectName)

		namedArgs := map[string]string{
			"description": fmt.Sprintf("description-%d", projectIdx),
		}

		err := suite.ExecuteNuctl([]string{
			"create",
			"project",
			projectName,
			"--verbose",
		}, namedArgs)

		suite.Require().NoError(err)

		// cleanup
		defer func(projectName string) {

			// use nutctl to delete the project when we're done
			suite.ExecuteNuctl([]string{"delete", "proj", projectName}, nil)

		}(projectName)
	}

	err = suite.ExecuteNuctl([]string{"get", "project"}, nil)
	suite.Require().NoError(err)

	// find function names in get result
	suite.findPatternsInOutput(projectNames, nil)

	// delete the second project
	err = suite.ExecuteNuctl([]string{"delete", "proj", projectNames[1], "--verbose"}, nil)
	suite.Require().NoError(err)

	// get again
	err = suite.ExecuteNuctl([]string{"get", "project"}, nil)
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
	err := suite.ExecuteNuctl([]string{
		"create",
		"project",
		projectName,
	}, nil)

	suite.Require().NoError(err)

	// cleanup
	defer func(projectName string) {

		// use nutctl to delete the project when we're done
		suite.ExecuteNuctl([]string{"delete", "proj", projectName}, nil)

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

	err = suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"}, namedArgs)
	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName)

	// make sure the function is deleted
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)

	// try to delete the project - should fail
	err = suite.ExecuteNuctl([]string{"delete", "proj", projectName}, nil)
	suite.Require().Error(err)

	// delete the function
	err = suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)
	suite.Require().NoError(err)

	// now delete the project again - should succeed
	err = suite.ExecuteNuctl([]string{"delete", "proj", projectName}, nil)
	suite.Require().NoError(err)
}

type projectExportImportTestSuite struct {
	Suite
}

func (suite *projectExportImportTestSuite) createProject(projectName string) {
	namedArgs := map[string]string{
		"description": projectName,
	}

	err := suite.ExecuteNuctl([]string{
		"create",
		"project",
		projectName,
		"--verbose",
	}, namedArgs)
	suite.Require().NoError(err)

	// wait until able to get the project
	err = suite.ExecuteNuctlAndWait([]string{"get", "project", projectName}, map[string]string{}, false)
	suite.Require().NoError(err)
}

func (suite *projectExportImportTestSuite) createFunction(functionName, projectName string) {
	namedArgs := map[string]string{
		"path":         path.Join(suite.GetFunctionsDir(), "common", "reverser", "golang"),
		"project-name": projectName,
		"runtime":      "golang",
		"handler":      "main:Reverse",
	}

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"}, namedArgs)
	suite.Require().NoError(err)

	// wait until able to get the project
	err = suite.ExecuteNuctl([]string{"get", "function", functionName}, map[string]string{})
	suite.Require().NoError(err)
}

func (suite *projectExportImportTestSuite) createFunctionEvent(functionEventName, functionName string) {
	namedArgs := map[string]string{
		"function":     functionName,
		"display-name": fmt.Sprintf("display-name-%s", functionEventName),
		"trigger-name": fmt.Sprintf("trigger-name-%s", functionEventName),
		"trigger-kind": fmt.Sprintf("trigger-kind-%s", functionEventName),
		"body":         fmt.Sprintf("body-%s", functionEventName),
	}

	err := suite.ExecuteNuctl([]string{
		"create",
		"functionevent",
		functionEventName,
	}, namedArgs)

	suite.Require().NoError(err)
}

func (suite *projectExportImportTestSuite) assertProjectImported(projectName string) {

	// reset output buffer for reading the nex output cleanly
	suite.outputBuffer.Reset()
	err := suite.ExecuteNuctlAndWait([]string{"get", "project", projectName}, map[string]string{
		"output": "yaml",
	}, false)
	suite.Require().NoError(err)

	project := platform.ProjectConfig{}
	err = yaml.Unmarshal(suite.outputBuffer.Bytes(), &project)
	suite.Require().NoError(err)

	suite.Assert().Equal(projectName, project.Meta.Name)
}

func (suite *projectExportImportTestSuite) assertFunctionImported(functionName string, imported bool) {

	// reset output buffer for reading the nex output cleanly
	suite.outputBuffer.Reset()
	err := suite.ExecuteNuctlAndWait([]string{"get", "function", functionName}, map[string]string{
		"output": "yaml",
	}, false)
	suite.Require().NoError(err)

	function := functionconfig.Config{}
	err = yaml.Unmarshal(suite.outputBuffer.Bytes(), &function)
	suite.Require().NoError(err)

	suite.Assert().Equal(functionName, function.Meta.Name)
	if imported {

		// get imported functions
		err = suite.ExecuteNuctl([]string{"get", "function", functionName}, nil)
		suite.Require().NoError(err)

		// ensure function state is imported
		suite.findPatternsInOutput([]string{"imported"}, nil)
	}
}

func (suite *projectExportImportTestSuite) assertFunctionEventExistenceByFunction(functionEventDisplayName,
	functionName string) string {

	// reset output buffer for reading the nex output cleanly
	suite.outputBuffer.Reset()
	err := suite.ExecuteNuctlAndWait([]string{"get", "functionevent"}, map[string]string{
		"output": "yaml",
		"function": functionName,
	}, false)
	suite.Require().NoError(err)

	functionEvent := platform.FunctionEventConfig{}
	err = yaml.Unmarshal(suite.outputBuffer.Bytes(), &functionEvent)
	suite.Require().NoError(err)

	suite.Assert().Equal(functionEventDisplayName, functionEvent.Spec.DisplayName)
	suite.Assert().Equal(functionName, functionEvent.Meta.Labels["nuclio.io/function-name"])
	return functionEvent.Meta.Name
}

func (suite *projectExportImportTestSuite) TestExportProject() {
	projectName := "test-project"
	functionName := "test-function"
	functionEventName := "test-function-event"

	suite.createProject(projectName)
	suite.createFunction(functionName, projectName)
	suite.createFunctionEvent(functionEventName, functionName)

	defer suite.ExecuteNuctl([]string{"delete", "proj", projectName}, nil)
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)
	defer suite.ExecuteNuctl([]string{"delete", "fe", functionEventName}, nil)

	// reset output buffer for reading the nex output cleanly
	suite.outputBuffer.Reset()

	// export the project
	err := suite.ExecuteNuctlAndWait([]string{"export", "proj", projectName, "--verbose"}, map[string]string{}, false)
	suite.Require().NoError(err)

	exportedProjectConfig := &command.ProjectImportConfig{}
	err = yaml.Unmarshal(suite.outputBuffer.Bytes(), &exportedProjectConfig)
	suite.Require().NoError(err)

	suite.Assert().Equal(exportedProjectConfig.Project.Meta.Name, projectName)
	suite.Assert().Equal(exportedProjectConfig.Functions[functionName].Meta.Name, functionName)
	suite.Assert().Equal(exportedProjectConfig.FunctionEvents[functionEventName].Meta.Name, functionEventName)
}

func (suite *projectExportImportTestSuite) TestImportProject() {
	projectConfigPath := path.Join(suite.GetImportsDir(), "project.yaml")
	projectName := "test-project"
	function1Name := "test-function-1"
	function2Name := "test-function-2"
	function1EventDisplayName := "test-function-event-1"
	function2EventDisplayName := "test-function-event-2"

	defer suite.ExecuteNuctl([]string{"delete", "proj", projectName}, nil)
	defer suite.ExecuteNuctl([]string{"delete", "fu", function1Name}, nil)
	defer suite.ExecuteNuctl([]string{"delete", "fu", function2Name}, nil)

	// import the project
	err := suite.ExecuteNuctl([]string{"import", "proj", projectConfigPath, "--verbose"}, map[string]string{})
	suite.Require().NoError(err)

	suite.assertProjectImported(projectName)
	suite.assertFunctionImported(function1Name, true)
	suite.assertFunctionImported(function2Name, true)
	function1EventName := suite.assertFunctionEventExistenceByFunction(function1EventDisplayName, function1Name)
	function2EventName := suite.assertFunctionEventExistenceByFunction(function2EventDisplayName, function2Name)

	defer suite.ExecuteNuctl([]string{"delete", "fe", function1EventName}, nil)
	defer suite.ExecuteNuctl([]string{"delete", "fe", function2EventName}, nil)
}

func (suite *projectExportImportTestSuite) TestImportProjectWithExistingFunction() {
	projectConfigPath := path.Join(suite.GetImportsDir(), "project.yaml")
	projectName := "test-project"
	function1Name := "test-function-1"
	function2Name := "test-function-2"
	function1EventDisplayName := "test-function-event-1"
	function2EventDisplayName := "test-function-event-2"

	suite.createProject(projectName)
	suite.createFunction(function1Name, projectName)

	defer suite.ExecuteNuctl([]string{"delete", "proj", projectName}, nil)
	defer suite.ExecuteNuctl([]string{"delete", "fu", function1Name}, nil)
	defer suite.ExecuteNuctl([]string{"delete", "fu", function2Name}, nil)

	// import the project
	err := suite.ExecuteNuctl([]string{"import", "proj", projectConfigPath, "--verbose"}, map[string]string{})

	// Expect error for existing function
	suite.Require().Error(err)

	suite.assertProjectImported(projectName)
	suite.assertFunctionImported(function1Name, false)
	suite.assertFunctionImported(function2Name, true)
	function1EventName := suite.assertFunctionEventExistenceByFunction(function1EventDisplayName, function1Name)
	function2EventName := suite.assertFunctionEventExistenceByFunction(function2EventDisplayName, function2Name)

	defer suite.ExecuteNuctl([]string{"delete", "fe", function1EventName}, nil)
	defer suite.ExecuteNuctl([]string{"delete", "fe", function2EventName}, nil)
}

func TestProjectTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(projectGetTestSuite))
	suite.Run(t, new(projectDeleteTestSuite))
	suite.Run(t, new(projectExportImportTestSuite))
}
