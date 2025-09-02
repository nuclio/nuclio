//go:build test_unit

/*
Copyright 2025 The Nuclio Authors.

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

package mlrun

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type LeaderTestSuite struct {
	suite.Suite
	logger    logger.Logger
	leaderOps *LeaderOps
}

func (suite *LeaderTestSuite) SetupSuite() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("test-mlrun-leader")
	suite.Require().NoError(err)
	suite.leaderOps = NewLeaderOps(suite.logger)
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
				var resultProject MLRunProject
				suite.Require().NoError(json.Unmarshal(result, &resultProject))
				suite.Require().Equal(testCase.project.Meta.Name, resultProject.Metadata.Name)
			}
		})
	}
}

func (suite *LeaderTestSuite) TestGenerateProjectDeletionRequestBody() {
	suite.Run("ValidProjectName", func() {
		result, err := suite.leaderOps.GenerateProjectDeletionRequestBody("my-project")
		suite.Require().NoError(err)
		var project MLRunProject
		suite.Require().NoError(json.Unmarshal(result, &project))
		suite.Require().Equal("my-project", project.Metadata.Name)
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
				b, _ := json.Marshal(MLRunProject{Metadata: ProjectMetadata{Name: "test-project"}})
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
				suite.Require().Equal(resp.GetLastJobID(), "")
				responseProject, ok := resp.(*MLRunProject)
				suite.Require().True(ok)
				suite.Require().Equal("test-project", responseProject.Metadata.Name)
			}
		})
	}
}

func (suite *LeaderTestSuite) TestResolveGetProjectResponse() {
	testCases := []struct {
		name   string
		body   []byte
		detail bool
	}{
		{
			name:   "DetailTrue",
			detail: true,
			body:   []byte(`{}`),
		},
		{
			name:   "DetailFalse",
			detail: false,
			body:   []byte(`{}`),
		},
		{
			name:   "EmptyBody",
			detail: false,
			body:   nil,
		},
	}

	for _, testCase := range testCases {
		projects, err := suite.leaderOps.ResolveGetProjectResponse(testCase.detail, testCase.body)
		suite.Require().Error(err)
		suite.Require().Nil(projects)
	}
}

func (suite *LeaderTestSuite) TestParseJobStatusResponse() {
	resp, ok := suite.leaderOps.ParseJobStatusResponse(context.TODO(), nil)
	suite.Require().Nil(resp)
	suite.Require().False(ok)
}

func (suite *LeaderTestSuite) TestGenerateCreateProjectRequestURL() {
	testCases := []struct {
		name     string
		address  string
		expected string
	}{
		{
			name:     "Basic",
			address:  "http://localhost",
			expected: "http://localhost/projects",
		},
		{
			name:     "WithoutHttpPrefix",
			address:  "some-address",
			expected: "some-address/projects",
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			url := suite.leaderOps.GenerateCreateProjectRequestURL(testCase.address)
			suite.Require().Equal(testCase.expected, url)
		})
	}
}

func (suite *LeaderTestSuite) TestHandleCreateResponseErr() {
	testCases := []struct {
		name         string
		body         []byte
		response     *http.Response
		expectErrStr string
	}{
		{
			name: "MLRunError",
			body: func() []byte {
				b, _ := json.Marshal(MlrunError{Detail: "some error"})
				return b
			}(),
			response:     &http.Response{StatusCode: http.StatusBadRequest},
			expectErrStr: "some error",
		},
		{
			name:         "NonMLRunError",
			body:         []byte(`not-json`),
			response:     &http.Response{StatusCode: http.StatusBadRequest},
			expectErrStr: "Failed to send request to leader",
		},
		{
			name: "NilResponse",
			body: func() []byte {
				b, _ := json.Marshal(MlrunError{Detail: "some error"})
				return b
			}(),
			response:     nil,
			expectErrStr: "Failed to get response from leader, response is nil",
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			err := suite.leaderOps.HandleCreateResponseErr(context.TODO(), testCase.body, testCase.response, errors.New(""))
			suite.Require().Error(err)
			suite.Require().Equal(err.Error(), testCase.expectErrStr)
		})
	}
}

func (suite *LeaderTestSuite) TestIsJobCompleted() {
	suite.Run("AlwaysNil", func() {
		err := suite.leaderOps.IsJobCompleted(context.TODO(), nil, "")
		suite.Require().NoError(err)
	})
}

func (suite *LeaderTestSuite) TestGenerateUpdateProjectRequestURL() {
	testCases := []struct {
		name        string
		address     string
		projectName string
		expected    string
	}{
		{
			name:        "Basic",
			address:     "http://localhost",
			projectName: "test-project",
			expected:    "http://localhost/projects/test-project",
		},
		{
			name:        "WithEmptyUrl",
			address:     "",
			projectName: "test-project",
			expected:    "/projects/test-project",
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			url := suite.leaderOps.GenerateUpdateProjectRequestURL(testCase.address, testCase.projectName)
			suite.Require().Equal(testCase.expected, url)
		})
	}
}

func (suite *LeaderTestSuite) TestGetDeleteExpectedStatusCode() {
	suite.Run("AlwaysNoContent", func() {
		code := suite.leaderOps.GetDeleteExpectedStatusCode()
		suite.Require().Equal(http.StatusNoContent, code)
	})
}

func (suite *LeaderTestSuite) TestGetDeleteStrategyHeaderName() {
	header := suite.leaderOps.GetDeleteStrategyHeaderName()
	suite.Require().Equal(header, "x-mlrun-deletion-strategy")
}

func (suite *LeaderTestSuite) TestGenerateGetProjectsRequestURL() {
	url := suite.leaderOps.GenerateGetProjectsRequestURL("a", "b")
	suite.Require().Equal("", url)
}

func (suite *LeaderTestSuite) TestGenerateGetUpdatedAfterRequestURL() {
	url := suite.leaderOps.GenerateGetUpdatedAfterRequestURL("test")
	suite.Require().Equal("", url)
}

func (suite *LeaderTestSuite) TestGenerateDeleteProjectRequestURL() {
	url := suite.leaderOps.GenerateDeleteProjectRequestURL("http://localhost", "test-project")
	suite.Require().Equal("http://localhost/projects/test-project", url)
}

func (suite *LeaderTestSuite) TestShouldWaitForCreateCompletion() {
	suite.Require().False(suite.leaderOps.ShouldWaitForCreateCompletion())
}

func TestLeaderTestSuite(t *testing.T) {
	suite.Run(t, new(LeaderTestSuite))
}
