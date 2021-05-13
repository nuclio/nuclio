package iguazio

import (
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/platform"
	leadermock "github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader/mock"
	internalmock "github.com/nuclio/nuclio/pkg/platform/abstract/project/mock"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type SynchronizerTestSuite struct {
	suite.Suite

	synchronizer               Synchronizer
	logger                     logger.Logger
	mockInternalProjectsClient *internalmock.Client
	mockLeaderProjectsClient   *leadermock.Client
}

func (suite *SynchronizerTestSuite) SetupTest() {
	var err error

	// create logger
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	// mock internal client
	suite.mockInternalProjectsClient = &internalmock.Client{}

	//mock leader client
	suite.mockLeaderProjectsClient = &leadermock.Client{}

	// create synchronizer
	suite.synchronizer = Synchronizer{
		logger:                     suite.logger,
		internalProjectsClient:     suite.mockInternalProjectsClient,
		leaderClient:               suite.mockLeaderProjectsClient,
		synchronizationIntervalStr: "0",
	}
}

func (suite *SynchronizerTestSuite) TestNoLeaderProjects() {
	testBeginningTime := time.Now()

	suite.testSynchronizeProjectsFromLeader([]platform.Project{},
		[]platform.Project{
			suite.createAbstractProject("test-internal-project", "", "online", testBeginningTime),
		},
		[]*platform.CreateProjectOptions{},
		[]*platform.UpdateProjectOptions{},
		nil)
}

func (suite *SynchronizerTestSuite) TestLeaderProjectsDoesntExistInternally() {
	testBeginningTime := time.Now()
	testBeginningTimePlusOneHour := testBeginningTime.Add(time.Hour)

	leaderProjectMostUpdated := suite.createAbstractProject("leader-project-most-updated", "", "online", testBeginningTimePlusOneHour)
	leaderProjectLessUpdated := suite.createAbstractProject("leader-project-less-updated", "", "online", testBeginningTime)

	suite.testSynchronizeProjectsFromLeader(
		[]platform.Project{leaderProjectMostUpdated, leaderProjectLessUpdated},
		[]platform.Project{},
		[]*platform.CreateProjectOptions{
			{
				ProjectConfig: &leaderProjectMostUpdated.ProjectConfig,
			},
			{
				ProjectConfig: &leaderProjectLessUpdated.ProjectConfig,
			},
		},
		[]*platform.UpdateProjectOptions{},
		&testBeginningTimePlusOneHour)
}

func (suite *SynchronizerTestSuite) TestLeaderProjectsIsntUpdatedInternally() {
	testBeginningTime := time.Now()

	updatedProject := suite.createAbstractProject("leader-project", "updated", "online", testBeginningTime)
	notUpdatedProject := suite.createAbstractProject("leader-project", "not-updated", "online", testBeginningTime)

	suite.testSynchronizeProjectsFromLeader(
		[]platform.Project{updatedProject},
		[]platform.Project{notUpdatedProject},
		[]*platform.CreateProjectOptions{},
		[]*platform.UpdateProjectOptions{{ProjectConfig: updatedProject.ProjectConfig}},
		&testBeginningTime)
}

func (suite *SynchronizerTestSuite) TestLeaderProjectsThatExistInternally() {
	testBeginningTime := time.Now()

	projectInstance := suite.createAbstractProject("leader-project", "updated", "online", testBeginningTime)

	suite.testSynchronizeProjectsFromLeader(
		[]platform.Project{projectInstance},
		[]platform.Project{projectInstance},
		[]*platform.CreateProjectOptions{},
		[]*platform.UpdateProjectOptions{},
		&testBeginningTime)
}

func (suite *SynchronizerTestSuite) testSynchronizeProjectsFromLeader(leaderProjects []platform.Project,
	internalProjects []platform.Project,
	projectsToCreate []*platform.CreateProjectOptions,
	projectsToUpdate []*platform.UpdateProjectOptions,
	expectedNewMostRecentUpdatedProjectTime *time.Time) {
	var uninitializedTime *time.Time

	// mock leader client get projects
	suite.mockLeaderProjectsClient.
		On("GetUpdatedAfter", uninitializedTime).
		Return(leaderProjects, nil).
		Once()

	// mock internal client get projects
	suite.mockInternalProjectsClient.
		On("Get", &platform.GetProjectsOptions{}).
		Return(internalProjects, nil).
		Once()

	// mock internal client create project for each expected project to create
	for _, createProjectOptions := range projectsToCreate {
		suite.mockInternalProjectsClient.
			On("Create", createProjectOptions).
			Return(&platform.AbstractProject{}, nil).
			Once()
	}

	// mock internal client update project for each expected project to update
	for _, updateProjectOptions := range projectsToUpdate {
		suite.mockInternalProjectsClient.
			On("Update", updateProjectOptions).
			Return(&platform.AbstractProject{}, nil).
			Once()
	}

	newMostRecentUpdatedProjectTime, err := suite.synchronizer.SynchronizeProjectsFromLeader(uninitializedTime)
	suite.Require().NoError(err)

	suite.Require().Equal(newMostRecentUpdatedProjectTime, expectedNewMostRecentUpdatedProjectTime)

	// sleep for 1 second so every mock create/update go routine would finish
	time.Sleep(1 * time.Second)

	// assert that it created all expected projects
	for _, createProjectOptions := range projectsToCreate {
		suite.mockInternalProjectsClient.AssertCalled(suite.T(), "Create", createProjectOptions)
	}
	if len(projectsToCreate) == 0 {
		suite.mockInternalProjectsClient.AssertNotCalled(suite.T(), "Create", mock.Anything)
	}

	// assert that it updated all expected projects
	for _, updateProjectOptions := range projectsToUpdate {
		suite.mockInternalProjectsClient.AssertCalled(suite.T(), "Update", updateProjectOptions)
	}
	if len(projectsToUpdate) == 0 {
		suite.mockInternalProjectsClient.AssertNotCalled(suite.T(), "Update", mock.Anything)
	}
}

func (suite *SynchronizerTestSuite) createAbstractProject(name string,
	description string,
	status string,
	updatedAt time.Time) *platform.AbstractProject {

	return &platform.AbstractProject{
		ProjectConfig: platform.ProjectConfig{
			Meta: platform.ProjectMeta{
				Name: name,
			},
			Spec: platform.ProjectSpec{
				Description: description,
			},
			Status: platform.ProjectStatus{
				OperationalStatus: status,
				AdminStatus:       status,
				UpdatedAt:         updatedAt,
			},
		},
	}
}

func TestSynchronizerTestSuite(t *testing.T) {
	suite.Run(t, new(SynchronizerTestSuite))
}
