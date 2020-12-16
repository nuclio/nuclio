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
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/nuctl/command"
	"github.com/nuclio/nuclio/pkg/nuctl/command/common"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/ghodss/yaml"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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
			suite.ExecuteNuctl([]string{"delete", "proj", projectName}, nil) // nolint: errcheck

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
		suite.ExecuteNuctl([]string{"delete", "proj", projectName}, nil) // nolint: errcheck

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
	defer suite.dockerClient.RemoveImage(imageName) // nolint: errcheck

	// make sure the function is deleted
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

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

func (suite *projectExportImportTestSuite) TestExportProject() {
	apiGatewaysEnabled := suite.origPlatformKind == "kube"

	uniqueSuffix := "-" + xid.New().String()
	projectName := "test-project" + uniqueSuffix
	functionName := "test-function" + uniqueSuffix
	functionEventName := "test-function-event" + uniqueSuffix
	apiGatewayName := "test-api-gateway" + uniqueSuffix

	suite.createProject(projectName)
	defer suite.ExecuteNuctl([]string{"delete", "proj", projectName}, nil) // nolint: errcheck

	suite.createFunction(functionName, projectName)
	suite.createFunctionEvent(functionEventName, functionName)
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil)      // nolint: errcheck
	defer suite.ExecuteNuctl([]string{"delete", "fe", functionEventName}, nil) // nolint: errcheck

	if apiGatewaysEnabled {
		suite.createAPIGateway(apiGatewayName, functionName, projectName)
		defer suite.ExecuteNuctl([]string{"delete", "agw", apiGatewayName}, nil) // nolint: errcheck
	}

	exportedProjectConfig := suite.getExportedProject(projectName, []string{})

	suite.Assert().Equal(exportedProjectConfig.Project.Meta.Name, projectName)
	suite.Assert().Equal(exportedProjectConfig.Functions[functionName].Meta.Name, functionName)
	suite.Assert().Equal(exportedProjectConfig.FunctionEvents[functionEventName].Meta.Name, functionEventName)

	if apiGatewaysEnabled {
		suite.Assert().Equal(exportedProjectConfig.APIGateways[apiGatewayName].Meta.Name, apiGatewayName)
		suite.Assert().Equal(exportedProjectConfig.APIGateways[apiGatewayName].Spec.Host, fmt.Sprintf("host-%s", apiGatewayName))
		suite.Assert().Equal(exportedProjectConfig.APIGateways[apiGatewayName].Spec.Upstreams[0].Kind, platform.APIGatewayUpstreamKindNuclioFunction)
		suite.Assert().Equal(exportedProjectConfig.APIGateways[apiGatewayName].Spec.Upstreams[0].Nucliofunction.Name, functionName)
	}
}

func (suite *projectExportImportTestSuite) TestImportProjectWithDisplayName() {
	exportedProjectTemplate := `%[1]s:
  apiGateways: {}
  functionEvents: {}
  functions:
    %[3]s:
      metadata:
        name: %[3]s
        annotations:
          skip-build: "true"
          skip-deploy: "true"
        labels:
          nuclio.io/project-name: %[1]s
      spec:
        build:
          codeEntryType: sourceCode
          functionSourceCode: ZWNobyAidGVzdDEi
          noBaseImagesPull: true
        handler: main.sh
        runtime: shell
  project:
    meta:
      name: %[1]s
    spec:
      displayName: %[2]s`

	for _, testCase := range []struct {
		name                        string
		projectName                 string
		displayName                 string
		importProjectPositionalArgs []string
		expectedProjectName         string
		expectedFailure             bool
	}{
		{
			name:        "EmptyDisplayName",
			projectName: "test-project-" + xid.New().String(),
			displayName: "",
		},
		{
			name:                "TransformedDisplayName",
			projectName:         "12345678-1234-1234-1234-123456789001",
			displayName:         "assign me KEBAB",
			expectedProjectName: "assign-me-kebab",
		},

		// project name is not transformable
		{
			name:            "ImportProjectWithDisplayNameExplode",
			projectName:     "test-project-" + xid.New().String(),
			displayName:     "explode",
			expectedFailure: true,
		},
	} {
		suite.Run(testCase.name, func() {
			if testCase.expectedProjectName == "" {
				testCase.expectedProjectName = testCase.projectName
			}

			// import project from stdin
			functionName := "import-prof-with-display-name-" + xid.New().String()
			exportedProject := fmt.Sprintf(exportedProjectTemplate,
				testCase.projectName,
				testCase.displayName,
				functionName)
			suite.inputBuffer = *bytes.NewBufferString(exportedProject)

			// import
			importProjectPositionalArgs := []string{"import", "project", "--verbose"}
			if testCase.importProjectPositionalArgs != nil {
				importProjectPositionalArgs = append(importProjectPositionalArgs,
					testCase.importProjectPositionalArgs...)
			}
			err := suite.ExecuteNuctl(importProjectPositionalArgs, nil)

			// delete leftovers
			defer suite.ExecuteNuctl([]string{"delete", "project", testCase.expectedProjectName}, nil) // nolint: errcheck
			defer suite.ExecuteNuctl([]string{"delete", "function", functionName}, nil)                // nolint: errcheck

			if testCase.expectedFailure {
				suite.Require().Error(err)
				return
			}

			// assertions
			suite.Require().NoError(err)
			suite.assertProjectImported(testCase.expectedProjectName)
			suite.assertFunctionImported(functionName, true)

			functionConfigWithStatus, err := suite.getFunctionInFormat(functionName, common.OutputFormatYAML)
			suite.Require().NoError(err)
			suite.Require().Equal(testCase.expectedProjectName,
				functionConfigWithStatus.Meta.Labels["nuclio.io/project-name"])
		})
	}
}

func (suite *projectExportImportTestSuite) TestExportProjectWithDisplayName() {
	suite.ensureRunningOnPlatform("kube")

	importProjectWithDisplayName := func(projectName string) {
		_, err := suite.shellClient.Run(nil, `cat <<EOF | kubectl apply -f -
apiVersion: nuclio.io/v1beta1
kind: NuclioProject
metadata:
  name: %[1]s
  namespace: %[2]s
spec:
  displayName: test-display-name
  description: test-description
EOF
`, projectName, suite.namespace)
		suite.Require().NoError(err)
	}

	for _, testCase := range []struct {
		name                        string
		projectName                 string
		importProjectPositionalArgs []string
		expectedExportedProject     *platform.ProjectConfig
	}{
		{
			name:        "Omit display name",
			projectName: "test-project" + xid.New().String(),
			expectedExportedProject: &platform.ProjectConfig{
				Meta: platform.ProjectMeta{
					Namespace: suite.namespace,
				},
				Spec: platform.ProjectSpec{
					Description: "test-description",
				},
			},
		},
	} {
		suite.Run(testCase.name, func() {
			importProjectWithDisplayName(testCase.projectName)

			// name is dynamically created and should not changed during creating / exporting
			testCase.expectedExportedProject.Meta.Name = testCase.projectName

			// delete leftovers
			defer suite.ExecuteNuctl([]string{"delete", "project", testCase.projectName}, nil) // nolint: errcheck

			exportedProjectConfig := suite.getExportedProject(testCase.projectName,
				testCase.importProjectPositionalArgs)
			suite.Require().Empty(cmp.Diff(testCase.expectedExportedProject, exportedProjectConfig.Project,
				cmp.Options{
					cmpopts.IgnoreFields(testCase.expectedExportedProject.Meta, "Annotations"),
				},
			))
		})
	}
}

func (suite *projectExportImportTestSuite) TestImportProjects() {
	apiGatewaysEnabled := suite.origPlatformKind == "kube"

	projectConfigPath := path.Join(suite.GetImportsDir(), "projects.yaml")

	// these names explicitly defined within projects.yaml
	projectAName := "project-a"
	projectBName := "project-b"
	function1Name := "test-function-1"
	function2Name := "test-function-2"
	function3Name := "test-function-3"
	function4Name := "test-function-4"
	function1EventDisplayName := "test-function-event-1"
	function2EventDisplayName := "test-function-event-2"
	function3EventDisplayName := "test-function-event-3"
	function4EventDisplayName := "test-function-event-4"

	apiGateway1Name := "test-api-gateway-1"
	apiGateway2Name := "test-api-gateway-2"

	defer suite.ExecuteNuctl([]string{"delete", "fu", function1Name}, nil)  // nolint: errcheck
	defer suite.ExecuteNuctl([]string{"delete", "fu", function2Name}, nil)  // nolint: errcheck
	defer suite.ExecuteNuctl([]string{"delete", "fu", function3Name}, nil)  // nolint: errcheck
	defer suite.ExecuteNuctl([]string{"delete", "fu", function4Name}, nil)  // nolint: errcheck
	defer suite.ExecuteNuctl([]string{"delete", "proj", projectAName}, nil) // nolint: errcheck
	defer suite.ExecuteNuctl([]string{"delete", "proj", projectBName}, nil) // nolint: errcheck

	if apiGatewaysEnabled {
		defer suite.ExecuteNuctl([]string{"delete", "agw", apiGateway1Name}, nil) // nolint: errcheck
		defer suite.ExecuteNuctl([]string{"delete", "agw", apiGateway2Name}, nil) // nolint: errcheck
	}

	// import the project
	err := suite.ExecuteNuctl([]string{"import", "proj", projectConfigPath, "--verbose"}, nil)
	suite.Require().NoError(err)

	suite.assertProjectImported(projectAName)
	suite.assertProjectImported(projectBName)
	suite.assertFunctionImported(function1Name, true)
	suite.assertFunctionImported(function2Name, true)
	suite.assertFunctionImported(function3Name, true)
	suite.assertFunctionImported(function4Name, true)
	function1EventName := suite.assertFunctionEventExistenceByFunction(function1EventDisplayName, function1Name)
	function2EventName := suite.assertFunctionEventExistenceByFunction(function2EventDisplayName, function2Name)
	function3EventName := suite.assertFunctionEventExistenceByFunction(function3EventDisplayName, function3Name)
	function4EventName := suite.assertFunctionEventExistenceByFunction(function4EventDisplayName, function4Name)
	if apiGatewaysEnabled {
		suite.verifyAPIGatewayExists(apiGateway1Name, function1Name)
		suite.verifyAPIGatewayExists(apiGateway2Name, function3Name)
	}

	defer suite.ExecuteNuctl([]string{"delete", "fe", function1EventName}, nil) // nolint: errcheck
	defer suite.ExecuteNuctl([]string{"delete", "fe", function2EventName}, nil) // nolint: errcheck
	defer suite.ExecuteNuctl([]string{"delete", "fe", function3EventName}, nil) // nolint: errcheck
	defer suite.ExecuteNuctl([]string{"delete", "fe", function4EventName}, nil) // nolint: errcheck
}

func (suite *projectExportImportTestSuite) TestImportProject() {
	apiGatewaysEnabled := suite.origPlatformKind == "kube"

	uniqueSuffix := "-" + xid.New().String()
	projectConfigPath := path.Join(suite.GetImportsDir(), "project.yaml")
	projectName := "test-project" + uniqueSuffix
	function1Name := "test-function-1"
	function2Name := "test-function-2"
	function1EventDisplayName := "test-function-event-1"
	function2EventDisplayName := "test-function-event-2"
	apiGateway1Name := "test-api-gateway-1"
	apiGateway2Name := "test-api-gateway-2"

	uniqueProjectConfigPath := suite.addUniqueSuffixToImportConfig(projectConfigPath,
		uniqueSuffix,
		[]string{function1Name, function2Name},
		[]string{function1EventDisplayName, function2EventDisplayName},
		//[]string{apiGateway1Name})
		[]string{apiGateway1Name, apiGateway2Name})
	defer os.Remove(uniqueProjectConfigPath) // nolint: errcheck

	function1Name = function1Name + uniqueSuffix
	function2Name = function2Name + uniqueSuffix
	function1EventDisplayName = function1EventDisplayName + uniqueSuffix
	function2EventDisplayName = function2EventDisplayName + uniqueSuffix
	apiGateway1Name = apiGateway1Name + uniqueSuffix
	apiGateway2Name = apiGateway2Name + uniqueSuffix

	defer suite.ExecuteNuctl([]string{"delete", "proj", projectName}, nil) // nolint: errcheck
	defer suite.ExecuteNuctl([]string{"delete", "fu", function1Name}, nil) // nolint: errcheck
	defer suite.ExecuteNuctl([]string{"delete", "fu", function2Name}, nil) // nolint: errcheck

	if apiGatewaysEnabled {
		defer suite.ExecuteNuctl([]string{"delete", "agw", apiGateway1Name}, nil) // nolint: errcheck
		defer suite.ExecuteNuctl([]string{"delete", "agw", apiGateway2Name}, nil) // nolint: errcheck
	}

	// import the project
	err := suite.ExecuteNuctl([]string{"import", "proj", uniqueProjectConfigPath, "--verbose"}, nil)
	suite.Require().NoError(err)

	suite.assertProjectImported(projectName)
	suite.assertFunctionImported(function1Name, true)
	suite.assertFunctionImported(function2Name, true)
	function1EventName := suite.assertFunctionEventExistenceByFunction(function1EventDisplayName, function1Name)
	function2EventName := suite.assertFunctionEventExistenceByFunction(function2EventDisplayName, function2Name)

	// these function events were created as part of the project import performed above
	defer suite.ExecuteNuctl([]string{"delete", "fe", function1EventName}, nil) // nolint: errcheck
	defer suite.ExecuteNuctl([]string{"delete", "fe", function2EventName}, nil) // nolint: errcheck

	if apiGatewaysEnabled {
		suite.verifyAPIGatewayExists(apiGateway1Name, function1Name)
		suite.verifyAPIGatewayExists(apiGateway2Name, function2Name)
	}
}

func (suite *projectExportImportTestSuite) TestFailToImportProjectNoInput() {

	// import project without input
	err := suite.ExecuteNuctl([]string{"import", "project", "--verbose"}, nil)
	suite.Require().Error(err)
	suite.Require().Contains(err.Error(), "Failed to resolve project body")
}

func (suite *projectExportImportTestSuite) TestImportProjectWithExistingFunction() {
	uniqueSuffix := "-" + xid.New().String()
	projectConfigPath := path.Join(suite.GetImportsDir(), "project.yaml")
	projectName := "test-project" + uniqueSuffix
	function1Name := "test-function-1"
	function2Name := "test-function-2"
	function1EventDisplayName := "test-function-event-1"
	function2EventDisplayName := "test-function-event-2"

	uniqueProjectConfigPath := suite.addUniqueSuffixToImportConfig(projectConfigPath,
		uniqueSuffix,
		[]string{function1Name, function2Name},
		[]string{function1EventDisplayName, function2EventDisplayName},
		nil)
	defer os.Remove(uniqueProjectConfigPath) // nolint: errcheck

	function1Name = function1Name + uniqueSuffix
	function2Name = function2Name + uniqueSuffix
	function1EventDisplayName = function1EventDisplayName + uniqueSuffix
	function2EventDisplayName = function2EventDisplayName + uniqueSuffix

	suite.createProject(projectName)
	defer suite.ExecuteNuctl([]string{"delete", "proj", projectName}, nil) // nolint: errcheck

	suite.createFunction(function1Name, projectName)
	defer suite.ExecuteNuctl([]string{"delete", "fu", function1Name}, nil) // nolint: errcheck
	defer suite.ExecuteNuctl([]string{"delete", "fu", function2Name}, nil) // nolint: errcheck

	// import the project
	err := suite.ExecuteNuctl([]string{"import", "proj", uniqueProjectConfigPath, "--verbose"}, nil)

	// Expect error for existing function
	suite.Require().Error(err)

	suite.assertProjectImported(projectName)
	suite.assertFunctionImported(function1Name, false)
	suite.assertFunctionImported(function2Name, true)
	function1EventName := suite.assertFunctionEventExistenceByFunction(function1EventDisplayName, function1Name)
	function2EventName := suite.assertFunctionEventExistenceByFunction(function2EventDisplayName, function2Name)

	defer suite.ExecuteNuctl([]string{"delete", "fe", function1EventName}, nil) // nolint: errcheck
	defer suite.ExecuteNuctl([]string{"delete", "fe", function2EventName}, nil) // nolint: errcheck
}

func (suite *projectExportImportTestSuite) addUniqueSuffixToImportConfig(configPath, uniqueSuffix string,
	functionNames, functionEventNames []string, apiGatewayNames []string) string {
	file, err := ioutil.ReadFile(configPath)
	suite.Require().NoError(err)

	projectConfig := &command.ImportProjectConfig{}
	err = yaml.Unmarshal(file, projectConfig)
	suite.Require().NoError(err)

	projectConfig.Project.Meta.Name = projectConfig.Project.Meta.Name + uniqueSuffix
	projectConfig.Project.Meta.Namespace = suite.namespace
	functions := map[string]*functionconfig.Config{}
	for _, functionName := range functionNames {
		functionUniqueName := functionName + uniqueSuffix
		functions[functionUniqueName] = projectConfig.Functions[functionName]
		functions[functionUniqueName].Meta.Name = functionName + uniqueSuffix
		functions[functionUniqueName].Meta.Namespace = suite.namespace
		functions[functionUniqueName].Meta.Labels["nuclio.io/project-name"] =
			functions[functionUniqueName].Meta.Labels["nuclio.io/project-name"] + uniqueSuffix
	}
	projectConfig.Functions = functions

	functionEvents := map[string]*platform.FunctionEventConfig{}
	for _, functionEventName := range functionEventNames {
		functionEventUniqueName := functionEventName + uniqueSuffix
		functionEvents[functionEventUniqueName] = projectConfig.FunctionEvents[functionEventName]
		functionEvents[functionEventUniqueName].Spec.DisplayName = functionEventName + uniqueSuffix
		functionEvents[functionEventUniqueName].Meta.Namespace = suite.namespace

		functionEvents[functionEventUniqueName].Meta.Labels["nuclio.io/function-name"] =
			functionEvents[functionEventUniqueName].Meta.Labels["nuclio.io/function-name"] + uniqueSuffix
	}
	projectConfig.FunctionEvents = functionEvents

	apiGateways := map[string]*platform.APIGatewayConfig{}
	for index, apiGatwayName := range apiGatewayNames {
		apiGatewayUniqueName := apiGatwayName + uniqueSuffix
		apiGateways[apiGatewayUniqueName] = projectConfig.APIGateways[apiGatwayName]
		apiGateways[apiGatewayUniqueName].Meta.Name = apiGatewayUniqueName
		apiGateways[apiGatewayUniqueName].Meta.Namespace = suite.namespace
		apiGateways[apiGatewayUniqueName].Spec.Upstreams = []platform.APIGatewayUpstreamSpec{
			{
				Kind: platform.APIGatewayUpstreamKindNuclioFunction,
				Nucliofunction: &platform.NuclioFunctionAPIGatewaySpec{
					Name: functionNames[index] + uniqueSuffix,
				},
			},
		}
	}
	projectConfig.APIGateways = apiGateways

	projectConfigYaml, err := yaml.Marshal(projectConfig)
	suite.Require().NoError(err)

	// write exported function config to temp file
	tempFile, err := ioutil.TempFile("", "project-import.*.json")
	suite.Require().NoError(err)

	_, err = tempFile.Write(projectConfigYaml)
	suite.Require().NoError(err)

	return tempFile.Name()
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
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"get", "project", projectName}, nil, false)
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

	// wait until able to get the function
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"get", "function", functionName}, nil, false)
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

func (suite *projectExportImportTestSuite) createAPIGateway(apiGatewayName, functionName, projectName string) {
	namedArgs := map[string]string{
		"function": functionName,
		"project":  projectName,
		"host":     fmt.Sprintf("host-%s", apiGatewayName),
	}

	err := suite.ExecuteNuctl([]string{
		"create",
		"apigateway",
		apiGatewayName,
	}, namedArgs)

	suite.Require().NoError(err)
}

func (suite *projectExportImportTestSuite) assertProjectImported(projectName string) {

	// reset output buffer for reading the nex output cleanly
	suite.outputBuffer.Reset()
	err := suite.RetryExecuteNuctlUntilSuccessful([]string{"get", "project", projectName}, map[string]string{
		"output": common.OutputFormatYAML,
	}, false)
	suite.Require().NoError(err)

	project := platform.ProjectConfig{}
	err = yaml.Unmarshal(suite.outputBuffer.Bytes(), &project)
	suite.Require().NoError(err)

	suite.Assert().Equal(projectName, project.Meta.Name)
}

func (suite *projectExportImportTestSuite) assertFunctionEventExistenceByFunction(functionEventDisplayName,
	functionName string) string {

	// reset output buffer for reading the nex output cleanly
	suite.outputBuffer.Reset()
	err := suite.RetryExecuteNuctlUntilSuccessful([]string{"get", "functionevent"}, map[string]string{
		"output":   common.OutputFormatYAML,
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

func (suite *projectExportImportTestSuite) getExportedProject(projectName string,
	positionalArgs []string) *command.ImportProjectConfig {

	// reset output buffer for reading the nex output cleanly
	suite.outputBuffer.Reset()

	// export the project
	exportProjectPositionalArgs := []string{"export", "project", projectName, "--verbose"}
	exportProjectPositionalArgs = append(exportProjectPositionalArgs, positionalArgs...)
	err := suite.RetryExecuteNuctlUntilSuccessful(exportProjectPositionalArgs,
		nil,
		false)
	suite.Require().NoError(err)

	exportedProjectConfig := &command.ImportProjectConfig{}
	err = yaml.Unmarshal(suite.outputBuffer.Bytes(), &exportedProjectConfig)
	suite.Require().NoError(err)

	return exportedProjectConfig
}

func TestProjectTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(projectGetTestSuite))
	suite.Run(t, new(projectDeleteTestSuite))
	suite.Run(t, new(projectExportImportTestSuite))
}
