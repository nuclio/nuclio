//go:build test_unit && test_broken

// TODO: fix below unit testings that fail on CI due to time drifting (not idempotent)
/*
	Various tests fail with TestLeaderProjectsDoesntExistInternally

	Error:      	Not equal:
	            	expected: &time.Time{wall:0x1a0ccafe, ext:63761762082, loc:(*time.Location)(nil)}
	            	actual  : &time.Time{wall:0x1a0ccaf0, ext:63761762082, loc:(*time.Location)(nil)}

	            	Diff:
	            	--- Expected
	            	+++ Actual
	            	@@ -1,3 +1,3 @@
	            	 (*time.Time)({
	            	- wall: (uint64) 437046014,
	            	+ wall: (uint64) 437046000,
	            	  ext: (int64) 63761762082,
*/

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
	testBeginningTime := time.Now().UTC()

	suite.testSynchronizeProjectsFromLeader("some-namespace",
		[]platform.Project{},
		[]platform.Project{
			suite.compileProject("test-internal-project",
				"",
				"online",
				testBeginningTime.Format(ProjectTimeLayout)),
		},
		[]*platform.CreateProjectOptions{},
		[]*platform.UpdateProjectOptions{},
		nil)
}

func (suite *SynchronizerTestSuite) TestLeaderProjectsDoesntExistInternally() {
	testBeginningTime := time.Now().UTC()
	testBeginningTimePlusOneHour := testBeginningTime.Add(time.Hour)

	namespace := "some-namespace"
	leaderProjectMostUpdated := suite.compileProject(
		"leader-project-most-updated",
		"",
		"online",
		testBeginningTimePlusOneHour.Format(ProjectTimeLayout))
	leaderProjectLessUpdated := suite.compileProject(
		"leader-project-less-updated",
		"",
		"online",
		testBeginningTime.Format(ProjectTimeLayout))

	suite.testSynchronizeProjectsFromLeader(
		namespace,
		[]platform.Project{leaderProjectMostUpdated, leaderProjectLessUpdated},
		[]platform.Project{},
		[]*platform.CreateProjectOptions{
			{
				ProjectConfig: func() *platform.ProjectConfig {
					platformConfig := leaderProjectMostUpdated.GetConfig()
					platformConfig.Meta.Namespace = namespace
					return platformConfig
				}(),
			},
			{
				ProjectConfig: func() *platform.ProjectConfig {
					platformConfig := leaderProjectLessUpdated.GetConfig()
					platformConfig.Meta.Namespace = namespace
					return platformConfig
				}(),
			},
		},
		[]*platform.UpdateProjectOptions{},
		&testBeginningTimePlusOneHour)
}

func (suite *SynchronizerTestSuite) TestLeaderProjectsNotUpdatedInternally() {
	testBeginningTime := time.Now().UTC()

	namespace := "some-namespace"
	updatedProject := suite.compileProject("leader-project",
		"updated",
		"online",
		testBeginningTime.Format(ProjectTimeLayout))
	updatedProject.(*Project).Data.Attributes.Namespace = "some-namespace"
	notUpdatedProject := suite.compileProject("leader-project",
		"not-updated",
		"online",
		testBeginningTime.Format(ProjectTimeLayout))
	notUpdatedProject.(*Project).Data.Attributes.Namespace = "some-namespace"
	suite.testSynchronizeProjectsFromLeader(
		namespace,
		[]platform.Project{updatedProject},
		[]platform.Project{notUpdatedProject},
		[]*platform.CreateProjectOptions{},
		[]*platform.UpdateProjectOptions{{
			ProjectConfig: func() platform.ProjectConfig {
				platformConfig := *updatedProject.GetConfig()
				platformConfig.Meta.Namespace = namespace
				return platformConfig
			}(),
		}},
		&testBeginningTime)
}

func (suite *SynchronizerTestSuite) TestLeaderProjectsThatExistInternally() {
	testBeginningTime := time.Now().UTC()

	projectInstance := suite.compileProject("leader-project",
		"updated",
		"online",
		testBeginningTime.Format(ProjectTimeLayout))

	namespace := "some-namespace"
	suite.testSynchronizeProjectsFromLeader(
		namespace,
		[]platform.Project{projectInstance},
		[]platform.Project{projectInstance},
		[]*platform.CreateProjectOptions{},
		[]*platform.UpdateProjectOptions{},
		&testBeginningTime)
}

func (suite *SynchronizerTestSuite) testSynchronizeProjectsFromLeader(namespace string,
	leaderProjects []platform.Project,
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
		On("Get", &platform.GetProjectsOptions{
			Meta: platform.ProjectMeta{
				Namespace: namespace,
			},
		}).
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

	newMostRecentUpdatedProjectTime, err := suite.synchronizer.synchronizeProjectsFromLeader(namespace, uninitializedTime)
	suite.Require().NoError(err)

	suite.Require().Equal(expectedNewMostRecentUpdatedProjectTime, newMostRecentUpdatedProjectTime)

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

	suite.mockInternalProjectsClient.AssertExpectations(suite.T())
}

func (suite *SynchronizerTestSuite) compileProject(name string,
	description string,
	status string,
	updatedAt string) platform.Project {

	return &Project{
		Data: ProjectData{
			Attributes: ProjectAttributes{
				Name:              name,
				Description:       description,
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
