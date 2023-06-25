//go:build test_unit && test_broken

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
	"context"
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
				"some-namespace",
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
		namespace,
		"",
		"online",
		testBeginningTimePlusOneHour.Format(ProjectTimeLayout))
	leaderProjectLessUpdated := suite.compileProject(
		"leader-project-less-updated",
		namespace,
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
		namespace,
		"updated",
		"online",
		testBeginningTime.Format(ProjectTimeLayout))
	updatedProject.(*Project).Data.Attributes.Namespace = "some-namespace"
	notUpdatedProject := suite.compileProject("leader-project",
		namespace,
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
		"some-namespace",
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

func (suite *SynchronizerTestSuite) TestFilterInvalidLabels() {
	invalidLabels := map[string]string{
		"my@weird/label": "value",
		"my.wierd/label": "value@",
		"%weird+/label":  "v8$alue",
	}

	labels := map[string]string{
		"valid":          "label",
		"another-valid":  "label-value",
		"also_123_valid": "label_456_value",
	}

	// add invalid labels to labels
	for key, value := range invalidLabels {
		labels[key] = value
	}

	filteredLabels := suite.synchronizer.filterInvalidLabels(labels)

	suite.Require().Equal(len(filteredLabels), len(labels)-len(invalidLabels))
	for key := range invalidLabels {
		_, ok := filteredLabels[key]
		suite.Require().False(ok, "invalid label %s should not be in filtered labels", key)
	}
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
		On("GetUpdatedAfter", mock.Anything, uninitializedTime).
		Return(leaderProjects, nil).
		Once()

	// mock internal client get projects
	suite.mockInternalProjectsClient.
		On("Get",
			mock.Anything,
			&platform.GetProjectsOptions{
				Meta: platform.ProjectMeta{
					Namespace: namespace,
				},
			}).
		Return(internalProjects, nil).
		Once()

	// mock internal client create project for each expected project to create
	for _, createProjectOptions := range projectsToCreate {
		suite.mockInternalProjectsClient.
			On("Create", mock.Anything, createProjectOptions).
			Return(&platform.AbstractProject{}, nil).
			Once()
	}

	// mock internal client update project for each expected project to update
	for _, updateProjectOptions := range projectsToUpdate {
		suite.mockInternalProjectsClient.
			On("Update", mock.Anything, updateProjectOptions).
			Return(&platform.AbstractProject{}, nil).
			Once()
	}

	newMostRecentUpdatedProjectTime, err := suite.synchronizer.synchronizeProjectsFromLeader(
		context.TODO(),
		namespace, uninitializedTime)
	suite.Require().NoError(err)

	suite.Require().Equal(expectedNewMostRecentUpdatedProjectTime, newMostRecentUpdatedProjectTime)

	// sleep for 1 second so every mock create/update go routine would finish
	time.Sleep(1 * time.Second)

	// assert that it created all expected projects
	for _, createProjectOptions := range projectsToCreate {
		suite.mockInternalProjectsClient.AssertCalled(suite.T(), "Create", mock.Anything, createProjectOptions)
	}
	if len(projectsToCreate) == 0 {
		suite.mockInternalProjectsClient.AssertNotCalled(suite.T(), "Create", mock.Anything, mock.Anything)
	}

	// assert that it updated all expected projects
	for _, updateProjectOptions := range projectsToUpdate {
		suite.mockInternalProjectsClient.AssertCalled(suite.T(), "Update", mock.Anything, updateProjectOptions)
	}
	if len(projectsToUpdate) == 0 {
		suite.mockInternalProjectsClient.AssertNotCalled(suite.T(), "Update", mock.Anything, mock.Anything)
	}

	suite.mockInternalProjectsClient.AssertExpectations(suite.T())
}

func (suite *SynchronizerTestSuite) compileProject(name string,
	namespace string,
	description string,
	status string,
	updatedAt string) platform.Project {

	return &Project{
		Data: ProjectData{
			Attributes: ProjectAttributes{
				Name:              name,
				Namespace:         namespace,
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
