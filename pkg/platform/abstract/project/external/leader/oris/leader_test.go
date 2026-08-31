//go:build test_unit

/*
Copyright 2026 The Nuclio Authors.

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

package oris

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type LeaderTestSuite struct {
	suite.Suite
	logger    logger.Logger
	leaderOps *LeaderOps
	namespace string
}

func (suite *LeaderTestSuite) SetupSuite() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("test-oris-leader")
	suite.Require().NoError(err)
	suite.namespace = "test-namespace"
	suite.leaderOps = NewLeaderOps(suite.logger, suite.namespace)
}

func (suite *LeaderTestSuite) TestGenerateProjectRequestBody() {
	tests := []struct {
		name        string
		project     *platform.ProjectConfig
		expectError bool
	}{
		{
			name: "ValidProject",
			project: &platform.ProjectConfig{
				Meta: platform.ProjectMeta{Name: "test"},
				Spec: platform.ProjectSpec{Owner: "jsmith", Description: "some description"},
			},
		},
		{
			name:        "NilProject",
			project:     nil,
			expectError: true,
		},
	}

	for _, testCase := range tests {
		suite.Run(testCase.name, func() {
			result, err := suite.leaderOps.GenerateProjectRequestBody(testCase.project)
			if testCase.expectError {
				suite.Require().Error(err)
				suite.Require().Nil(result)
			} else {
				suite.Require().NoError(err)
				suite.Require().NotNil(result)
				var resultProjectRequest OrisProjectRequest
				suite.Require().NoError(json.Unmarshal(result, &resultProjectRequest))
				suite.Require().Equal(testCase.project.Meta.Name, resultProjectRequest.Name)
				suite.Require().Equal(testCase.project.Spec.Owner, resultProjectRequest.Owner)
				suite.Require().Equal(testCase.project.Spec.Description, resultProjectRequest.Description)
			}
		})
	}
}

func (suite *LeaderTestSuite) TestGenerateProjectDeletionRequestBody() {
	suite.Run("ValidProjectName", func() {
		result, err := suite.leaderOps.GenerateProjectDeletionRequestBody("my-project")
		suite.Require().NoError(err)
		suite.Require().Empty(result)
	})
}

func (suite *LeaderTestSuite) TestResolveCreateProjectResponse() {
	testCases := []struct {
		name        string
		body        []byte
		expectError bool
	}{
		{
			name: "ValidResponse",
			body: func() []byte {
				b, _ := json.Marshal(OrisProject{
					Metadata: OrisProjectMetadata{Name: "test-project"},
					Spec:     OrisProjectSpec{Owner: "jsmith", Description: "some description"},
				})
				return b
			}(),
		},
		{
			name:        "InvalidResponse",
			body:        []byte(`not-json`),
			expectError: true,
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			resp, err := suite.leaderOps.ResolveCreateProjectResponse(context.TODO(), testCase.body)
			if testCase.expectError {
				suite.Require().Error(err)
				suite.Require().Nil(resp)
			} else {
				suite.Require().NoError(err)
				suite.Require().NotNil(resp)
				suite.Require().Equal("", resp.GetLastJobID())
				responseProject, ok := resp.(*OrisProject)
				suite.Require().True(ok)
				suite.Require().Equal("test-project", responseProject.Metadata.Name)
				suite.Require().Equal("jsmith", responseProject.Spec.Owner)
				suite.Require().Equal("some description", responseProject.Spec.Description)
			}
		})
	}
}

func (suite *LeaderTestSuite) TestResolveGetProjectResponse() {
	body := []byte(`{"items":[
		{"metadata":{"name":"proj-a","labels":{"team":"ds"},"annotations":{"note":"first"}},"spec":{"owner":"jsmith","description":"first project"}},
		{"metadata":{"name":"proj-b"},"spec":{"owner":"jdoe","description":"second project"}}
	]}`)
	projects, err := suite.leaderOps.ResolveGetProjectResponse(false, body)
	suite.Require().NoError(err)
	suite.Require().Len(projects, 2)
	for _, project := range projects {
		suite.Require().Equal(suite.namespace, project.GetConfig().Meta.Namespace)
	}

	firstConfig := projects[0].GetConfig()
	suite.Require().Equal("proj-a", firstConfig.Meta.Name)
	suite.Require().Equal(map[string]string{"team": "ds"}, firstConfig.Meta.Labels)
	suite.Require().Equal(map[string]string{"note": "first"}, firstConfig.Meta.Annotations)
	suite.Require().Equal("jsmith", firstConfig.Spec.Owner)
	suite.Require().Equal("first project", firstConfig.Spec.Description)
}

func (suite *LeaderTestSuite) TestHandleCreateResponseErr() {
	testCases := []struct {
		name         string
		body         []byte
		response     *http.Response
		expectErrStr string
	}{
		{
			name:         "UnmarshalFailure",
			body:         []byte(`not-json`),
			response:     &http.Response{StatusCode: http.StatusBadRequest},
			expectErrStr: "Failed to unmarshal response body",
		},
		{
			name: "ErrorMessagePresent",
			body: func() []byte {
				b, _ := json.Marshal(OrisProject{Status: OrisProjectStatus{ErrorMessage: "some error", StatusCode: http.StatusBadRequest}})
				return b
			}(),
			response:     &http.Response{StatusCode: http.StatusBadRequest},
			expectErrStr: "some error",
		},
		{
			name: "ErrorMessagePresentNilResponse",
			body: func() []byte {
				b, _ := json.Marshal(OrisProject{Status: OrisProjectStatus{ErrorMessage: "some error"}})
				return b
			}(),
			response:     nil,
			expectErrStr: "Failed to get response from leader, response is nil",
		},
		{
			name: "NoErrorMessage",
			body: func() []byte {
				b, _ := json.Marshal(OrisProject{Metadata: OrisProjectMetadata{Name: "test-project"}})
				return b
			}(),
			response:     &http.Response{StatusCode: http.StatusOK},
			expectErrStr: "Failed to send request to leader",
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			err := suite.leaderOps.HandleCreateResponseErr(context.TODO(), testCase.body, testCase.response, errors.New(""))
			suite.Require().Error(err)
			suite.Require().Equal(testCase.expectErrStr, err.Error())
		})
	}
}

func (suite *LeaderTestSuite) TestParseJobStatusResponse() {
	resp, ok := suite.leaderOps.ParseJobStatusResponse(context.TODO(), nil)
	suite.Require().Nil(resp)
	suite.Require().False(ok)
}

func (suite *LeaderTestSuite) TestGenerateCreateProjectRequestURL() {
	url := suite.leaderOps.GenerateCreateProjectRequestURL("http://localhost/api")
	suite.Require().Equal("http://localhost/api/v1/projects", url)
}

func (suite *LeaderTestSuite) TestGenerateUpdateProjectRequestURL() {
	url := suite.leaderOps.GenerateUpdateProjectRequestURL("http://localhost/api", "test-project")
	suite.Require().Equal("http://localhost/api/v1/projects/test-project", url)
}

func (suite *LeaderTestSuite) TestGenerateGetProjectsRequestURL() {
	url := suite.leaderOps.GenerateGetProjectsRequestURL("http://localhost/api", "test-project")
	suite.Require().Equal("http://localhost/api/v1/projects/test-project", url)
}

func (suite *LeaderTestSuite) TestGenerateDeleteProjectRequestURL() {
	url := suite.leaderOps.GenerateDeleteProjectRequestURL("http://localhost/api", "test-project")
	suite.Require().Equal("http://localhost/api/v1/projects/test-project", url)
}

func (suite *LeaderTestSuite) TestGenerateGetUpdatedAfterRequestURL() {
	url := suite.leaderOps.GenerateGetUpdatedAfterRequestURL("http://localhost/api")
	suite.Require().Equal("http://localhost/api/v1/projects", url)
}

func (suite *LeaderTestSuite) TestShouldWaitForCreateCompletion() {
	suite.Require().False(suite.leaderOps.ShouldWaitForCreateCompletion())
}

func (suite *LeaderTestSuite) TestGetDeleteExpectedStatusCode() {
	suite.Require().Equal(204, suite.leaderOps.GetDeleteExpectedStatusCode())
}

func (suite *LeaderTestSuite) TestProjectSync2PCEnabled() {
	suite.Require().False(suite.leaderOps.ProjectSync2PCEnabled())
}

func (suite *LeaderTestSuite) TestEvaluateLeaderRequestPassesThrough() {
	shouldApply, err := suite.leaderOps.EvaluateLeaderRequest(context.TODO(), map[string]string{}, nil)
	suite.Require().NoError(err)
	suite.Require().True(shouldApply)
}

func (suite *LeaderTestSuite) TestIsJobCompleted() {
	suite.Require().NoError(suite.leaderOps.IsJobCompleted(context.TODO(), nil, ""))
}

func TestLeaderTestSuite(t *testing.T) {
	suite.Run(t, new(LeaderTestSuite))
}
