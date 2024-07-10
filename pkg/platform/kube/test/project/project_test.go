//go:build test_integration && test_kube

/*
Copyright 2024 The Nuclio Authors.

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

package project

import (
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/platform"
	suite2 "github.com/nuclio/nuclio/pkg/platform/kube/test/suite"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/suite"
)

type ProjectTestSuite struct {
	suite2.KubeTestSuite
}

func (suite *ProjectTestSuite) TestCreate() {
	projectConfig := platform.ProjectConfig{
		Meta: platform.ProjectMeta{
			Name:      "test-project",
			Namespace: suite.Namespace,
			Labels: map[string]string{
				"label-key": "label-value",
			},
			Annotations: map[string]string{
				"annotation-key": "annotation-value",
			},
		},
		Spec: platform.ProjectSpec{
			Description: "some description",
		},
	}

	// create project
	err := suite.Platform.CreateProject(suite.Ctx, &platform.CreateProjectOptions{
		ProjectConfig: &projectConfig,
	})
	suite.Require().NoError(err, "Failed to create project")
	defer func() {
		err = suite.Platform.DeleteProject(suite.Ctx, &platform.DeleteProjectOptions{
			Meta:     projectConfig.Meta,
			Strategy: platform.DeleteProjectStrategyRestricted,
		})
		suite.Require().NoError(err, "Failed to delete project")
	}()

	// get created project
	projects, err := suite.Platform.GetProjects(suite.Ctx, &platform.GetProjectsOptions{
		Meta: projectConfig.Meta,
	})
	suite.Require().NoError(err, "Failed to get projects")
	suite.Require().Equal(len(projects), 1)

	// requested and created project are equal
	createdProject := projects[0]
	suite.Require().Equal(projectConfig, *createdProject.GetConfig())
}

func (suite *ProjectTestSuite) TestCreateFromLeaderIgnoreInvalidLabels() {
	invalidLabelKey1 := "invalid.label@-key"
	invalidLabelKey2 := "the-key-is-invalid"
	projectConfig := platform.ProjectConfig{
		Meta: platform.ProjectMeta{
			Name:      "test-project",
			Namespace: suite.Namespace,
			Labels: map[string]string{
				invalidLabelKey1: "label-value",
				invalidLabelKey2: "inva!id_v8$alue",
			},
		},
		Spec: platform.ProjectSpec{
			Description: "some description",
		},
	}

	// set the platform's leader kind
	suite.Platform.GetConfig().ProjectsLeader = &platformconfig.ProjectsLeader{
		Kind: platformconfig.ProjectsLeaderKindMock,
	}

	// create project
	err := suite.Platform.CreateProject(suite.Ctx, &platform.CreateProjectOptions{
		ProjectConfig: &projectConfig,
		RequestOrigin: platformconfig.ProjectsLeaderKindMock,
	})
	suite.Require().NoError(err, "Failed to create project")
	defer func() {
		err = suite.Platform.DeleteProject(suite.Ctx, &platform.DeleteProjectOptions{
			Meta:     projectConfig.Meta,
			Strategy: platform.DeleteProjectStrategyRestricted,
		})
		suite.Require().NoError(err, "Failed to delete project")
	}()

	// get created project
	projects, err := suite.Platform.GetProjects(suite.Ctx, &platform.GetProjectsOptions{
		Meta: projectConfig.Meta,
	})
	suite.Require().NoError(err, "Failed to get projects")
	suite.Require().Equal(len(projects), 1)

	// verify created project does not contain the invalid label
	createdProject := projects[0]
	suite.Require().NotContains(createdProject.GetConfig().Meta.Labels, invalidLabelKey1)
	suite.Require().NotContains(createdProject.GetConfig().Meta.Labels, invalidLabelKey2)
}

func (suite *ProjectTestSuite) TestUpdate() {
	projectConfig := platform.ProjectConfig{
		Meta: platform.ProjectMeta{
			Name:      "test-project",
			Namespace: suite.Namespace,
			Labels: map[string]string{
				"something": "here",
			},
			Annotations: map[string]string{
				"annotation-key": "annotation-value",
			},
		},
		Spec: platform.ProjectSpec{
			Description: "Simple description",
		},
	}

	// create project
	err := suite.Platform.CreateProject(suite.Ctx, &platform.CreateProjectOptions{
		ProjectConfig: &projectConfig,
	})
	suite.Require().NoError(err, "Failed to create project")

	// delete leftover
	defer func() {
		err := suite.Platform.DeleteProject(suite.Ctx, &platform.DeleteProjectOptions{
			Meta:     projectConfig.Meta,
			Strategy: platform.DeleteProjectStrategyRestricted,
		})
		suite.Require().NoError(err, "Failed to delete project")
	}()

	// change project annotations
	projectConfig.Meta.Annotations["annotation-key"] = "annotation-value-changed"
	projectConfig.Meta.Annotations["added-annotation"] = "added-annotation-value"

	// change project labels
	projectConfig.Meta.Labels["label-key"] = "label-value-changed"
	projectConfig.Meta.Labels["added-label"] = "added-label-value"

	// update project
	err = suite.Platform.UpdateProject(suite.Ctx, &platform.UpdateProjectOptions{
		ProjectConfig: projectConfig,
	})
	suite.Require().NoError(err, "Failed to update project")

	// get updated project
	updatedProject := suite.GetProject(&platform.GetProjectsOptions{
		Meta: projectConfig.Meta,
	})
	suite.Require().Empty(cmp.Diff(projectConfig, *updatedProject.GetConfig(),
		cmp.Options{
			cmpopts.IgnoreFields(projectConfig.Status, "UpdatedAt"), // automatically populated
		}))
}

func (suite *ProjectTestSuite) TestDelete() {
	projectConfig := platform.ProjectConfig{
		Meta: platform.ProjectMeta{
			Name:      "test-project",
			Namespace: suite.Namespace,
			Labels: map[string]string{
				"something": "here",
			},
			Annotations: map[string]string{
				"annotation-key": "annotation-value",
			},
		},
		Spec: platform.ProjectSpec{
			Description: "Simple description",
		},
	}

	// create project
	err := suite.Platform.CreateProject(suite.Ctx, &platform.CreateProjectOptions{
		ProjectConfig: &projectConfig,
	})
	suite.Require().NoError(err, "Failed to create project")

	// delete project
	err = suite.Platform.DeleteProject(suite.Ctx, &platform.DeleteProjectOptions{
		Meta:     projectConfig.Meta,
		Strategy: platform.DeleteProjectStrategyRestricted,
	})
	suite.Require().NoError(err, "Failed to delete project")

	// ensure project does not exist
	projects, err := suite.Platform.GetProjects(suite.Ctx, &platform.GetProjectsOptions{
		Meta: projectConfig.Meta,
	})
	suite.Require().NoError(err, "Failed to get projects")
	suite.Require().Equal(0, len(projects))
}

func (suite *ProjectTestSuite) TestDeleteCascading() {

	// create project
	projectToDeleteConfig := platform.ProjectConfig{
		Meta: platform.ProjectMeta{
			Name:      "project-to-delete",
			Namespace: suite.Namespace,
		},
	}
	err := suite.Platform.CreateProject(suite.Ctx, &platform.CreateProjectOptions{
		ProjectConfig: &projectToDeleteConfig,
	})
	suite.Require().NoError(err, "Failed to create project")

	// create 2 functions (deleted along with `projectToDeleteConfig`)

	// create function A
	functionToDeleteA := suite.CreateImportedFunction("func-to-delete-a", projectToDeleteConfig.Meta.Name)
	functionToDeleteB := suite.CreateImportedFunction("func-to-delete-b", projectToDeleteConfig.Meta.Name)

	// delete leftovers
	defer suite.Platform.DeleteFunction(suite.Ctx, &platform.DeleteFunctionOptions{ // nolint: errcheck
		FunctionConfig: *functionToDeleteA,
	})
	defer suite.Platform.DeleteFunction(suite.Ctx, &platform.DeleteFunctionOptions{ // nolint: errcheck
		FunctionConfig: *functionToDeleteB,
	})

	// create api gateway for function A (deleted along with `projectToDeleteConfig`)
	createAPIGatewayOptions := suite.CompileCreateAPIGatewayOptions("apigw-to-delete",
		functionToDeleteA.Meta.Name)
	createAPIGatewayOptions.APIGatewayConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName] = projectToDeleteConfig.Meta.Name
	err = suite.Platform.CreateAPIGateway(suite.Ctx, createAPIGatewayOptions)
	suite.Require().NoError(err)
	defer suite.Platform.DeleteAPIGateway(suite.Ctx, &platform.DeleteAPIGatewayOptions{ // nolint: errcheck
		Meta: createAPIGatewayOptions.APIGatewayConfig.Meta,
	})

	suite.WaitForAPIGatewayState(&platform.GetAPIGatewaysOptions{
		Name:      createAPIGatewayOptions.APIGatewayConfig.Meta.Name,
		Namespace: createAPIGatewayOptions.APIGatewayConfig.Meta.Namespace,
	}, platform.APIGatewayStateReady, 10*time.Second)

	// create 2 function events for function B (deleted along with `projectToDeleteConfig`)
	functionEventA := suite.CompileCreateFunctionEventOptions("function-event-a", functionToDeleteB.Meta.Name)
	err = suite.Platform.CreateFunctionEvent(suite.Ctx, functionEventA)
	suite.Require().NoError(err)
	defer suite.Platform.DeleteFunctionEvent(suite.Ctx, &platform.DeleteFunctionEventOptions{ // nolint: errcheck
		Meta: platform.FunctionEventMeta{
			Name:      functionEventA.FunctionEventConfig.Meta.Name,
			Namespace: suite.Namespace,
		},
	})

	functionEventB := suite.CompileCreateFunctionEventOptions("function-event-b", functionToDeleteB.Meta.Name)
	err = suite.Platform.CreateFunctionEvent(suite.Ctx, functionEventB)
	suite.Require().NoError(err)
	defer suite.Platform.DeleteFunctionEvent(suite.Ctx, &platform.DeleteFunctionEventOptions{ // nolint: errcheck
		Meta: platform.FunctionEventMeta{
			Name:      functionEventB.FunctionEventConfig.Meta.Name,
			Namespace: suite.Namespace,
		},
	})

	// try restrict - expect it to fail (project has sub resources)
	err = suite.Platform.DeleteProject(suite.Ctx, &platform.DeleteProjectOptions{
		Meta: platform.ProjectMeta{
			Name:      projectToDeleteConfig.Meta.Name,
			Namespace: suite.Namespace,
		},
		Strategy: platform.DeleteProjectStrategyRestricted,
	})
	suite.Require().Error(err)

	// try cascading - expect it succeed
	err = suite.Platform.DeleteProject(suite.Ctx, &platform.DeleteProjectOptions{
		Meta: platform.ProjectMeta{
			Name:      projectToDeleteConfig.Meta.Name,
			Namespace: suite.Namespace,
		},
		Strategy:                           platform.DeleteProjectStrategyCascading,
		WaitForResourcesDeletionCompletion: true,
		WaitForResourcesDeletionCompletionDuration: 3 * time.Minute,
	})
	suite.Require().NoError(err)

	// assertion - project should be deleted
	projects, err := suite.Platform.GetProjects(suite.Ctx, &platform.GetProjectsOptions{
		Meta: platform.ProjectMeta{
			Name:      projectToDeleteConfig.Meta.Name,
			Namespace: suite.Namespace,
		},
	})
	suite.Require().NoError(err)
	suite.Require().Len(projects, 0)

	suite.Logger.InfoWith("Ensuring resources were removed (deletion is being executed in background")

	// ensure api gateway deleted
	apiGateways, err := suite.Platform.GetAPIGateways(suite.Ctx, &platform.GetAPIGatewaysOptions{
		Name:      createAPIGatewayOptions.APIGatewayConfig.Meta.Name,
		Namespace: suite.Namespace,
	})
	suite.Require().NoError(err)
	suite.Require().Len(apiGateways, 0, "Some api gateways were not removed")

	// ensure functions were deleted successfully
	for _, functionName := range []string{
		functionToDeleteA.Meta.Name,
		functionToDeleteB.Meta.Name,
	} {
		functions, err := suite.Platform.GetFunctions(suite.Ctx, &platform.GetFunctionsOptions{
			Name:      functionName,
			Namespace: suite.Namespace,
		})
		suite.Require().NoError(err)
		suite.Require().Len(functions, 0, "Some functions were not removed")
	}

	// ensure function events were deleted successfully
	for _, functionEventName := range []string{
		functionEventA.FunctionEventConfig.Meta.Name,
		functionEventB.FunctionEventConfig.Meta.Name,
	} {
		functionEvents, err := suite.Platform.GetFunctionEvents(suite.Ctx, &platform.GetFunctionEventsOptions{
			Meta: platform.FunctionEventMeta{
				Name:      functionEventName,
				Namespace: suite.Namespace,
			},
		})
		suite.Require().NoError(err)
		suite.Require().Len(functionEvents, 0, "Some function events were not removed")
	}
}

func TestProjectTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}
	suite.Run(t, new(ProjectTestSuite))
}
