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

type ClientTestSuite struct {
	suite.Suite
	logger logger.Logger
	client *Client
}

func (suite *ClientTestSuite) SetupSuite() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("test-mlrun-client")
	suite.Require().NoError(err)
	suite.client = NewClient(suite.logger)
}

func (suite *ClientTestSuite) TestGenerateProjectRequestBody() {
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
			result, err := suite.client.GenerateProjectRequestBody(testCase.project)
			if testCase.expectError {
				suite.Require().Error(err)
				suite.Require().Nil(result)
			} else {
				suite.Require().NoError(err)
				suite.Require().NotNil(result)
				var resultProject Project
				suite.Require().NoError(json.Unmarshal(result, &resultProject))
				suite.Require().Equal(testCase.project.Meta.Name, resultProject.Metadata.Name)
			}
		})
	}
}

func (suite *ClientTestSuite) TestGenerateProjectDeletionRequestBody() {
	suite.Run("ValidProjectName", func() {
		result, err := suite.client.GenerateProjectDeletionRequestBody("my-project")
		suite.Require().NoError(err)
		var project Project
		suite.Require().NoError(json.Unmarshal(result, &project))
		suite.Require().Equal("my-project", project.Metadata.Name)
	})
}

func (suite *ClientTestSuite) TestResolveCreateProjectResponse() {
	testCases := []struct {
		name        string
		body        []byte
		expectError bool
	}{
		{
			name: "ValidResponse",
			body: func() []byte {
				b, _ := json.Marshal(Project{Metadata: ProjectMetadata{Name: "test-project"}})
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
			resp, err := suite.client.ResolveCreateProjectResponse(context.TODO(), testCase.body)
			if testCase.expectError {
				suite.Require().Error(err)
				suite.Require().Nil(resp)
			} else {
				suite.Require().NoError(err)
				suite.Require().NotNil(resp)
				suite.Require().Equal(resp.GetLastJobID(), "")
				responseProject, ok := resp.(*Project)
				suite.Require().True(ok)
				suite.Require().Equal("test-project", responseProject.Metadata.Name)
			}
		})
	}
}

func (suite *ClientTestSuite) TestResolveGetProjectResponse() {
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
		projects, err := suite.client.ResolveGetProjectResponse(testCase.detail, testCase.body)
		suite.Require().Error(err)
		suite.Require().Nil(projects)
	}
}

func (suite *ClientTestSuite) TestParseJobStatusResponse() {
	resp, ok := suite.client.ParseJobStatusResponse(context.TODO(), nil)
	suite.Require().Nil(resp)
	suite.Require().False(ok)
}

func (suite *ClientTestSuite) TestGenerateCreateProjectRequestURL() {
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
			url := suite.client.GenerateCreateProjectRequestURL(testCase.address)
			suite.Require().Equal(testCase.expected, url)
		})
	}
}

func (suite *ClientTestSuite) TestHandleCreateResponseErr() {
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
			err := suite.client.HandleCreateResponseErr(context.TODO(), testCase.body, testCase.response, errors.New(""))
			suite.Require().Error(err)
			suite.Require().Equal(err.Error(), testCase.expectErrStr)
		})
	}
}

func (suite *ClientTestSuite) TestValidateJobState() {
	suite.Run("AlwaysNil", func() {
		err := suite.client.ValidateJobState(context.TODO(), nil, "")
		suite.Require().NoError(err)
	})
}

func (suite *ClientTestSuite) TestGenerateUpdateProjectRequestURL() {
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
			url := suite.client.GenerateUpdateProjectRequestURL(testCase.address, testCase.projectName)
			suite.Require().Equal(testCase.expected, url)
		})
	}
}

func (suite *ClientTestSuite) TestGetDeleteExpectedStatusCode() {
	suite.Run("AlwaysNoContent", func() {
		code := suite.client.GetDeleteExpectedStatusCode()
		suite.Require().Equal(http.StatusNoContent, code)
	})
}

func (suite *ClientTestSuite) TestAddDeleteStrategyHeader() {
	headers := map[string]string{}
	suite.client.AddDeleteStrategyHeader(headers, "testStrategy")
	suite.Require().Empty(headers)
}

func (suite *ClientTestSuite) TestGenerateGetProjectsRequestURL() {
	url := suite.client.GenerateGetProjectsRequestURL("a", "b")
	suite.Require().Equal("", url)
}

func (suite *ClientTestSuite) TestGenerateGetUpdatedAfterRequestURL() {
	url := suite.client.GenerateGetUpdatedAfterRequestURL("test")
	suite.Require().Equal("", url)
}

func (suite *ClientTestSuite) TestGenerateDeleteProjectRequestURL() {
	url := suite.client.GenerateDeleteProjectRequestURL("http://localhost", "test-project")
	suite.Require().Equal("http://localhost/projects/test-project", url)
}

func (suite *ClientTestSuite) TestShouldWaitForCreateCompletion() {
	suite.Require().False(suite.client.ShouldWaitForCreateCompletion())
}

func TestClientTestSuite(t *testing.T) {
	suite.Run(t, new(ClientTestSuite))
}
