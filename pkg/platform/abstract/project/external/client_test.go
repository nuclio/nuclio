//go:build test_unit

/*
Copyright 2023 The Nuclio Authors.

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

package external

import (
	"context"
	"testing"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project"
	leadermock "github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader/mock"
	internalmock "github.com/nuclio/nuclio/pkg/platform/abstract/project/mock"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ExternalProjectClientTestSuite struct {
	suite.Suite

	project.Client
	Logger                     logger.Logger
	mockInternalProjectsClient *internalmock.Client
	mockLeaderProjectsClient   *leadermock.Client
	ctx                        context.Context
}

func (suite *ExternalProjectClientTestSuite) SetupTest() {
	var err error

	// create logger
	suite.Logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	// create context
	suite.ctx = context.Background()

	// mock internal client
	suite.mockInternalProjectsClient = &internalmock.Client{}

	//mock leader client
	suite.mockLeaderProjectsClient = leadermock.NewClient()

	// create platform configuration
	platformConfiguration := platformconfig.Config{
		ProjectsLeader: &platformconfig.ProjectsLeader{
			Kind: platformconfig.ProjectsLeaderKindMock,
		},
	}

	// create external projects client
	suite.Client = &Client{
		platformConfiguration: &platformConfiguration,
		internalClient:        suite.mockInternalProjectsClient,
		leaderClient:          suite.mockLeaderProjectsClient,
	}
}

func (suite *ExternalProjectClientTestSuite) TestLeaderCreate() {
	createProjectOptions := platform.CreateProjectOptions{
		RequestOrigin: platformconfig.ProjectsLeaderKindMock,
		ProjectConfig: &platform.ProjectConfig{
			Meta: platform.ProjectMeta{
				Name: "test-func",
			},
		},
	}

	suite.expectGetExistingProject("test-func")
	suite.expectEvaluateLeaderRequest(true)
	suite.mockInternalProjectsClient.
		On("Create", suite.ctx, &createProjectOptions).
		Return(&platform.AbstractProject{}, nil).
		Once()

	_, err := suite.Create(suite.ctx, &createProjectOptions)
	suite.Require().NoError(err)
}

func (suite *ExternalProjectClientTestSuite) TestLeaderUpdate() {
	updateProjectOptions := platform.UpdateProjectOptions{
		RequestOrigin: platformconfig.ProjectsLeaderKindMock,
		ProjectConfig: platform.ProjectConfig{
			Meta: platform.ProjectMeta{Name: "test-func"},
		},
	}

	suite.expectGetExistingProject("test-func")
	suite.expectEvaluateLeaderRequest(true)
	suite.mockInternalProjectsClient.
		On("Update", suite.ctx, &updateProjectOptions).
		Return(&platform.AbstractProject{}, nil).
		Once()

	_, err := suite.Update(suite.ctx, &updateProjectOptions)
	suite.Require().NoError(err)
}

func (suite *ExternalProjectClientTestSuite) TestLeaderDelete() {
	deleteProjectOptions := platform.DeleteProjectOptions{
		RequestOrigin: platformconfig.ProjectsLeaderKindMock,
		Meta:          platform.ProjectMeta{Name: "test-func"},
	}

	suite.expectGetExistingProject("test-func")
	suite.expectEvaluateLeaderRequest(true)
	suite.mockInternalProjectsClient.
		On("Delete", suite.ctx, &deleteProjectOptions).
		Return(nil).
		Once()

	err := suite.Delete(suite.ctx, &deleteProjectOptions)
	suite.Require().NoError(err)
}

// TestLeaderCreateSkipsEvaluationWhen2PCDisabled covers the short-circuit path: when
// the configured leader does not run 2PC (Iguazio pass-through, or MLRun with the
// feature flag disabled), the external client must skip both the internal Get and the
// EvaluateLeaderRequest call, going straight to the internal write.
func (suite *ExternalProjectClientTestSuite) TestLeaderCreateSkipsEvaluationWhen2PCDisabled() {
	createProjectOptions := platform.CreateProjectOptions{
		RequestOrigin: platformconfig.ProjectsLeaderKindMock,
		ProjectConfig: &platform.ProjectConfig{
			Meta: platform.ProjectMeta{Name: "test-func"},
		},
	}

	suite.mockLeaderProjectsClient.
		On("ProjectSync2PCEnabled").
		Return(false).
		Once()
	suite.mockInternalProjectsClient.
		On("Create", suite.ctx, &createProjectOptions).
		Return(&platform.AbstractProject{}, nil).
		Once()

	_, err := suite.Create(suite.ctx, &createProjectOptions)
	suite.Require().NoError(err)

	suite.mockInternalProjectsClient.AssertNotCalled(suite.T(), "Get", mock.Anything, mock.Anything)
	suite.mockLeaderProjectsClient.AssertNotCalled(suite.T(), "EvaluateLeaderRequest", mock.Anything, mock.Anything, mock.Anything)
}

// TestCreateSkipsLeaderEvaluation asserts that when a leader-origin request carries
// SkipLeaderEvaluation (sourced from the x-mlrun-force-sync header), Create skips the
// 2PC pipeline entirely — no ProjectSync2PCEnabled, no Get, no EvaluateLeaderRequest —
// and applies the write through the internal client directly.
func (suite *ExternalProjectClientTestSuite) TestCreateSkipsLeaderEvaluation() {
	createProjectOptions := platform.CreateProjectOptions{
		RequestOrigin:        platformconfig.ProjectsLeaderKindMock,
		SkipLeaderEvaluation: true,
		ProjectConfig: &platform.ProjectConfig{
			Meta: platform.ProjectMeta{Name: "test-func"},
		},
	}

	suite.mockInternalProjectsClient.
		On("Create", suite.ctx, &createProjectOptions).
		Return(&platform.AbstractProject{}, nil).
		Once()

	_, err := suite.Create(suite.ctx, &createProjectOptions)
	suite.Require().NoError(err)

	suite.assertLeaderEvaluationSkipped()
}

// TestUpdateSkipsLeaderEvaluation mirrors the Create case for Update.
func (suite *ExternalProjectClientTestSuite) TestUpdateSkipsLeaderEvaluation() {
	updateProjectOptions := platform.UpdateProjectOptions{
		RequestOrigin:        platformconfig.ProjectsLeaderKindMock,
		SkipLeaderEvaluation: true,
		ProjectConfig: platform.ProjectConfig{
			Meta: platform.ProjectMeta{Name: "test-func"},
		},
	}

	suite.mockInternalProjectsClient.
		On("Update", suite.ctx, &updateProjectOptions).
		Return(&platform.AbstractProject{}, nil).
		Once()

	_, err := suite.Update(suite.ctx, &updateProjectOptions)
	suite.Require().NoError(err)

	suite.assertLeaderEvaluationSkipped()
}

// TestDeleteSkipsLeaderEvaluation mirrors the Create case for Delete.
func (suite *ExternalProjectClientTestSuite) TestDeleteSkipsLeaderEvaluation() {
	deleteProjectOptions := platform.DeleteProjectOptions{
		RequestOrigin:        platformconfig.ProjectsLeaderKindMock,
		SkipLeaderEvaluation: true,
		Meta:                 platform.ProjectMeta{Name: "test-func"},
	}

	suite.mockInternalProjectsClient.
		On("Delete", suite.ctx, &deleteProjectOptions).
		Return(nil).
		Once()

	err := suite.Delete(suite.ctx, &deleteProjectOptions)
	suite.Require().NoError(err)

	suite.assertLeaderEvaluationSkipped()
}

// TestNotLeaderCreateIgnoresSkipLeaderEvaluation asserts the security-tightening contract:
// a non-leader caller cannot use the x-mlrun-force-sync header to bypass leader
// forwarding. The request must be sent to the external leader as if the header were absent.
// Update and Delete use identical routing; one Create test is sufficient.
func (suite *ExternalProjectClientTestSuite) TestNotLeaderCreateIgnoresSkipLeaderEvaluation() {
	createProjectOptions := platform.CreateProjectOptions{
		RequestOrigin:        "not-leader",
		SkipLeaderEvaluation: true,
		ProjectConfig: &platform.ProjectConfig{
			Meta: platform.ProjectMeta{Name: "test-func"},
		},
	}

	suite.mockLeaderProjectsClient.
		On("Create", suite.ctx, &createProjectOptions).
		Return(nil, nil).
		Once()

	_, err := suite.Create(suite.ctx, &createProjectOptions)
	suite.Require().Error(err)
	suite.Require().Equal(err, platform.ErrSuccessfulCreateProjectLeader)

	suite.mockInternalProjectsClient.AssertNotCalled(suite.T(), "Create", mock.Anything, mock.Anything)
}

func (suite *ExternalProjectClientTestSuite) TestNotLeaderCreate() {
	createProjectOptions := platform.CreateProjectOptions{
		RequestOrigin: "not-leader",
		ProjectConfig: &platform.ProjectConfig{
			Meta: platform.ProjectMeta{Name: "test-func"},
		},
	}

	suite.mockLeaderProjectsClient.
		On("Create", suite.ctx, &createProjectOptions).
		Return(nil, nil).
		Once()

	_, err := suite.Create(suite.ctx, &createProjectOptions)
	suite.Require().Error(err)
	suite.Require().Equal(err, platform.ErrSuccessfulCreateProjectLeader)
}

func (suite *ExternalProjectClientTestSuite) TestNotLeaderUpdate() {
	updateProjectOptions := platform.UpdateProjectOptions{
		RequestOrigin: "not-leader",
		ProjectConfig: platform.ProjectConfig{
			Meta: platform.ProjectMeta{Name: "test-func"},
		},
	}

	suite.mockLeaderProjectsClient.
		On("Update", suite.ctx, &updateProjectOptions).
		Return(nil, nil).
		Once()

	_, err := suite.Update(suite.ctx, &updateProjectOptions)
	suite.Require().Error(err)
	suite.Require().Equal(err, platform.ErrSuccessfulUpdateProjectLeader)
}

func (suite *ExternalProjectClientTestSuite) TestNotLeaderDelete() {
	deleteProjectOptions := platform.DeleteProjectOptions{
		RequestOrigin: "not-leader",
		Meta:          platform.ProjectMeta{Name: "test-func"},
	}

	suite.mockLeaderProjectsClient.
		On("Delete", suite.ctx, &deleteProjectOptions).
		Return(nil).
		Once()

	err := suite.Delete(suite.ctx, &deleteProjectOptions)
	suite.Require().Error(err)
	suite.Require().Equal(err, platform.ErrSuccessfulDeleteProjectLeader)
}

func (suite *ExternalProjectClientTestSuite) TestGet() {
	getProjectOptions := platform.GetProjectsOptions{
		Meta: platform.ProjectMeta{Name: "test-func"},
	}

	suite.mockInternalProjectsClient.
		On("Get", suite.ctx, &getProjectOptions).
		Return([]platform.Project{}, nil).
		Once()

	_, err := suite.Get(suite.ctx, &getProjectOptions)
	suite.Require().NoError(err)
}

// expectGetExistingProject stubs the internal Get used by getExistingProject
// to return no existing project (the simplest valid 2PC pre-state), and tells the
// leader mock that 2PC is enabled so the external client performs the Get.
func (suite *ExternalProjectClientTestSuite) expectGetExistingProject(name string) {
	suite.mockLeaderProjectsClient.
		On("ProjectSync2PCEnabled").
		Return(true).
		Once()
	suite.mockInternalProjectsClient.
		On("Get", suite.ctx, mock.MatchedBy(func(opts *platform.GetProjectsOptions) bool {
			return opts != nil && opts.Meta.Name == name
		})).
		Return([]platform.Project{}, nil).
		Once()
}

// expectEvaluateLeaderRequest stubs the leader's 2PC evaluation to return
// shouldApply=true so the test exercises the apply branch.
func (suite *ExternalProjectClientTestSuite) expectEvaluateLeaderRequest(shouldApply bool) {
	suite.mockLeaderProjectsClient.
		On("EvaluateLeaderRequest", suite.ctx, mock.Anything, mock.Anything).
		Return(shouldApply, nil).
		Once()
}

// assertLeaderEvaluationSkipped asserts that none of the 2PC evaluation calls
// fired — neither the feature-flag probe, the Get for the existing CRD, nor the
// leader-side evaluation. Used by the SkipLeaderEvaluation tests to prove the skip
// is short-circuit-complete and not just a "skip the result" check.
func (suite *ExternalProjectClientTestSuite) assertLeaderEvaluationSkipped() {
	suite.mockLeaderProjectsClient.AssertNotCalled(suite.T(), "ProjectSync2PCEnabled")
	suite.mockInternalProjectsClient.AssertNotCalled(suite.T(), "Get", mock.Anything, mock.Anything)
	suite.mockLeaderProjectsClient.AssertNotCalled(suite.T(), "EvaluateLeaderRequest", mock.Anything, mock.Anything, mock.Anything)
}

func TestExternalProjectClientTestSuite(t *testing.T) {
	suite.Run(t, new(ExternalProjectClientTestSuite))
}
