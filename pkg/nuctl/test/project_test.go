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
	"github.com/nuclio/errors"
	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
)

type projectGetTestSuite struct {
	Suite
}

func (suite *projectGetTestSuite) TestGet() {
	numOfProjects := 3
	projectNames := make([]string, numOfProjects)

	// get with nothing created - should pass
	err := suite.ExecuteNuctl([]string{"get", "project"}, nil)
	suite.Require().NoError(err)

	for projectIdx := 0; projectIdx < numOfProjects; projectIdx++ {
		projectName := fmt.Sprintf("get-test-project-%s-%d", xid.New().String(), projectIdx)

		// add project name to list
		projectNames[projectIdx] = projectName

		namedArgs := map[string]string{
			"description": fmt.Sprintf("description-%d", projectIdx),
		}

		// create project
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
			suite.ExecuteNuctl([]string{"delete", "project", projectName}, nil) // nolint: errcheck

		}(projectName)
	}

	err = suite.ExecuteNuctl([]string{"get", "project"}, nil)
	suite.Require().NoError(err)

	// find function names in get result
	suite.findPatternsInOutput(projectNames, nil)

	// delete the second project
	err = suite.ExecuteNuctl([]string{"delete", "project", projectNames[1], "--verbose"}, nil)
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

func (suite *projectExportImportTestSuite) TestDeleteProject() {
	for _, testCase := range []struct {
		name            string
		importFunctions bool
		strategy        platform.DeleteProjectStrategy
		expectedError   error
	}{
		{
			name:     "DeleteProjectCascading",
			strategy: platform.DeleteProjectStrategyCascading,
		},
		{
			name:     "DeleteProjectRestricted",
			strategy: platform.DeleteProjectStrategyRestricted,
		},
		{
			name:            "DeleteProjectWithFunctions",
			importFunctions: true,
			strategy:        platform.DeleteProjectStrategyCascading,
		},
		{
			name:            "FailDeleteProjectWithFunctions",
			importFunctions: true,
			strategy:        platform.DeleteProjectStrategyRestricted,
			expectedError:   platform.ErrProjectContainsFunctions,
		},
	} {
		suite.Run(testCase.name, func() {
			var functionNames []string
			uniqueID := xid.New().String()
			projectName := "delete-project-test-" + uniqueID

			// create a project
			err := suite.ExecuteNuctl([]string{
				"create",
				"project",
				projectName,
			}, nil)
			suite.Require().NoError(err)

			if testCase.importFunctions {
				functionNames = append(functionNames,
					"test-function-a-"+uniqueID,
					"test-function-b-"+uniqueID)
				suite.createImportedFunctions(projectName, functionNames...)

				// ensure deleted
				defer func() {
					for _, functionName := range functionNames {
						suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck
					}
				}()
			}

			// delete project
			err = suite.ExecuteNuctl([]string{"delete", "project", projectName, "--wait"}, map[string]string{
				"strategy": string(testCase.strategy),
			})
			if testCase.expectedError != nil {
				suite.Require().EqualError(errors.RootCause(err), testCase.expectedError.Error())
				suite.Require().Error(err)
				return
			}

			suite.Require().NoError(err)
			if testCase.importFunctions {

				// ensure functions were deleted
				for _, functionName := range functionNames {
					err = suite.RetryExecuteNuctlUntilSuccessful([]string{"get", "function", functionName}, nil, true)
					suite.Require().NoError(err)
				}

			}
		})
	}
}

func (suite *projectExportImportTestSuite) createImportedFunctions(projectName string, functionNames ...string) {
	functionToImportTemplate := `%[1]s:
  metadata:
    labels:
      nuclio.io/project-name: %[2]s
    annotations:
      skip-build: "true"
      skip-deploy: "true"
    name: %[1]s
  spec:
    build:
      codeEntryType: sourceCode
      functionSourceCode: ZWNobyAidGVzdDEi
      noBaseImagesPull: true
    handler: main.sh
    runtime: shell`

	functionsToImportEncoded := ""
	for _, functionName := range functionNames {
		functionToImportEncoded := fmt.Sprintf(functionToImportTemplate, functionName, projectName)
		functionsToImportEncoded += fmt.Sprintf("\n%s", functionToImportEncoded)
	}
	suite.inputBuffer = *bytes.NewBufferString(functionsToImportEncoded)

	// import the project
	err := suite.ExecuteNuctl([]string{
		"import",
		"functions",
		"--verbose",
	}, nil)
	suite.Require().NoError(err)

	// wait for functions to be imported
	for _, functionName := range functionNames {
		suite.waitForFunctionState(functionName, functionconfig.FunctionStateImported)
	}

	// reset buffer
	suite.inputBuffer.Reset()

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

	exportedProjectConfig := suite.exportProject(projectName, []string{})

	suite.Assert().Equal(exportedProjectConfig.Project.Meta.Name, projectName)
	suite.Assert().Equal(exportedProjectConfig.Functions[functionName].Meta.Name, functionName)
	suite.Assert().Equal(exportedProjectConfig.FunctionEvents[functionEventName].Meta.Name, functionEventName)

	if apiGatewaysEnabled {
		suite.Assert().Equal(exportedProjectConfig.APIGateways[apiGatewayName].Meta.Name, apiGatewayName)
		suite.Assert().Equal(exportedProjectConfig.APIGateways[apiGatewayName].Spec.Host, fmt.Sprintf("host-%s", apiGatewayName))
		suite.Assert().Equal(exportedProjectConfig.APIGateways[apiGatewayName].Spec.Upstreams[0].Kind, platform.APIGatewayUpstreamKindNuclioFunction)
		suite.Assert().Equal(exportedProjectConfig.APIGateways[apiGatewayName].Spec.Upstreams[0].NuclioFunction.Name, functionName)
	}
}

func (suite *projectExportImportTestSuite) TestImportProjectSkipBySelectors() {
	for _, testCase := range []struct {
		name                      string
		projectImportConfig       *command.ProjectImportConfig
		encodedSkipLabelSelectors string
		skipImportingProject      bool
		expectedFailure           bool
	}{

		// skip import
		{
			name: "SkipImportLabelSelectorsSanity",
			projectImportConfig: &command.ProjectImportConfig{
				Project: &platform.ProjectConfig{
					Meta: platform.ProjectMeta{
						Name:      "test-project-" + xid.New().String(),
						Namespace: suite.namespace,
						Labels: map[string]string{
							"skip": "me",
						},
					},
				},
			},
			encodedSkipLabelSelectors: "skip=me",
			skipImportingProject:      true,
		},
		{
			name: "SkipImportInLabelSelectors",
			projectImportConfig: &command.ProjectImportConfig{
				Project: &platform.ProjectConfig{
					Meta: platform.ProjectMeta{
						Name:      "test-project-" + xid.New().String(),
						Namespace: suite.namespace,
						Labels: map[string]string{
							"skip": "b",
						},
					},
				},
			},
			encodedSkipLabelSelectors: "skip in (a,b,c)",
			skipImportingProject:      true,
		},
		{
			name: "ComplexLabelNameValue",
			projectImportConfig: &command.ProjectImportConfig{
				Project: &platform.ProjectConfig{
					Meta: platform.ProjectMeta{
						Name:      "test-project-" + xid.New().String(),
						Namespace: suite.namespace,
						Labels: map[string]string{
							"app.kubernetes.io/part-of": "nuclio-test",
						},
					},
				},
			},
			encodedSkipLabelSelectors: "app.kubernetes.io/part-of=nuclio-test",
			skipImportingProject:      true,
		},

		// import
		{
			name: "ImportSanity",
			projectImportConfig: &command.ProjectImportConfig{
				Project: &platform.ProjectConfig{
					Meta: platform.ProjectMeta{
						Name:      "test-project-" + xid.New().String(),
						Namespace: suite.namespace,
						Labels: map[string]string{
							"whatever": "ishere",
						},
					},
				},
			},
		},
		{
			name: "ImportLabelSelectorsMissMatch",
			projectImportConfig: &command.ProjectImportConfig{
				Project: &platform.ProjectConfig{
					Meta: platform.ProjectMeta{
						Name:      "test-project-" + xid.New().String(),
						Namespace: suite.namespace,
						Labels: map[string]string{
							"donot-skip": "me",
						},
					},
				},
			},
			encodedSkipLabelSelectors: "skip=me",
		},

		// export failure
		{
			name: "InvalidLabelSelectorInput",
			projectImportConfig: &command.ProjectImportConfig{
				Project: &platform.ProjectConfig{
					Meta: platform.ProjectMeta{
						Name:      "test-project-" + xid.New().String(),
						Namespace: suite.namespace,
						Labels: map[string]string{
							"whatever": "ishere",
						},
					},
				},
			},
			encodedSkipLabelSelectors: "invalid@label^selector!",
			expectedFailure:           true,
		},
	} {
		suite.Run(testCase.name, func() {
			projectName := testCase.projectImportConfig.Project.Meta.Name

			var encodedProjectImportConfig []byte
			encodedProjectImportConfig, err := yaml.Marshal(testCase.projectImportConfig)
			suite.Require().NoError(err)

			// import project from stdin
			suite.inputBuffer = *bytes.NewBuffer(encodedProjectImportConfig)

			// import
			err = suite.ExecuteNuctl([]string{"import", "project", "--verbose"}, map[string]string{
				"skip-label-selectors": testCase.encodedSkipLabelSelectors,
			})

			// delete leftovers
			defer suite.ExecuteNuctl([]string{"delete", "project", projectName}, nil) // nolint: errcheck

			if testCase.expectedFailure {
				suite.Require().Error(err)
				return
			}

			// assertions
			suite.Require().NoError(err)

			if testCase.skipImportingProject {
				err = suite.ExecuteNuctl([]string{"get", "project", projectName}, nil)
				suite.Require().Error(err)
				suite.Require().EqualError(err, "No projects found")

			} else {
				suite.assertProjectImported(projectName)
			}
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
		[]string{apiGateway1Name, apiGateway2Name})
	defer os.Remove(uniqueProjectConfigPath) // nolint: errcheck

	function1Name += uniqueSuffix
	function2Name += uniqueSuffix
	function1EventDisplayName += uniqueSuffix
	function2EventDisplayName += uniqueSuffix
	apiGateway1Name += uniqueSuffix
	apiGateway2Name += uniqueSuffix

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
	suite.Require().Contains(err.Error(), "Failed to resolve the project-configuration body.")
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

	function1Name += uniqueSuffix
	function2Name += uniqueSuffix
	function1EventDisplayName += uniqueSuffix
	function2EventDisplayName += uniqueSuffix

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

	projectImportConfig := &command.ProjectImportConfig{}
	err = yaml.Unmarshal(file, projectImportConfig)
	suite.Require().NoError(err)

	projectImportConfig.Project.Meta.Name += uniqueSuffix
	projectImportConfig.Project.Meta.Namespace = suite.namespace
	functions := map[string]*functionconfig.Config{}
	for _, functionName := range functionNames {
		functionUniqueName := functionName + uniqueSuffix
		functions[functionUniqueName] = projectImportConfig.Functions[functionName]
		functions[functionUniqueName].Meta.Name = functionName + uniqueSuffix
		functions[functionUniqueName].Meta.Namespace = suite.namespace
		functions[functionUniqueName].Meta.Labels["nuclio.io/project-name"] += uniqueSuffix
	}
	projectImportConfig.Functions = functions

	functionEvents := map[string]*platform.FunctionEventConfig{}
	for _, functionEventName := range functionEventNames {
		functionEventUniqueName := functionEventName + uniqueSuffix
		functionEvents[functionEventUniqueName] = projectImportConfig.FunctionEvents[functionEventName]
		functionEvents[functionEventUniqueName].Spec.DisplayName = functionEventName + uniqueSuffix
		functionEvents[functionEventUniqueName].Meta.Namespace = suite.namespace
		functionEvents[functionEventUniqueName].Meta.Labels["nuclio.io/function-name"] += uniqueSuffix
	}
	projectImportConfig.FunctionEvents = functionEvents

	apiGateways := map[string]*platform.APIGatewayConfig{}
	for index, apiGatwayName := range apiGatewayNames {
		apiGatewayUniqueName := apiGatwayName + uniqueSuffix
		apiGateways[apiGatewayUniqueName] = projectImportConfig.APIGateways[apiGatwayName]
		apiGateways[apiGatewayUniqueName].Meta.Name = apiGatewayUniqueName
		apiGateways[apiGatewayUniqueName].Meta.Namespace = suite.namespace
		apiGateways[apiGatewayUniqueName].Spec.Upstreams = []platform.APIGatewayUpstreamSpec{
			{
				Kind: platform.APIGatewayUpstreamKindNuclioFunction,
				NuclioFunction: &platform.NuclioFunctionAPIGatewaySpec{
					Name: functionNames[index] + uniqueSuffix,
				},
			},
		}
	}
	projectImportConfig.APIGateways = apiGateways

	projectConfigYaml, err := yaml.Marshal(projectImportConfig)
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

func (suite *projectExportImportTestSuite) exportProject(projectName string,
	positionalArgs []string) *command.ProjectImportConfig {

	// reset output buffer for reading the nex output cleanly
	suite.outputBuffer.Reset()

	// export the project
	exportProjectPositionalArgs := []string{"export", "project", projectName, "--verbose"}
	exportProjectPositionalArgs = append(exportProjectPositionalArgs, positionalArgs...)
	err := suite.RetryExecuteNuctlUntilSuccessful(exportProjectPositionalArgs,
		nil,
		false)
	suite.Require().NoError(err)

	projectImportConfig := &command.ProjectImportConfig{}
	err = yaml.Unmarshal(suite.outputBuffer.Bytes(), &projectImportConfig)
	suite.Require().NoError(err)

	return projectImportConfig
}

func TestProjectTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(projectGetTestSuite))
	suite.Run(t, new(projectDeleteTestSuite))
	suite.Run(t, new(projectExportImportTestSuite))
}
