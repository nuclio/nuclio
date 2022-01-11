//go:build test_unit

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

package abstract

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	mockedplatform "github.com/nuclio/nuclio/pkg/platform/mock"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/google/go-cmp/cmp"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/rs/xid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"k8s.io/api/core/v1"
)

const (
	MultiWorkerFunctionLogsFilePath          = "test/logs_examples/multi_worker"
	PanicFunctionLogsFilePath                = "test/logs_examples/panic"
	GoWithCallStackFunctionLogsFilePath      = "test/logs_examples/go_with_call_stack"
	SpecialSubstringsFunctionLogsFilePath    = "test/logs_examples/special_substrings"
	ConsecutiveDuplicateFunctionLogsFilePath = "test/logs_examples/consecutive_duplicate"
	FunctionLogsFile                         = "function_logs.txt"
	FormattedFunctionLogsFile                = "formatted_function_logs.txt"
	BriefErrorsMessageFile                   = "brief_errors_message.txt"
)

type AbstractPlatformTestSuite struct {
	suite.Suite
	mockedPlatform *mockedplatform.Platform

	Logger           logger.Logger
	DockerClient     dockerclient.Client
	Platform         *Platform
	TestID           string
	Runtime          string
	RuntimeDir       string
	FunctionDir      string
	TempDir          string
	CleanupTemp      bool
	DefaultNamespace string
	ctx              context.Context
}

func (suite *AbstractPlatformTestSuite) SetupSuite() {
	var err error

	common.SetVersionFromEnv()

	suite.DefaultNamespace = "nuclio"

	suite.ctx = context.Background()

	suite.Logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err, "Logger should create successfully")
	suite.initializeMockedPlatform()
}

func (suite *AbstractPlatformTestSuite) SetupTest() {
	suite.TestID = xid.New().String()
}

func (suite *AbstractPlatformTestSuite) initializeMockedPlatform() {
	var err error
	suite.mockedPlatform = &mockedplatform.Platform{}
	suite.Platform, err = NewPlatform(suite.Logger, suite.mockedPlatform, &platformconfig.Config{}, "")
	suite.Require().NoError(err, "Could not create platform")

	suite.Platform.ContainerBuilder, err = containerimagebuilderpusher.NewNop(suite.Logger, nil)
	suite.Require().NoError(err)
}

func (suite *AbstractPlatformTestSuite) TestProjectCreateOptions() {
	for _, testCase := range []struct {
		Name                    string
		CreateProjectOptions    *platform.CreateProjectOptions
		ExpectValidationFailure bool
		ExpectedProjectName     string
	}{

		// happy flows
		{
			Name: "Sanity",
			CreateProjectOptions: &platform.CreateProjectOptions{
				ProjectConfig: &platform.ProjectConfig{
					Meta: platform.ProjectMeta{
						Name: "a-name",
					},
					Spec: platform.ProjectSpec{
						Description: "just a description",
					},
				},
			},
			ExpectedProjectName: "a-name",
		},

		// bad flows
		{
			Name: "InvalidName",
			CreateProjectOptions: &platform.CreateProjectOptions{
				ProjectConfig: &platform.ProjectConfig{
					Meta: platform.ProjectMeta{
						Name: "invalid project name ## .. %%",
					},
				},
			},
			ExpectValidationFailure: true,
		},
		{
			Name: "EmptyName",
			CreateProjectOptions: &platform.CreateProjectOptions{
				ProjectConfig: &platform.ProjectConfig{
					Meta: platform.ProjectMeta{
						Name: "",
					},
				},
			},
			ExpectValidationFailure: true,
		},
	} {
		suite.Run(testCase.Name, func() {
			defer func() {
				suite.initializeMockedPlatform()
			}()
			err := suite.Platform.EnrichCreateProjectConfig(testCase.CreateProjectOptions)
			suite.Require().NoError(err)
			err = suite.Platform.ValidateProjectConfig(testCase.CreateProjectOptions.ProjectConfig)
			if testCase.ExpectValidationFailure {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
				suite.Require().Equal(testCase.ExpectedProjectName,
					testCase.CreateProjectOptions.ProjectConfig.Meta.Name)
			}
		})
	}
}

func (suite *AbstractPlatformTestSuite) TestValidationFailOnMalformedIngressesStructure() {
	functionConfig := functionconfig.NewConfig()
	functionConfig.Meta.Name = "f1"
	functionConfig.Meta.Namespace = "default"
	functionConfig.Meta.Labels = map[string]string{
		"nuclio.io/project-name": platform.DefaultProjectName,
	}

	for _, testCase := range []struct {
		Triggers      map[string]functionconfig.Trigger
		ExpectedError string
	}{

		// test malformed ingresses structure
		{
			Triggers: map[string]functionconfig.Trigger{
				"http-trigger": {
					Kind: "http",
					Attributes: map[string]interface{}{
						"ingresses": "I should be a map and not a string",
					},
				},
			},
			ExpectedError: "Malformed structure for ingresses in trigger 'http-trigger' (expects a map)",
		},

		// test malformed specific ingress structure
		{
			Triggers: map[string]functionconfig.Trigger{
				"http-trigger": {
					Kind: "http",
					Attributes: map[string]interface{}{
						"ingresses": map[string]interface{}{
							"0": map[string]interface{}{
								"host":  "some-host",
								"paths": []string{"/"},
							},
							"malformed-ingress": "I should be a map and not a string",
						},
					},
				},
			},
			ExpectedError: "Malformed structure for ingress 'malformed-ingress' in trigger 'http-trigger'",
		},

		// test good flow (expecting no error)
		{
			Triggers: map[string]functionconfig.Trigger{
				"http-trigger": {
					Kind: "http",
					Attributes: map[string]interface{}{
						"ingresses": map[string]interface{}{
							"0": map[string]interface{}{
								"host":  "some-host",
								"paths": []string{"/"},
							},
						},
					},
				},
			},
			ExpectedError: "",
		},
	} {

		suite.mockedPlatform.On("GetProjects", mock.Anything, &platform.GetProjectsOptions{
			Meta: platform.ProjectMeta{
				Name:      platform.DefaultProjectName,
				Namespace: "default",
			},
		}).Return([]platform.Project{
			&platform.AbstractProject{},
		}, nil).Once()

		// set test triggers
		functionConfig.Spec.Triggers = testCase.Triggers

		// enrich
		err := suite.Platform.EnrichFunctionConfig(suite.ctx, functionConfig)
		suite.Require().NoError(err)

		// validate
		err = suite.Platform.ValidateFunctionConfig(suite.ctx, functionConfig)
		if testCase.ExpectedError != "" {
			suite.Assert().Error(err)
			suite.Assert().Equal(testCase.ExpectedError, errors.RootCause(err).Error())
		} else {
			suite.Assert().NoError(err)
		}
	}
}

func (suite *AbstractPlatformTestSuite) TestValidateDeleteFunctionOptions() {
	for _, testCase := range []struct {
		name                  string
		existingFunctions     []platform.Function
		deleteFunctionOptions *platform.DeleteFunctionOptions
		shouldFailValidation  bool
	}{
		{
			name: "sanity",
			existingFunctions: []platform.Function{
				&platform.AbstractFunction{
					Logger:   suite.Logger,
					Platform: suite.Platform.platform,
					Config: functionconfig.Config{
						Meta: functionconfig.Meta{
							Name: "existing",
						},
					},
					Status: functionconfig.Status{
						State: functionconfig.FunctionStateReady,
					},
				},
			},
			deleteFunctionOptions: &platform.DeleteFunctionOptions{
				FunctionConfig: functionconfig.Config{
					Meta: functionconfig.Meta{
						Name: "existing",
					},
				},
			},
		},
		{
			name: "idempotent-non-existing-function",
			deleteFunctionOptions: &platform.DeleteFunctionOptions{
				FunctionConfig: functionconfig.Config{
					Meta: functionconfig.Meta{
						Name:            "not-existing",
						ResourceVersion: "",
					},
					Spec: functionconfig.Spec{},
				},
			},
		},

		{
			name: "matching-resource-validation",
			existingFunctions: []platform.Function{
				&platform.AbstractFunction{
					Logger:   suite.Logger,
					Platform: suite.Platform.platform,
					Config: functionconfig.Config{
						Meta: functionconfig.Meta{
							Name:            "existing",
							ResourceVersion: "1",
						},
					},
					Status: functionconfig.Status{
						State: functionconfig.FunctionStateReady,
					},
				},
			},
			deleteFunctionOptions: &platform.DeleteFunctionOptions{
				FunctionConfig: functionconfig.Config{
					Meta: functionconfig.Meta{
						Name:            "existing",
						ResourceVersion: "1",
					},
				},
			},
		},

		{
			name: "function-being-provisioned",
			existingFunctions: []platform.Function{
				&platform.AbstractFunction{
					Logger:   suite.Logger,
					Platform: suite.Platform.platform,
					Config: functionconfig.Config{
						Meta: functionconfig.Meta{
							Name: "existing",
						},
					},
					Status: functionconfig.Status{
						State: functionconfig.FunctionStateBuilding,
					},
				},
			},
			deleteFunctionOptions: &platform.DeleteFunctionOptions{
				FunctionConfig: functionconfig.Config{
					Meta: functionconfig.Meta{
						Name: "existing",
					},
				},
			},
			shouldFailValidation: true,
		},

		{
			name: "fail-stale-resource-version",
			existingFunctions: []platform.Function{
				&platform.AbstractFunction{
					Logger:   suite.Logger,
					Platform: suite.Platform.platform,
					Config: functionconfig.Config{
						Meta: functionconfig.Meta{
							Name:            "existing",
							ResourceVersion: "2",
						},
					},
				},
			},
			deleteFunctionOptions: &platform.DeleteFunctionOptions{
				FunctionConfig: functionconfig.Config{
					Meta: functionconfig.Meta{
						Name:            "existing",
						ResourceVersion: "1",
					},
				},
			},
			shouldFailValidation: true,
		},
	} {
		suite.Run(testCase.name, func() {
			suite.mockedPlatform.
				On("GetFunctions", suite.ctx, &platform.GetFunctionsOptions{
					Name:      testCase.deleteFunctionOptions.FunctionConfig.Meta.Name,
					Namespace: testCase.deleteFunctionOptions.FunctionConfig.Meta.Namespace,
				}).
				Return(testCase.existingFunctions, nil).
				Once()

			_, err := suite.Platform.ValidateDeleteFunctionOptions(suite.ctx, testCase.deleteFunctionOptions)

			suite.mockedPlatform.AssertExpectations(suite.T())

			if testCase.shouldFailValidation {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
			}
		})
	}
}

func (suite *AbstractPlatformTestSuite) TestValidateDeleteProjectOptions() {
	for _, testCase := range []struct {
		name                 string
		deleteProjectOptions *platform.DeleteProjectOptions
		existingProjects     []platform.Project
		existingFunctions    []platform.Function
		existingAPIGateway   []platform.APIGateway
		expectedFailure      bool
	}{
		{
			name: "Delete",
			deleteProjectOptions: &platform.DeleteProjectOptions{
				Meta: platform.ProjectMeta{
					Name:      "something",
					Namespace: suite.DefaultNamespace,
				},
			},
			existingProjects: make([]platform.Project, 1),
		},
		{
			name: "DeleteCascading",
			deleteProjectOptions: &platform.DeleteProjectOptions{
				Meta: platform.ProjectMeta{
					Name:      "something",
					Namespace: suite.DefaultNamespace,
				},
				Strategy: platform.DeleteProjectStrategyCascading,
			},
			existingProjects: make([]platform.Project, 1),
		},
		{
			name: "DeleteNonExistingProject",
			deleteProjectOptions: &platform.DeleteProjectOptions{
				Meta: platform.ProjectMeta{
					Name:      "something",
					Namespace: suite.DefaultNamespace,
				},
			},
		},

		// bad flows
		{
			name: "ProjectNameEmptyFail",
			deleteProjectOptions: &platform.DeleteProjectOptions{
				Meta: platform.ProjectMeta{
					Namespace: suite.DefaultNamespace,
					Name:      "",
				},
			},
			expectedFailure: true,
		},
		{
			name: "FailDeletingDefaultProject",
			deleteProjectOptions: &platform.DeleteProjectOptions{
				Meta: platform.ProjectMeta{
					Namespace: suite.DefaultNamespace,
					Name:      platform.DefaultProjectName,
				},
			},
			expectedFailure: true,
		},
		{
			name: "FailDeletingProjectWithFunctions",
			deleteProjectOptions: &platform.DeleteProjectOptions{
				Meta: platform.ProjectMeta{
					Name:      "something",
					Namespace: suite.DefaultNamespace,
				},
			},
			existingProjects:  make([]platform.Project, 1),
			existingFunctions: make([]platform.Function, 1),
			expectedFailure:   true,
		},
		{
			name: "FailDeletingProjectWithAPIGateways",
			deleteProjectOptions: &platform.DeleteProjectOptions{
				Meta: platform.ProjectMeta{
					Name:      "something",
					Namespace: suite.DefaultNamespace,
				},
			},
			existingProjects:   make([]platform.Project, 1),
			existingAPIGateway: make([]platform.APIGateway, 1),
			expectedFailure:    true,
		},
	} {

		suite.Run(testCase.name, func() {
			defer func() {
				suite.initializeMockedPlatform()
			}()

			suite.mockedPlatform.
				On("GetProjects", suite.ctx, mock.Anything).
				Return(testCase.existingProjects, nil).
				Once()

			if testCase.deleteProjectOptions.Strategy == "" {
				testCase.deleteProjectOptions.Strategy = platform.DeleteProjectStrategyRestricted
			}

			if len(testCase.existingProjects) > 0 {
				suite.mockedPlatform.
					On("GetFunctions", suite.ctx, &platform.GetFunctionsOptions{
						Namespace: suite.DefaultNamespace,
						Labels:    fmt.Sprintf("nuclio.io/project-name=%s", testCase.deleteProjectOptions.Meta.Name),
					}).
					Return(testCase.existingFunctions, nil).
					Once()

				suite.mockedPlatform.
					On("GetAPIGateways", suite.ctx, &platform.GetAPIGatewaysOptions{
						Namespace: suite.DefaultNamespace,
						Labels:    fmt.Sprintf("nuclio.io/project-name=%s", testCase.deleteProjectOptions.Meta.Name),
					}).
					Return(testCase.existingAPIGateway, nil).
					Once()
			} else {

				// do not get validations if project does not exists
				suite.mockedPlatform.AssertNotCalled(suite.T(), "GetFunctions", mock.Anything)
				suite.mockedPlatform.AssertNotCalled(suite.T(), "GetAPIGateways", mock.Anything)
			}

			err := suite.Platform.ValidateDeleteProjectOptions(suite.ctx, testCase.deleteProjectOptions)
			if testCase.expectedFailure {
				suite.Require().Error(err)
				return
			}
			suite.Require().NoError(err)
		})
	}
}

func (suite *AbstractPlatformTestSuite) TestGetProjectResources() {
	for _, testCase := range []struct {
		name              string
		functions         []platform.Function
		apiGateways       []platform.APIGateway
		getFunctionsError error
		expectedFailure   bool
	}{
		{
			name:        "GetProjectResources",
			functions:   make([]platform.Function, 2),
			apiGateways: make([]platform.APIGateway, 2),
		},
		{
			name:        "GetProjectResourcesNoResources",
			functions:   nil,
			apiGateways: nil,
		},
		{
			name:              "GetProjectResourcesFail",
			functions:         nil,
			apiGateways:       nil,
			getFunctionsError: errors.New("Something bad happened"),
			expectedFailure:   true,
		},
	} {
		suite.Run(testCase.name, func() {
			defer func() {
				suite.initializeMockedPlatform()
			}()

			suite.mockedPlatform.
				On("GetAPIGateways", suite.ctx, mock.Anything).
				Return(testCase.apiGateways, nil).Once()

			suite.mockedPlatform.
				On("GetFunctions", suite.ctx, mock.Anything).
				Return(testCase.functions, testCase.getFunctionsError).Once()

			projectFunctions, projectAPIGateways, err := suite.Platform.GetProjectResources(suite.ctx,
				&platform.ProjectMeta{
					Namespace: suite.DefaultNamespace,
					Name:      xid.New().String(),
				})
			if testCase.expectedFailure {
				suite.Require().Error(err)
				return
			}
			suite.Require().NoError(err)
			suite.Require().Empty(cmp.Diff(projectFunctions, testCase.functions))
			suite.Require().Empty(cmp.Diff(projectAPIGateways, testCase.apiGateways))
		})
	}
}

func (suite *AbstractPlatformTestSuite) TestValidateCreateFunctionOptionsAgainstExistingFunctionConfig() {
	for _, testCase := range []struct {
		name                    string
		existingFunction        *functionconfig.ConfigWithStatus
		createFunctionOptions   *platform.CreateFunctionOptions
		expectValidationFailure bool
	}{
		{
			name:             "sanityCreate",
			existingFunction: nil,
			createFunctionOptions: &platform.CreateFunctionOptions{
				FunctionConfig: functionconfig.Config{},
			},
		},
		{
			name: "sanityUpdate",
			existingFunction: &functionconfig.ConfigWithStatus{
				Status: functionconfig.Status{
					State: functionconfig.FunctionStateReady,
				},
			},
			createFunctionOptions: &platform.CreateFunctionOptions{
				FunctionConfig: functionconfig.Config{},
			},
		},

		// bad flows
		{
			name:             "mustBuildWhenCreatingFunction",
			existingFunction: nil,
			createFunctionOptions: &platform.CreateFunctionOptions{
				FunctionConfig: functionconfig.Config{
					Spec: functionconfig.Spec{
						Build: functionconfig.Build{
							Mode: functionconfig.NeverBuild,
						},
					},
				},
			},
			expectValidationFailure: true,
		},
		{
			name: "staleResourceVersion",
			existingFunction: &functionconfig.ConfigWithStatus{
				Config: functionconfig.Config{
					Meta: functionconfig.Meta{
						ResourceVersion: "1",
					},
				},
				Status: functionconfig.Status{
					State: functionconfig.FunctionStateReady,
				},
			},
			createFunctionOptions: &platform.CreateFunctionOptions{
				FunctionConfig: functionconfig.Config{
					Meta: functionconfig.Meta{
						ResourceVersion: "2",
					},
				},
			},
			expectValidationFailure: true,
		},
		{
			name: "disablingFunctionWithAPIGateways",
			existingFunction: &functionconfig.ConfigWithStatus{
				Config: functionconfig.Config{},
				Status: functionconfig.Status{
					State:       functionconfig.FunctionStateReady,
					APIGateways: []string{"x", "y", "z"},
				},
			},
			createFunctionOptions: &platform.CreateFunctionOptions{
				FunctionConfig: functionconfig.Config{
					Spec: functionconfig.Spec{
						Disable: true,
					},
				},
			},
			expectValidationFailure: true,
		},
		{
			name: "functionBeingProvisioned",
			existingFunction: &functionconfig.ConfigWithStatus{
				Status: functionconfig.Status{
					State: functionconfig.FunctionStateBuilding,
				},
			},
			createFunctionOptions:   &platform.CreateFunctionOptions{},
			expectValidationFailure: true,
		},
	} {
		suite.Run(testCase.name, func() {
			err := suite.Platform.ValidateCreateFunctionOptionsAgainstExistingFunctionConfig(suite.ctx,
				testCase.existingFunction,
				testCase.createFunctionOptions)
			if testCase.expectValidationFailure {
				suite.Require().Error(err)
				return
			}
			suite.Require().NoError(err)
		})
	}
}

func (suite *AbstractPlatformTestSuite) TestResolveProjectNameFromLabelsStr() {
	for _, testCase := range []struct {
		name                string
		label               string
		expectedProjectName string
		expectedError       bool
	}{
		{
			name:                "empty",
			label:               "",
			expectedProjectName: "",
		},
		{
			name:                "with-project",
			label:               fmt.Sprintf("%s=x", common.NuclioResourceLabelKeyProjectName),
			expectedProjectName: "x",
		},
		{
			name:                "other-label",
			label:               "something-else=y",
			expectedProjectName: "",
		},
		{
			name:          "with-malformed-project",
			label:         "/%12",
			expectedError: true,
		},
	} {
		suite.Run(testCase.name, func() {
			projectName, err := suite.Platform.ResolveProjectNameFromLabelsStr(testCase.label)
			if testCase.expectedError {
				suite.Require().Error(err)
				return
			}
			suite.Require().NoError(err)
			suite.Require().Equal(testCase.expectedProjectName, projectName)

		})
	}
}

// Test function with invalid min max replicas
func (suite *AbstractPlatformTestSuite) TestMinMaxReplicas() {
	zero := 0
	one := 1
	two := 2
	for idx, MinMaxReplicas := range []struct {
		MinReplicas          *int
		MaxReplicas          *int
		ExpectedMinReplicas  *int
		ExpectedMaxReplicas  *int
		shouldFailValidation bool
	}{
		{MinReplicas: nil, MaxReplicas: nil, ExpectedMinReplicas: nil, ExpectedMaxReplicas: nil, shouldFailValidation: false},
		{MinReplicas: nil, MaxReplicas: &zero, shouldFailValidation: true},
		{MinReplicas: nil, MaxReplicas: &one, ExpectedMinReplicas: &one, ExpectedMaxReplicas: &one, shouldFailValidation: false},
		{MinReplicas: nil, MaxReplicas: &two, ExpectedMinReplicas: &two, ExpectedMaxReplicas: &two, shouldFailValidation: false},

		{MinReplicas: &zero, MaxReplicas: nil, shouldFailValidation: true},
		{MinReplicas: &zero, MaxReplicas: &zero, shouldFailValidation: true},
		{MinReplicas: &zero, MaxReplicas: &one, ExpectedMinReplicas: &zero, ExpectedMaxReplicas: &one, shouldFailValidation: false},
		{MinReplicas: &zero, MaxReplicas: &two, ExpectedMinReplicas: &zero, ExpectedMaxReplicas: &two, shouldFailValidation: false},

		{MinReplicas: &one, MaxReplicas: nil, ExpectedMinReplicas: &one, ExpectedMaxReplicas: &one, shouldFailValidation: false},
		{MinReplicas: &one, MaxReplicas: &zero, shouldFailValidation: true},
		{MinReplicas: &one, MaxReplicas: &one, ExpectedMinReplicas: &one, ExpectedMaxReplicas: &one, shouldFailValidation: false},
		{MinReplicas: &one, MaxReplicas: &two, ExpectedMinReplicas: &one, ExpectedMaxReplicas: &two, shouldFailValidation: false},

		{MinReplicas: &two, MaxReplicas: nil, ExpectedMinReplicas: &two, ExpectedMaxReplicas: &two, shouldFailValidation: false},
		{MinReplicas: &two, MaxReplicas: &zero, shouldFailValidation: true},
		{MinReplicas: &two, MaxReplicas: &one, shouldFailValidation: true},
		{MinReplicas: &two, MaxReplicas: &two, ExpectedMinReplicas: &two, ExpectedMaxReplicas: &two, shouldFailValidation: false},
	} {

		suite.mockedPlatform.On("GetProjects", suite.ctx, &platform.GetProjectsOptions{
			Meta: platform.ProjectMeta{
				Name:      platform.DefaultProjectName,
				Namespace: "default",
			},
		}).Return([]platform.Project{
			&platform.AbstractProject{},
		}, nil).Once()

		// name it with index and shift with 65 to get A as first letter
		functionName := string(rune(idx + 65))
		functionConfig := *functionconfig.NewConfig()

		createFunctionOptions := &platform.CreateFunctionOptions{
			Logger:         suite.Logger,
			FunctionConfig: functionConfig,
		}

		createFunctionOptions.FunctionConfig.Meta.Name = functionName
		createFunctionOptions.FunctionConfig.Meta.Labels = map[string]string{
			"nuclio.io/project-name": platform.DefaultProjectName,
		}
		createFunctionOptions.FunctionConfig.Spec.MinReplicas = MinMaxReplicas.MinReplicas
		createFunctionOptions.FunctionConfig.Spec.MaxReplicas = MinMaxReplicas.MaxReplicas
		suite.Logger.DebugWith("Checking function ", "functionName", functionName)

		err := suite.Platform.EnrichFunctionConfig(suite.ctx, &createFunctionOptions.FunctionConfig)
		suite.Require().NoError(err, "Failed to enrich function config")

		err = suite.Platform.ValidateFunctionConfig(suite.ctx, &createFunctionOptions.FunctionConfig)
		if MinMaxReplicas.shouldFailValidation {
			suite.Error(err, "Validation should fail")
			suite.Logger.DebugWith("Validation failed as expected ", "functionName", functionName)
			continue
		}
		suite.Require().NoError(err, "Failed to validate function")
		functionConfigSpec := createFunctionOptions.FunctionConfig.Spec

		if MinMaxReplicas.ExpectedMinReplicas != nil {
			suite.Assert().Equal(*MinMaxReplicas.ExpectedMinReplicas, *functionConfigSpec.MinReplicas)
		} else {
			suite.Assert().Nil(functionConfigSpec.MinReplicas)
		}

		if MinMaxReplicas.ExpectedMaxReplicas != nil {
			suite.Assert().Equal(*MinMaxReplicas.ExpectedMaxReplicas, *functionConfigSpec.MaxReplicas)
		} else {
			suite.Assert().Nil(functionConfigSpec.MaxReplicas)
		}
		suite.Logger.DebugWith("Validation passed successfully", "functionName", functionName)
	}
}

func (suite *AbstractPlatformTestSuite) TestEnrichAndValidateFunctionTriggers() {
	for idx, testCase := range []struct {
		triggers                 map[string]functionconfig.Trigger
		expectedEnrichedTriggers map[string]functionconfig.Trigger
		shouldFailValidation     bool
	}{

		// enrich maxWorkers to 1
		// enrich name from key
		{
			triggers: map[string]functionconfig.Trigger{
				"some-trigger": {
					Kind: "http",
				},
			},
			expectedEnrichedTriggers: map[string]functionconfig.Trigger{
				"some-trigger": {
					Kind:       "http",
					MaxWorkers: 1,
					Name:       "some-trigger",
				},
			},
		},

		// enrich with default http trigger
		{
			triggers: nil,
			expectedEnrichedTriggers: func() map[string]functionconfig.Trigger {
				defaultHTTPTrigger := functionconfig.GetDefaultHTTPTrigger()
				return map[string]functionconfig.Trigger{
					defaultHTTPTrigger.Name: defaultHTTPTrigger,
				}
			}(),
		},

		// do not allow more than 1 http trigger
		{
			triggers: map[string]functionconfig.Trigger{
				"firstHTTPTrigger": {
					Kind: "http",
				},
				"secondHTTPTrigger": {
					Kind: "http",
				},
			},
			shouldFailValidation: true,
		},

		// do not allow empty name triggers
		{
			triggers: map[string]functionconfig.Trigger{
				"": {
					Kind: "http",
				},
			},
			shouldFailValidation: true,
		},

		// mismatching trigger key and name
		{
			triggers: map[string]functionconfig.Trigger{
				"a": {
					Kind: "http",
					Name: "b",
				},
			},
			shouldFailValidation: true,
		},
	} {

		suite.mockedPlatform.On("GetProjects", suite.ctx, &platform.GetProjectsOptions{
			Meta: platform.ProjectMeta{
				Name:      platform.DefaultProjectName,
				Namespace: "default",
			},
		}).Return([]platform.Project{
			&platform.AbstractProject{},
		}, nil).Once()

		// name it with index and shift with 65 to get A as first letter
		functionName := string(rune(idx + 65))
		functionConfig := *functionconfig.NewConfig()

		createFunctionOptions := &platform.CreateFunctionOptions{
			Logger:         suite.Logger,
			FunctionConfig: functionConfig,
		}
		createFunctionOptions.FunctionConfig.Meta.Name = functionName
		createFunctionOptions.FunctionConfig.Meta.Labels = map[string]string{
			"nuclio.io/project-name": platform.DefaultProjectName,
		}
		createFunctionOptions.FunctionConfig.Spec.Triggers = testCase.triggers
		suite.Logger.DebugWith("Checking function ", "functionName", functionName)

		err := suite.Platform.EnrichFunctionConfig(suite.ctx, &createFunctionOptions.FunctionConfig)
		suite.Require().NoError(err, "Failed to enrich function")

		err = suite.Platform.ValidateFunctionConfig(suite.ctx, &createFunctionOptions.FunctionConfig)
		if testCase.shouldFailValidation {
			suite.Require().Error(err, "Validation passed unexpectedly")
			continue
		}

		suite.Require().NoError(err, "Validation failed unexpectedly")
		suite.Equal(testCase.expectedEnrichedTriggers,
			createFunctionOptions.FunctionConfig.Spec.Triggers)
	}
}

func (suite *AbstractPlatformTestSuite) TestValidateFunctionConfigDockerImagesFields() {

	// we do our docker image test coverage on functionConfig.Spec.Build.Image but other fields, like
	// functionConfig.Spec.Image are going through the same validation
	for _, testCase := range []struct {
		buildImage string
		valid      bool
	}{
		// positive cases
		{"just_image-name", true},
		{"image-name-with:tag-333", true},
		{"repo/image:v1.0.0", true},
		{"123.123.123.123:123/image/tag:v1.0.0", true},
		{"some-domain.com/image/tag", true},
		{"some-domain.com/image/tag:v1.1.1-patch1", true},
		{"image/tag", true},
		{"image", true},
		{"image:v1.1.1-patch", true},
		{"ubuntu@sha256:45b23dee08af5e43a7fea6c4cf9c25ccf269ee113168c19722f87876677c5cb2", true},
		{"iguaziodocker/cloud_demo_functions", true},
		{"ghaanvkoqi-snilhltidtkmncpufnhdmpwngszj-naip_test_img", true},
		{"repo/image_with__two-underscores:v1.0.0", true},
		{"repo/underscored_repo__yes/name.with.dot:v1.0.0", true},
		{"underscored_repo__allowed/with/name.with.dot:v1.0.0", true},
		{"localhost:5000/", true}, // HACK: some tests use this so we allow this

		// negative cases
		{"image/tag:v1.0.0 || nc 127.0.0.1 8000 -e /bin/sh ls", false},
		{"123.123.123.123:123/tag:v1.0.0 | echo something", false},
		{"123.123_123.123:123/tag:v1.0.0", false},
		{"gcr_nope.io:80/repo_w_underscore_is_ok/tag:v1.0.0", false},
		{"repo/image:v1.0.0;xyz&netstat", false},
		{"repo/image:v1.0.0;ls|cp&rm", false},
		{"image\" cp something", false},
		{"image\\\" cp something", false},
	} {

		functionConfig := *functionconfig.NewConfig()
		functionConfig.Spec.Build.Image = testCase.buildImage

		suite.Logger.InfoWith("Running function spec sanitization case",
			"functionConfig", functionConfig,
			"valid", testCase.valid)

		suite.mockedPlatform.
			On("GetProjects", suite.ctx, &platform.GetProjectsOptions{
				Meta: platform.ProjectMeta{Namespace: "default"},
			}).
			Return([]platform.Project{&platform.AbstractProject{}}, nil).
			Once()

		err := suite.Platform.ValidateFunctionConfig(suite.ctx, &functionConfig)
		if !testCase.valid {
			suite.Require().Error(err, "Validation passed unexpectedly")
			suite.Logger.InfoWith("Expected error received", "err", err, "functionConfig", functionConfig)
			continue
		}
		suite.Require().NoError(err)
	}
}

func (suite *AbstractPlatformTestSuite) TestProcessLogLines() {
	for _, testCase := range []struct {
		name           string
		encoding       string
		encodingConfig *nucliozap.EncoderConfig
	}{
		{
			name:           "default",
			encoding:       "json",
			encodingConfig: nucliozap.NewEncoderConfig(),
		},
		{
			name:     "NoLineEnding",
			encoding: "json",
			encodingConfig: func() *nucliozap.EncoderConfig {
				encoding := nucliozap.NewEncoderConfig()
				encoding.JSON.VarGroupMode = nucliozap.VarGroupModeFlattened
				encoding.JSON.LineEnding = ""
				return encoding
			}(),
		},
		{
			name:     "Structured",
			encoding: "json",
			encodingConfig: func() *nucliozap.EncoderConfig {
				encoding := nucliozap.NewEncoderConfig()
				encoding.JSON.VarGroupMode = nucliozap.VarGroupModeStructured
				return encoding
			}(),
		},
		{
			name:     "Flattened",
			encoding: "json",
			encodingConfig: func() *nucliozap.EncoderConfig {
				encoding := nucliozap.NewEncoderConfig()
				encoding.JSON.VarGroupMode = nucliozap.VarGroupModeFlattened
				return encoding
			}(),
		},
	} {
		suite.Run(testCase.name, func() {
			writer := &bytes.Buffer{}
			loggerInstance, err := nucliozap.NewNuclioZap(testCase.name,
				testCase.encoding,
				testCase.encodingConfig,
				writer,
				writer,
				nucliozap.InfoLevel)
			suite.Require().NoError(err)

			errorMessage := "Failed to start trigger"
			loggerInstance.WarnWith(errorMessage, "var", "details", "extra", "information")

			suite.Logger.InfoWith("Getting processor logs and brief error", "logLine", writer.String())

			scanner := bufio.NewScanner(writer)
			formattedProcessorLogs, briefErrorsMessage := suite.Platform.GetProcessorLogsAndBriefError(scanner)

			suite.Logger.InfoWith("Successfully processed processor logs and brief error",
				"formattedProcessorLogs", formattedProcessorLogs,
				"briefErrorsMessage", briefErrorsMessage)

			suite.Require().Contains(formattedProcessorLogs, fmt.Sprintf("(W) %s", errorMessage))
			suite.Require().Contains(briefErrorsMessage, errorMessage)
			for _, processedLogLine := range []string{
				formattedProcessorLogs,
				briefErrorsMessage,
			} {
				suite.Require().Condition(func() (success bool) {
					success =
						strings.Contains(processedLogLine, `[var="details" || extra="information"]`)
					success = success ||
						strings.Contains(processedLogLine, `[extra="information" || var="details"]`)
					return success
				})
			}

		})
	}
}

func (suite *AbstractPlatformTestSuite) TestGetProcessorLogsOnMultiWorker() {
	suite.testGetProcessorLogsTestFromFile(MultiWorkerFunctionLogsFilePath)
}

func (suite *AbstractPlatformTestSuite) TestGetProcessorLogsOnPanic() {
	suite.testGetProcessorLogsTestFromFile(PanicFunctionLogsFilePath)
}

func (suite *AbstractPlatformTestSuite) TestGetProcessorLogsOnGoWithCallStack() {
	suite.testGetProcessorLogsTestFromFile(GoWithCallStackFunctionLogsFilePath)
}

func (suite *AbstractPlatformTestSuite) TestGetProcessorLogsWithSpecialSubstrings() {
	suite.testGetProcessorLogsTestFromFile(SpecialSubstringsFunctionLogsFilePath)
}

func (suite *AbstractPlatformTestSuite) TestGetProcessorLogsWithConsecutiveDuplicateMessages() {
	suite.testGetProcessorLogsTestFromFile(ConsecutiveDuplicateFunctionLogsFilePath)
}

func (suite *AbstractPlatformTestSuite) TestCreateFunctionEvent() {
	functionName := "some-function-name"
	projectName := "some-project-name"
	suite.mockedPlatform.On("GetFunctions", suite.ctx, mock.Anything).
		Return([]platform.Function{
			&platform.AbstractFunction{
				Logger:   suite.Logger,
				Platform: suite.Platform.platform,
				Config: functionconfig.Config{
					Meta: functionconfig.Meta{
						Name: functionName,
						Labels: map[string]string{
							common.NuclioResourceLabelKeyProjectName: projectName,
						},
					},
				},
			},
		}, nil).Once()
	defer suite.mockedPlatform.AssertExpectations(suite.T())

	functionEvent := platform.FunctionEventConfig{
		Meta: platform.FunctionEventMeta{
			Name:      "test-function-event",
			Namespace: suite.DefaultNamespace,
			Labels: map[string]string{
				common.NuclioResourceLabelKeyFunctionName: functionName,
			},
		},
	}

	// key not exists / enriched
	suite.Require().Equal(functionEvent.Meta.Labels[common.NuclioResourceLabelKeyProjectName], "")
	err := suite.Platform.EnrichFunctionEvent(suite.ctx, &functionEvent)

	// enriched with project name
	suite.Require().Equal(functionEvent.Meta.Labels[common.NuclioResourceLabelKeyProjectName], projectName)
	suite.Require().NoError(err)
}

func (suite *AbstractPlatformTestSuite) TestValidateNodeSelector() {
	for idx, testCase := range []struct {
		name                 string
		nodeSelector         map[string]string
		shouldFailValidation bool
	}{

		// happy flows
		{
			name: "Sanity",
			nodeSelector: map[string]string{
				"some-key": "some-value",
			},
		},
		{
			name: "AllowEmptyValue",
			nodeSelector: map[string]string{
				"some-key": "",
			},
		},
		{
			name: "TrueValuesSanity",
			nodeSelector: map[string]string{
				"beta.kubernetes.io/arch":                        "amd64",
				"beta.kubernetes.io/os":                          "linux",
				"feature.node.kubernetes.io/cpu-cpuid.ADX":       "true",
				"feature.node.kubernetes.io/kernel-version.full": "3.10.0-1127.13.1.el7.x86_64",
			},
		},

		// bad flows
		{
			name: "InvalidValue",
			nodeSelector: map[string]string{
				"some-key": "_some-value",
			},
			shouldFailValidation: true,
		},
		{
			name: "FailMissingKey",
			nodeSelector: map[string]string{
				"": "some-value",
			},
			shouldFailValidation: true,
		},
		{
			name: "Invalid2ndValue",
			nodeSelector: map[string]string{
				"some-key":      "some-value",
				"some-key/name": "_some-value",
			},
			shouldFailValidation: true,
		},
		{
			name: "SegmentInvalidPrefix",
			nodeSelector: map[string]string{
				"some-key":  "some-value",
				"/some-key": "some-value",
			},
			shouldFailValidation: true,
		},
	} {
		suite.Run(testCase.name, func() {
			suite.mockedPlatform.On("GetProjects", suite.ctx, &platform.GetProjectsOptions{
				Meta: platform.ProjectMeta{
					Name:      platform.DefaultProjectName,
					Namespace: "default",
				},
			}).Return([]platform.Project{
				&platform.AbstractProject{},
			}, nil).Once()

			// name it with index and shift with 65 to get A as first letter
			functionName := string(rune(idx + 65))
			functionConfig := *functionconfig.NewConfig()
			functionConfig.Spec.NodeSelector = testCase.nodeSelector

			createFunctionOptions := &platform.CreateFunctionOptions{
				Logger:         suite.Logger,
				FunctionConfig: functionConfig,
			}
			createFunctionOptions.FunctionConfig.Meta.Name = functionName
			createFunctionOptions.FunctionConfig.Meta.Labels = map[string]string{
				"nuclio.io/project-name": platform.DefaultProjectName,
			}
			suite.Logger.DebugWith("Checking function ", "functionName", functionName)

			err := suite.Platform.EnrichFunctionConfig(suite.ctx, &createFunctionOptions.FunctionConfig)
			suite.Require().NoError(err, "Failed to enrich function")

			err = suite.Platform.ValidateFunctionConfig(suite.ctx, &createFunctionOptions.FunctionConfig)
			if testCase.shouldFailValidation {
				suite.Require().Error(err, "Validation passed unexpectedly")
			} else {
				suite.Require().NoError(err, "Validation failed unexpectedly")
			}
		})
	}
}

func (suite *AbstractPlatformTestSuite) TestValidatePriorityClassName() {
	for idx, testCase := range []struct {
		name                            string
		priorityClassName               string
		validFunctionPriorityClassNames []string
		shouldFailValidation            bool
	}{

		// happy flows
		{
			name:                            "Sanity",
			priorityClassName:               "low-priority",
			validFunctionPriorityClassNames: []string{"low-priority"},
		},
		{
			name:                            "MultipleValidValues",
			priorityClassName:               "low-priority",
			validFunctionPriorityClassNames: []string{"low-priority", "medium-priority", "high-priority"},
		},
		{

			// all priorityClassName should be valid when validFunctionPriorityClassNames is nil
			name:                            "NilValidFunctionPriorityClassNames",
			priorityClassName:               "low-priority",
			validFunctionPriorityClassNames: nil,
		},

		// bad flows
		{
			name:                            "NonValidValue",
			priorityClassName:               "non-valid-priority",
			validFunctionPriorityClassNames: []string{"low-priority", "medium-priority", "high-priority"},
			shouldFailValidation:            true,
		},
		{
			name:                            "NoValidValues",
			priorityClassName:               "low-priority",
			validFunctionPriorityClassNames: []string{},
			shouldFailValidation:            true,
		},
	} {
		suite.Run(testCase.name, func() {
			suite.mockedPlatform.On("GetProjects", suite.ctx, &platform.GetProjectsOptions{
				Meta: platform.ProjectMeta{
					Name:      platform.DefaultProjectName,
					Namespace: "default",
				},
			}).Return([]platform.Project{
				&platform.AbstractProject{},
			}, nil).Once()

			// name it with index and shift with 65 to get A as first letter
			functionName := string(rune(idx + 65))
			functionConfig := *functionconfig.NewConfig()
			functionConfig.Spec.PriorityClassName = testCase.priorityClassName

			createFunctionOptions := &platform.CreateFunctionOptions{
				Logger:         suite.Logger,
				FunctionConfig: functionConfig,
			}
			createFunctionOptions.FunctionConfig.Meta.Name = functionName
			createFunctionOptions.FunctionConfig.Meta.Labels = map[string]string{
				"nuclio.io/project-name": platform.DefaultProjectName,
			}
			suite.Platform.Config.Kube.ValidFunctionPriorityClassNames = testCase.validFunctionPriorityClassNames
			suite.Logger.DebugWith("Checking function ", "functionName", functionName)

			err := suite.Platform.EnrichFunctionConfig(suite.ctx, &createFunctionOptions.FunctionConfig)
			suite.Require().NoError(err, "Failed to enrich function")

			err = suite.Platform.ValidateFunctionConfig(suite.ctx, &createFunctionOptions.FunctionConfig)
			if testCase.shouldFailValidation {
				suite.Require().Error(err, "Validation passed unexpectedly")
			} else {
				suite.Require().NoError(err, "Validation failed unexpectedly")
			}
		})
	}
}

func (suite *AbstractPlatformTestSuite) TestValidateVolumes() {
	for idx, testCase := range []struct {
		name                 string
		functionVolumes      []functionconfig.Volume
		shouldFailValidation bool
	}{

		// happy flows
		{
			name:            "noVolumes",
			functionVolumes: nil,
		},
		{
			name: "singleVolume",
			functionVolumes: []functionconfig.Volume{
				{
					Volume: v1.Volume{
						Name: "something",
					},
					VolumeMount: v1.VolumeMount{
						Name: "something",
					},
				},
			},
		},
		{
			name: "sameVolumesMultipleMounts",
			functionVolumes: []functionconfig.Volume{
				{
					Volume: v1.Volume{
						Name: "something",
						VolumeSource: v1.VolumeSource{
							Secret: &v1.SecretVolumeSource{
								SecretName: "some-secret",
							},
						},
					},
					VolumeMount: v1.VolumeMount{
						Name:      "something",
						MountPath: "/here",
					},
				},
				{
					Volume: v1.Volume{
						Name: "something",
						VolumeSource: v1.VolumeSource{
							Secret: &v1.SecretVolumeSource{
								SecretName: "some-secret",
							},
						},
					},
					VolumeMount: v1.VolumeMount{
						Name:      "something",
						MountPath: "/there",
					},
				},
			},
		},

		// bad flows
		{
			name: "missingName",
			functionVolumes: []functionconfig.Volume{
				{
					Volume: v1.Volume{
						Name: "",
					},
					VolumeMount: v1.VolumeMount{
						Name: "",
					},
				},
			},
			shouldFailValidation: true,
		},
		{
			name: "invalidNameReference",
			functionVolumes: []functionconfig.Volume{
				{
					Volume: v1.Volume{
						Name: "x",
					},
					VolumeMount: v1.VolumeMount{
						Name: "y",
					},
				},
			},
			shouldFailValidation: true,
		},
		{
			name: "differentVolumesMultipleMounts",
			functionVolumes: []functionconfig.Volume{
				{
					Volume: v1.Volume{
						Name: "something",
						VolumeSource: v1.VolumeSource{
							ConfigMap: &v1.ConfigMapVolumeSource{
								LocalObjectReference: v1.LocalObjectReference{
									Name: "some-cm",
								},
							},
						},
					},
					VolumeMount: v1.VolumeMount{
						Name:      "something",
						MountPath: "/here",
					},
				},
				{
					Volume: v1.Volume{
						Name: "something",
						VolumeSource: v1.VolumeSource{
							Secret: &v1.SecretVolumeSource{
								SecretName: "some-secret",
							},
						},
					},
					VolumeMount: v1.VolumeMount{
						Name:      "something",
						MountPath: "/there",
					},
				},
			},
			shouldFailValidation: true,
		},
	} {
		suite.Run(testCase.name, func() {
			suite.mockedPlatform.
				On("GetProjects", suite.ctx, &platform.GetProjectsOptions{
					Meta: platform.ProjectMeta{
						Name:      platform.DefaultProjectName,
						Namespace: "default",
					},
				}).
				Return([]platform.Project{
					&platform.AbstractProject{},
				}, nil).
				Once()

			// name it with index and shift with 65 to get A as first letter
			functionName := string(rune(idx + 65))
			functionConfig := *functionconfig.NewConfig()
			functionConfig.Spec.Volumes = testCase.functionVolumes

			createFunctionOptions := &platform.CreateFunctionOptions{
				Logger:         suite.Logger,
				FunctionConfig: functionConfig,
			}
			createFunctionOptions.FunctionConfig.Meta.Name = functionName
			createFunctionOptions.FunctionConfig.Meta.Labels = map[string]string{
				"nuclio.io/project-name": platform.DefaultProjectName,
			}
			suite.Logger.DebugWith("Checking function", "functionName", functionName)

			err := suite.Platform.EnrichFunctionConfig(suite.ctx, &createFunctionOptions.FunctionConfig)
			suite.Require().NoError(err)

			err = suite.Platform.ValidateFunctionConfig(suite.ctx, &createFunctionOptions.FunctionConfig)
			if testCase.shouldFailValidation {
				suite.Require().Error(err, "Validation passed unexpectedly")
			} else {
				suite.Require().NoError(err, "Validation failed unexpectedly")
			}
		})
	}
}

// Test that GetProcessorLogs() generates the expected formattedPodLogs and briefErrorsMessage
// Expects 3 files inside functionLogsFilePath: (kept in these constants)
// - FunctionLogsFile
// - FormattedFunctionLogsFile
// - BriefErrorsMessageFile
func (suite *AbstractPlatformTestSuite) testGetProcessorLogsTestFromFile(functionLogsFilePath string) {
	functionLogsFile, err := os.Open(path.Join(functionLogsFilePath, FunctionLogsFile))
	suite.Require().NoError(err, "Failed to read function logs file")

	functionLogsScanner := bufio.NewScanner(functionLogsFile)

	formattedPodLogs, briefErrorsMessage := suite.Platform.GetProcessorLogsAndBriefError(functionLogsScanner)

	expectedFormattedFunctionLogsFileBytes, err := ioutil.ReadFile(path.Join(functionLogsFilePath, FormattedFunctionLogsFile))
	suite.Require().NoError(err, "Failed to read formatted function logs file")
	suite.Assert().Equal(string(expectedFormattedFunctionLogsFileBytes), formattedPodLogs)

	expectedBriefErrorsMessageFileBytes, err := ioutil.ReadFile(path.Join(functionLogsFilePath, BriefErrorsMessageFile))
	suite.Require().NoError(err, "Failed to read brief errors message file")
	suite.Assert().Equal(string(expectedBriefErrorsMessageFileBytes), briefErrorsMessage)
}

func TestAbstractPlatformTestSuite(t *testing.T) {
	suite.Run(t, new(AbstractPlatformTestSuite))
}
