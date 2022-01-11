//go:build test_unit

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
	"github.com/nuclio/zap"
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

func (suite *ExternalProjectClientTestSuite) SetupSuite() {
	var err error

	// create logger
	suite.Logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	// create context
	suite.ctx = context.Background()

	// mock internal client
	suite.mockInternalProjectsClient = &internalmock.Client{}

	//mock leader client
	suite.mockLeaderProjectsClient = &leadermock.Client{}

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
	suite.Require().NoError(err)
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

	suite.mockInternalProjectsClient.
		On("Create", suite.ctx, &createProjectOptions).
		Return(&platform.AbstractProject{}, nil).
		Once()

	_, err := suite.Client.Create(suite.ctx, &createProjectOptions)
	suite.Require().NoError(err)
}

func (suite *ExternalProjectClientTestSuite) TestLeaderUpdate() {
	updateProjectOptions := platform.UpdateProjectOptions{
		RequestOrigin: platformconfig.ProjectsLeaderKindMock,
		ProjectConfig: platform.ProjectConfig{
			Meta: platform.ProjectMeta{
				Name: "test-func",
			},
		},
	}

	suite.mockInternalProjectsClient.
		On("Update", suite.ctx, &updateProjectOptions).
		Return(&platform.AbstractProject{}, nil).
		Once()

	_, err := suite.Client.Update(suite.ctx, &updateProjectOptions)
	suite.Require().NoError(err)
}

func (suite *ExternalProjectClientTestSuite) TestLeaderDelete() {
	deleteProjectOptions := platform.DeleteProjectOptions{
		RequestOrigin: platformconfig.ProjectsLeaderKindMock,
		Meta: platform.ProjectMeta{
			Name: "test-func",
		},
	}

	suite.mockInternalProjectsClient.
		On("Delete", suite.ctx, &deleteProjectOptions).
		Return(nil).
		Once()

	err := suite.Client.Delete(suite.ctx, &deleteProjectOptions)
	suite.Require().NoError(err)
}

func (suite *ExternalProjectClientTestSuite) TestNotLeaderCreate() {
	createProjectOptions := platform.CreateProjectOptions{
		RequestOrigin: "not-leader",
		ProjectConfig: &platform.ProjectConfig{
			Meta: platform.ProjectMeta{
				Name: "test-func",
			},
		},
	}

	suite.mockLeaderProjectsClient.
		On("Create", &createProjectOptions).
		Return(nil, nil).
		Once()

	_, err := suite.Client.Create(suite.ctx, &createProjectOptions)
	suite.Require().Error(err)
	suite.Require().Equal(err, platform.ErrSuccessfulCreateProjectLeader)
}

func (suite *ExternalProjectClientTestSuite) TestNotLeaderUpdate() {
	updateProjectOptions := platform.UpdateProjectOptions{
		RequestOrigin: "not-leader",
		ProjectConfig: platform.ProjectConfig{
			Meta: platform.ProjectMeta{
				Name: "test-func",
			},
		},
	}

	suite.mockLeaderProjectsClient.
		On("Update", &updateProjectOptions).
		Return(nil, nil).
		Once()

	_, err := suite.Client.Update(suite.ctx, &updateProjectOptions)
	suite.Require().Error(err)
	suite.Require().Equal(err, platform.ErrSuccessfulUpdateProjectLeader)
}

func (suite *ExternalProjectClientTestSuite) TestNotLeaderDelete() {
	deleteProjectOptions := platform.DeleteProjectOptions{
		RequestOrigin: "not-leader",
		Meta: platform.ProjectMeta{
			Name: "test-func",
		},
	}

	suite.mockLeaderProjectsClient.
		On("Delete", &deleteProjectOptions).
		Return(nil).
		Once()

	err := suite.Client.Delete(suite.ctx, &deleteProjectOptions)
	suite.Require().Error(err)
	suite.Require().Equal(err, platform.ErrSuccessfulDeleteProjectLeader)
}

func (suite *ExternalProjectClientTestSuite) TestGet() {
	getProjectOptions := platform.GetProjectsOptions{
		Meta: platform.ProjectMeta{
			Name: "test-func",
		},
	}

	suite.mockInternalProjectsClient.
		On("Get", suite.ctx, &getProjectOptions).
		Return([]platform.Project{}, nil).
		Once()

	_, err := suite.Client.Get(suite.ctx, &getProjectOptions)
	suite.Require().NoError(err)
}

func TestExternalProjectClientTestSuite(t *testing.T) {
	suite.Run(t, new(ExternalProjectClientTestSuite))
}
