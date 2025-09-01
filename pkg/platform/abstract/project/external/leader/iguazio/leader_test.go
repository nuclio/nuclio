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

package iguazio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/nuclio/nuclio/pkg/platform"
	leaderCommon "github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

const (
	detailedResponseTestCase  = "DetailedResponse"
	createOkJobFailedTestCase = "CreateOkJobFailed"
)

type LeaderTestSuite struct {
	suite.Suite

	logger logger.Logger
	leader *Leader
}

func (suite *LeaderTestSuite) SetupTest() {
	var err error

	// create logger
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	suite.leader = NewLeader(suite.logger)
}

func (suite *LeaderTestSuite) TestGenerateProjectRequestBody() {
	testCases := []struct {
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

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			result, err := suite.leader.GenerateProjectRequestBody(testCase.project)
			if testCase.expectError {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
				suite.Require().NotNil(result)
			}
		})
	}
}

func (suite *LeaderTestSuite) TestGenerateProjectDeletionRequestBody() {
	testCases := []struct {
		name        string
		projectName string
	}{
		{
			name:        "ValidProjectName",
			projectName: "my-project",
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			result, err := suite.leader.GenerateProjectDeletionRequestBody(testCase.projectName)
			suite.Require().NoError(err)
			suite.Require().NotNil(result)
		})
	}
}

func (suite *LeaderTestSuite) TestResolveCreateProjectResponse() {
	testCases := []struct {
		name        string
		body        []byte
		expectError bool
	}{
		{
			name: "ValidResponse",
			body: []byte(`{"data":{"type":"project","id":"id","attributes":{"name":"test"}},"meta":{"ctx":"ctx"}}`),
		},
		{
			name:        "InvalidResponse",
			body:        []byte(`not-json`),
			expectError: true,
		},
		{
			name: "DetailedResponse",
			body: suite.mockIgzAPIResponseBody(detailedResponseTestCase),
		},
		{
			name: "BadResponse",
			body: []byte(`{
    		"errors": [
				{ "status": 400, "detail": "Failed to get user id for username" }
    		],
			"meta": {
        		"ctx": "1234567890"
    			}
			}`),
		},
		{
			name: "CreateOkJobFailed",
			body: suite.mockIgzAPIResponseBody(createOkJobFailedTestCase),
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			resp, err := suite.leader.ResolveCreateProjectResponse(context.TODO(), testCase.body)
			if testCase.expectError {
				suite.Require().Error(err)
				suite.Require().Nil(resp)
			} else {
				suite.Require().NoError(err)
				suite.Require().NotNil(resp)
			}
		})
	}
}

func (suite *LeaderTestSuite) TestResolveGetProjectResponse() {
	testCases := []struct {
		name       string
		detail     bool
		body       []byte
		shouldFail bool
	}{
		{
			name:   "DetailTrue",
			detail: true,
		},
		{
			name:   "DetailFalse",
			detail: false,
		},
		{
			name:       "InvalidBody",
			detail:     false,
			body:       []byte(`not-json`),
			shouldFail: true,
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			body := testCase.body
			if body == nil {
				var err error
				body, err = io.ReadAll(suite.mockIgzAPIGetProject(testCase.detail))
				suite.Require().NoError(err)
			}

			projects, err := suite.leader.ResolveGetProjectResponse(testCase.detail, body)
			if testCase.shouldFail {
				suite.Require().Error(err)
				suite.Require().Nil(projects)
			} else {
				suite.Require().NoError(err)
				suite.Require().NotNil(projects)
			}
		})
	}
}

func (suite *LeaderTestSuite) TestParseJobStatusResponse() {
	testCases := []struct {
		name        string
		body        []byte
		expectValid bool
	}{
		{
			name:        "ValidJob",
			body:        []byte(`{"data":{"attributes":{"state":"completed"}},"meta":{"ctx":"ctx"}}`),
			expectValid: true,
		},
		{
			name:        "InvalidJob",
			body:        []byte(`not-json`),
			expectValid: false,
		},
		{
			name:        "InternalServerErrorJob",
			body:        []byte(""),
			expectValid: false,
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			job, valid := suite.leader.IsJobTerminated(context.TODO(), testCase.body)
			if testCase.expectValid {
				suite.Require().NotNil(job)
				suite.Require().True(valid)
			} else {
				suite.Require().Nil(job)
				suite.Require().False(valid)
			}
		})
	}
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
			name:     "WithTrailingSlash",
			address:  "http://localhost/",
			expected: "http://localhost//projects",
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			url := suite.leader.GenerateCreateProjectRequestURL(testCase.address)
			suite.Require().Equal(testCase.expected, url)
		})
	}
}

func (suite *LeaderTestSuite) TestHandleCreateResponseErr() {
	testCases := []struct {
		name         string
		body         []byte
		expectErrStr string
	}{
		{
			name:         "WithErrorResponse",
			body:         suite.mockIgzAPIResponseBody(detailedResponseTestCase),
			expectErrStr: "Failed to send request to leader",
		},
		{
			name:         "WithNoErrorResponse",
			body:         []byte(`not-json`),
			expectErrStr: "Failed to send request to leader",
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			err := suite.leader.HandleCreateResponseErr(context.TODO(), testCase.body, nil, fmt.Errorf("fail"))
			suite.Require().Error(err)
			suite.Require().Equal(err.Error(), testCase.expectErrStr)
		})
	}
}

func (suite *LeaderTestSuite) TestGetJobIdUrl() {
	testCases := []struct {
		name    string
		address string
		jobID   string
		expect  string
	}{
		{
			name:    "Basic",
			address: "http://localhost",
			jobID:   "job123",
			expect:  "http://localhost/jobs/job123",
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			url := suite.leader.GetJobIdUrl(testCase.address, testCase.jobID)
			suite.Require().Equal(testCase.expect, url)
		})
	}
}

func (suite *LeaderTestSuite) TestValidateJobState() {
	testCases := []struct {
		name      string
		job       leaderCommon.JobResponse
		expectErr bool
	}{
		{
			name:      "NilJob",
			job:       nil,
			expectErr: true,
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			err := suite.leader.ValidateJobState(context.TODO(), testCase.job, "test-project")
			if testCase.expectErr {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
			}
		})
	}
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
			projectName: "proj1",
			expected:    "http://localhost/projects/__name__/proj1",
		},
		{
			name:        "WithTrailingSlash",
			address:     "http://localhost/",
			projectName: "proj2",
			expected:    "http://localhost//projects/__name__/proj2",
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			url := suite.leader.GenerateUpdateProjectRequestURL(testCase.address, testCase.projectName)
			suite.Require().Equal(testCase.expected, url)
		})
	}
}

func (suite *LeaderTestSuite) TestGetDeleteExpectedStatusCode() {
	suite.Run("AlwaysAccepted", func() {
		code := suite.leader.GetDeleteExpectedStatusCode()
		suite.Require().Equal(http.StatusAccepted, code)
	})
}

func (suite *LeaderTestSuite) TestGetDeleteStrategyHeaderName() {
	header := suite.leader.GetDeleteStrategyHeaderName()
	suite.Require().Equal(header, "igz-project-deletion-strategy")
}

func (suite *LeaderTestSuite) TestGenerateGetProjectsRequestURL() {
	testCases := []struct {
		name        string
		address     string
		projectName string
		expected    string
	}{
		{
			name:        "WithProjectName",
			address:     "http://localhost",
			projectName: "proj1",
			expected:    "http://localhost/projects/__name__/proj1?include=owner&enrich_namespace=true",
		},
		{
			name:        "WithoutProjectName",
			address:     "http://localhost",
			projectName: "",
			expected:    "http://localhost/projects?include=owner&enrich_namespace=true",
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			url := suite.leader.GenerateGetProjectsRequestURL(testCase.address, testCase.projectName)
			suite.Require().Equal(testCase.expected, url)
		})
	}
}

func (suite *LeaderTestSuite) TestGenerateGetUpdatedAfterRequestURL() {
	testCases := []struct {
		name    string
		address string
		expect  string
	}{
		{
			name:    "Basic",
			address: "http://localhost",
			expect:  "http://localhost/projects?include=owner&enrich_namespace=true",
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			url := suite.leader.GenerateGetUpdatedAfterRequestURL(testCase.address)
			suite.Require().Equal(testCase.expect, url)
		})
	}
}

func (suite *LeaderTestSuite) TestGenerateDeleteProjectRequestURL() {
	testCases := []struct {
		name    string
		address string
		expect  string
	}{
		{
			name:    "Basic",
			address: "http://localhost",
			expect:  "http://localhost/projects",
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			url := suite.leader.GenerateDeleteProjectRequestURL(testCase.address, "")
			suite.Require().Equal(testCase.expect, url)
		})
	}
}

func (suite *LeaderTestSuite) TestShouldWaitForCreateCompletion() {
	suite.Run("AlwaysTrue", func() {
		suite.Require().True(suite.leader.ShouldWaitForCreateCompletion())
	})
}

func (suite *LeaderTestSuite) mockIgzAPIResponseBody(testCase string) []byte {
	var rawData *bytes.Buffer
	switch testCase {
	case detailedResponseTestCase:
		rawData = bytes.NewBufferString(`{
    "data": {
        "type": "project",
        "id": "e0d2a03d-884b-44e3-aa78-9c7cea0c0cf1",
        "attributes": {
            "name": "some-dummy-project",
            "description": "an example project",
            "created_at": "2021-08-23T19:39:50.522000+00:00",
            "updated_at": "2021-08-23T19:39:50.608000+00:00",
            "admin_status": "online",
            "operational_status": "creating",
            "labels": [],
            "annotations": []
        },
        "relationships": {
            "owner": {
                "data": {
                    "type": "user",
                    "id": "4274ecab-633a-4e99-8533-5df2e59bb358"
                }
            },
            "tenant": {
                "data": {
                    "type": "tenant",
                    "id": "b7c663b1-a8ee-49a9-ad62-ceae7e751ec8"
                }
            },
            "project_group": {
                "data": {
                    "type": "project_group",
                    "id": "33c160ff-86e8-4152-9456-faa751592bc0"
                }
            },
            "last_job": {
                "data": {
                    "type": "job",
                    "id": "some-job-id"
                }
            }
        }
    },
    "included": [],
    "meta": {
        "ctx": "13756324163199886387"
    }
	}`)
	case createOkJobFailedTestCase:
		rawData = bytes.NewBufferString(`{
    "data": {
        "type": "project",
        "id": "e0d2a03d-884b-44e3-aa78-9c7cea0c0cf1",
        "attributes": {
            "name": "some-dummy-project",
            "description": "an example project",
            "created_at": "2021-08-23T19:39:50.522000+00:00",
            "updated_at": "2021-08-23T19:39:50.608000+00:00",
            "admin_status": "online",
            "operational_status": "creating",
            "labels": [],
            "annotations": []
        },
        "relationships": {
            "owner": {
                "data": {
                    "type": "user",
                    "id": "4274ecab-633a-4e99-8533-5df2e59bb358"
                }
            },
            "tenant": {
                "data": {
                    "type": "tenant",
                    "id": "b7c663b1-a8ee-49a9-ad62-ceae7e751ec8"
                }
            },
            "project_group": {
                "data": {
                    "type": "project_group",
                    "id": "33c160ff-86e8-4152-9456-faa751592bc0"
                }
            },
            "last_job": {
                "data": {
                    "type": "job",
                    "id": "some-job-id"
                }
            }
        }
    },
    "included": [],
    "meta": {
        "ctx": "13756324163199886387"
    }
	}`)
	}
	return rawData.Bytes()
}

func (suite *LeaderTestSuite) mockIgzAPIGetProject(detail bool) io.ReadCloser {
	projectData := `{
        "attributes": {
            "admin_status": "online",
            "annotations": [],
            "created_at": "2021-08-12T07:13:19.620000+00:00",
            "labels": [],
            "name": "a1",
            "operational_status": "online",
            "owner_username": "admin",
            "updated_at": "0000-00-00T00:00:00.000000+00:00"
        },
        "id": "798d8441-1ca6-407d-8e8a-5ac24ba41ece",
        "relationships": {
            "owner": {
                "data": {
                    "id": "f595477c-945b-44c5-bf87-d6e4052409af",
                    "type": "user"
                }
            }
        },
        "type": "project"
    }`
	responseTemplate := `{"data": %s, "included": [], "meta": {"ctx": "11493070626596053818"}}`

	if detail {
		return io.NopCloser(bytes.NewBufferString(fmt.Sprintf(responseTemplate, projectData)))
	}

	return io.NopCloser(bytes.NewBufferString(fmt.Sprintf(responseTemplate, "["+projectData+"]")))
}

func TestLeaderTestSuite(t *testing.T) {
	suite.Run(t, new(LeaderTestSuite))
}
