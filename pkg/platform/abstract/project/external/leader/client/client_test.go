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

package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/common/testutils"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader"
	mockClient "github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader/mock"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

const (
	projectSuffix          = "/projects"
	getCreateProjectSuffix = "/jobs/some-job-id"
	createTestSuite        = "create"
	getTestSuite           = "get"
	getUpdatedAfter        = "get-updated-after"
	updateTestSuite        = "update"
	deleteTestSuite        = "delete"
)

type ClientTestSuite struct {
	suite.Suite

	logger logger.Logger
	client *Client
}

func (suite *ClientTestSuite) SetupTest() {
	var err error

	// create logger
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	// mock internal client
	suite.client, err = NewClient(suite.logger,
		false,
		&platformconfig.Config{
			ProjectsLeader: &platformconfig.ProjectsLeader{
				APIAddress: "somewhere.com",
			},
		},
		nil,
	)
	suite.Require().NoError(err)
}

func (suite *ClientTestSuite) TestCreate() {
	for _, testCase := range []struct {
		name                         string
		createProjectResponse        *http.Response
		getProjectCreationJobResults *http.Response
		errorExpected                string
	}{
		{
			name: "PositiveFlow",
			createProjectResponse: &http.Response{
				StatusCode: http.StatusCreated,
				Body: io.NopCloser(bytes.NewBufferString(`{
    "data": {
        "type": "project",
        "id": "e0d2a03d-884b-44e3-aa78-9c7cea0c0cf1",
        "attributes": {
            "name": "test-project",
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
}`)),
			},
			getProjectCreationJobResults: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewBufferString(`{
    "data": {
        "type": "job",
        "id": "4f4c834d-7cb5-4244-8ec4-8e21e88f4bc4",
        "attributes": {
            "kind": "project.creation",
            "state": "completed",
            "result": "",
            "created_at": "2021-08-23T18:55:35.363000+00:00",
            "updated_at": "2021-08-23T18:55:45.628000+00:00",
            "handler": "igz0.project.0"
        }
    },
    "included": [],
    "meta": {
        "ctx": "09337526008427605089"
    }
}`)),
			},
		},
		{
			name:          "FailedToRequestLeader",
			errorExpected: "Failed to send request to leader",
			createProjectResponse: &http.Response{
				StatusCode: http.StatusBadRequest,
				Body: io.NopCloser(bytes.NewBufferString(`{
    "errors": [
		{ "status": 400, "detail": "Failed to get user id for username" }
    ],
    "meta": {
        "ctx": "12391980595089803596"
    }
}`)),
			},
		},
		{
			name: "FailedToRequestJobStatus",
			createProjectResponse: &http.Response{
				StatusCode: http.StatusCreated,
				Body: io.NopCloser(bytes.NewBufferString(`{
    "data": {
        "type": "project",
        "id": "e0d2a03d-884b-44e3-aa78-9c7cea0c0cf1",
        "attributes": {
            "name": "test-project",
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
}`)),
			},
			getProjectCreationJobResults: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewBufferString(`{
    "data": {
        "type": "job",
        "id": "5e1db3b8-5870-4475-96c7-f858a3e1b198",
        "attributes": {
            "kind": "project.creation",
            "delay": 0.0,
            "state": "failed",
            "result": "{\"project_id\": \"e5d6c635-6a84-4cd8-b779-2d53884c8186\", \"status\": 400, \"message\": \"blablabla\"}",
            "created_at": "2021-08-23T18:56:31.346000+00:00",
            "updated_at": "2021-08-23T18:56:56.717000+00:00",
            "handler": "igz0.project.0"
        }
    },
    "included": [],
    "meta": {
        "ctx": "11002224568351879094"
    }
}`)),
			},
			errorExpected: "Failed waiting for create project job completion",
		},
		{
			name: "FailedCreateJobDidNotComplete",
			createProjectResponse: &http.Response{
				StatusCode: http.StatusCreated,
				Body: io.NopCloser(bytes.NewBufferString(`{
    "data": {
        "type": "project",
        "id": "e0d2a03d-884b-44e3-aa78-9c7cea0c0cf1",
        "attributes": {
            "name": "test-project",
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
}`)),
			},
			getProjectCreationJobResults: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewBufferString(`{
    "data": {
        "type": "job",
        "id": "5e1db3b8-5870-4475-96c7-f858a3e1b198",
        "attributes": {
			"kind":"project.creation",
			"state":"failed",
			"result":"{\"project_id\": \"72b28f22-f212-4001-b344-168ff3493989\", \"status\": null, \"message\": \"Failed to execute command by the given deadline. Last Exception: Job in progress. State: in_progress\"}"},
			"jobID":"a726f5d0-4d92-476e-afd7-51be8ee629ab"
    },
    "included": [],
    "meta": {
        "ctx": "11002224568351879094"
    }
}`)),
			},
			errorExpected: "Failed waiting for create project job completion",
		},
	} {
		suite.Run(testCase.name, func() {
			*suite.client.httpClient = *testutils.CreateDummyHTTPClient(func(r *http.Request) *http.Response {

				// post to create the project
				if r.Method == http.MethodPost && strings.HasSuffix(r.URL.String(), projectSuffix) {
					return testCase.createProjectResponse
				}

				if r.Method == http.MethodGet && strings.HasSuffix(r.URL.String(), getCreateProjectSuffix) {
					return testCase.getProjectCreationJobResults
				}

				panic(fmt.Sprintf("Unexpected request %s", r.RequestURI))
			})
			suite.client.leader = suite.generateMocksForClient(createTestSuite, testCase.errorExpected != "", 0)

			err := suite.client.Create(context.TODO(),
				&platform.CreateProjectOptions{
					ProjectConfig: &platform.ProjectConfig{
						Meta: platform.ProjectMeta{
							Name:      "test-project",
							Namespace: "test-namespace",
						},
					},
					WaitForCreateCompletion: true,
				})
			if testCase.errorExpected != "" {
				suite.Require().Error(err)
				suite.Require().Equal(err.Error(), testCase.errorExpected)
			} else {
				suite.Require().NoError(err)
			}
		})
	}
}

func (suite *ClientTestSuite) TestGetUpdatedAfter() {
	zeroUpdatedAfterTime := time.Time{}
	nowUpdatedAfterTime := time.Now()
	for _, testCase := range []struct {
		name             string
		updatedAfterTime *time.Time
		response         func(*http.Request) *http.Response
	}{
		{
			name:             "Sanity",
			updatedAfterTime: &nowUpdatedAfterTime,
			response: func(r *http.Request) *http.Response {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       suite.mockIgzAPIGetProject(false),
				}
			},
		},
		{
			name:             "RetryOnError",
			updatedAfterTime: &zeroUpdatedAfterTime,
			response: func(r *http.Request) *http.Response {
				if strings.Contains(r.URL.RawQuery, "0001-01-01T00:00:00Z") {
					suite.FailNow("updated_after should not be zero")
				} else if strings.Contains(r.URL.RawQuery, "1970-01-01T00:00:00Z") {
					return &http.Response{
						StatusCode: http.StatusInternalServerError,
						Body:       io.NopCloser(bytes.NewBufferString("")),
					}
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       suite.mockIgzAPIGetProject(false),
				}
			},
		},
	} {
		suite.Run(testCase.name, func() {
			*suite.client.httpClient = *testutils.CreateDummyHTTPClient(func(r *http.Request) *http.Response {
				suite.Require().LessOrEqual(strings.Count(r.URL.RawQuery, "updated_at"), 1)
				return testCase.response(r)
			})
			suite.client.leader = suite.generateMocksForClient(getUpdatedAfter, true, 0)
			projects, err := suite.client.GetUpdatedAfter(context.TODO(), testCase.updatedAfterTime)
			suite.Require().NoError(err)
			suite.Require().Len(projects, 1)
			suite.Require().Equal(projects[0].GetConfig().Spec.Owner, "admin")
		})
	}
}

func (suite *ClientTestSuite) TestGet() {
	for _, testCase := range []struct {
		name   string
		detail bool
	}{
		{
			name:   "Detail",
			detail: true,
		},
		{
			name:   "List",
			detail: false,
		},
	} {
		suite.Run(testCase.name, func() {
			*suite.client.httpClient = *testutils.CreateDummyHTTPClient(func(r *http.Request) *http.Response {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       suite.mockIgzAPIGetProject(testCase.detail),
				}
			})
			suite.client.leader = suite.generateMocksForClient(getTestSuite, true, 0)

			getProjectOptions := &platform.GetProjectsOptions{}
			if testCase.detail {
				getProjectOptions.Meta = platform.ProjectMeta{
					Name: "some-project",
				}
			}
			projects, err := suite.client.Get(context.TODO(), getProjectOptions)
			suite.Require().NoError(err)
			suite.Require().Len(projects, 1)
			suite.Require().Equal(projects[0].GetConfig().Spec.Owner, "admin")
		})
	}
}

func (suite *ClientTestSuite) TestUpdate() {
	for _, testCase := range []struct {
		name           string
		updateResponse *http.Response
		expectedError  string
	}{
		{
			name: "UpdateOkIGZResponse",
			updateResponse: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewBufferString(`{
					"data": {
						"type": "project",
						"id": "e0d2a03d-884b-44e3-aa78-9c7cea0c0cf1",
						"attributes": {
							"name": "test-project",
							"description": "an updated project",
							"created_at": "2021-08-23T19:39:50.522000+00:00",
							"updated_at": "2021-08-23T19:40:50.608000+00:00",
							"admin_status": "online",
							"operational_status": "online",
							"labels": [],
							"annotations": []
						}
					},
					"included": [],
					"meta": {
						"ctx": "13756324163199886387"
					}
				}`)),
			},
		},
		{
			name: "UpdateOkMLRunResponse",
			updateResponse: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewBufferString(`{
						"metadata": {
							"name": "test-project",
							"namespace" : "test-namespace",
							"created": "2021-08-23T19:39:50.522000+00:00"
							"labels": {},
							"annotations": {}
						},
						"spec": {
							"description": "an updated project",
						},
						"status": {
							"state": "completed"
						}
				}`)),
			},
		},
		{
			name: "UpdateFailed",
			updateResponse: &http.Response{
				StatusCode: http.StatusBadRequest,
				Body: io.NopCloser(bytes.NewBufferString(`{
					"errors": [
						{ "status": 400, "detail": "Failed to update project" }
					],
					"meta": {
						"ctx": "12391980595089803596"
					}
				}`)),
			},
			expectedError: "Failed to send update project request to leader",
		},
		{
			name:          "SendHTTPRequestError",
			expectedError: "Failed to send update project request to leader",
		},
	} {
		suite.Run(testCase.name, func() {
			*suite.client.httpClient = *testutils.CreateDummyHTTPClientWithError(func(r *http.Request) (*http.Response, error) {
				if testCase.expectedError != "" {
					return nil, errors.New(testCase.expectedError)
				}
				if r.Method == http.MethodPut && strings.HasSuffix(r.URL.String(), projectSuffix) {
					return testCase.updateResponse, nil
				}
				panic(fmt.Sprintf("Unexpected request %s", r.RequestURI))
			})
			suite.client.leader = suite.generateMocksForClient(updateTestSuite, true, 0)

			err := suite.client.Update(context.TODO(), &platform.UpdateProjectOptions{
				ProjectConfig: platform.ProjectConfig{
					Meta: platform.ProjectMeta{
						Name:      "test-project",
						Namespace: "test-namespace",
					},
				},
			})
			if testCase.expectedError != "" {
				suite.Require().Error(err)
				suite.Require().Equal(err.Error(), testCase.expectedError)
			} else {
				suite.Require().NoError(err)
			}
		})
	}
}

func (suite *ClientTestSuite) TestDelete() {
	for _, testCase := range []struct {
		name           string
		statusCode     int
		deleteResponse *http.Response
		expectedError  string
	}{
		{
			name:       "DeleteOkIGZResponse",
			statusCode: http.StatusAccepted,
			deleteResponse: &http.Response{
				StatusCode: http.StatusAccepted,
				Body: io.NopCloser(bytes.NewBufferString(`{
					"data": {
						"type": "project",
						"id": "e0d2a03d-884b-44e3-aa78-9c7cea0c0cf1",
						"attributes": {
							"name": "test-project",
							"description": "a deleted project",
							"created_at": "2021-08-23T19:39:50.522000+00:00",
							"updated_at": "2021-08-23T19:40:50.608000+00:00",
							"admin_status": "deleted",
							"operational_status": "deleted",
							"labels": [],
							"annotations": []
						}
					},
					"included": [],
					"meta": {
						"ctx": "13756324163199886387"
					}
				}`)),
			},
		},
		{
			name:       "DeleteOkMLRunResponse",
			statusCode: http.StatusNoContent,
			deleteResponse: &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
			},
		},
		{
			name:          "SendHTTPRequestError",
			expectedError: "Failed to send delete project request to leader",
		},
	} {
		suite.Run(testCase.name, func() {
			*suite.client.httpClient = *testutils.CreateDummyHTTPClientWithError(func(r *http.Request) (*http.Response, error) {
				if testCase.expectedError != "" {
					return nil, errors.New(testCase.expectedError)
				}
				if r.Method == http.MethodDelete && strings.HasSuffix(r.URL.String(), projectSuffix) {
					return testCase.deleteResponse, nil
				}
				panic(fmt.Sprintf("Unexpected request %s", r.RequestURI))
			})
			suite.client.leader = suite.generateMocksForClient(deleteTestSuite, true, testCase.statusCode)

			err := suite.client.Delete(context.TODO(), &platform.DeleteProjectOptions{
				Meta: platform.ProjectMeta{
					Name:      "test-project",
					Namespace: "test-namespace",
				},
			})
			if testCase.expectedError != "" {
				suite.Require().Error(err)
				suite.Require().Equal(err.Error(), testCase.expectedError)
			} else {
				suite.Require().NoError(err)
			}
		})
	}
}

func (suite *ClientTestSuite) mockIgzAPIGetProject(detail bool) io.ReadCloser {
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

func (suite *ClientTestSuite) generateMocksForClient(testSuiteType string, failureJobState bool, statusCode int) leader.ClientOps {
	newClient := mockClient.NewClient()
	testProject := &mockClient.MockProject{}
	testProject.On("GetConfig").Return(&platform.ProjectConfig{
		Meta: platform.ProjectMeta{
			Name: "test-project",
		},
		Spec: platform.ProjectSpec{
			Owner: "admin",
		},
	})

	switch testSuiteType {
	case createTestSuite:
		testJobResponse := mockClient.JobResponseMock{}
		if failureJobState {
			testJobResponse.On("GetState").Return(string(leader.JobStateFailed))
		} else {
			testJobResponse.On("GetState").Return(string(leader.JobStateCompleted))
		}
		newClient.On("IsJobTerminated", mock.Anything, mock.Anything).Return(&testJobResponse, true)
		newClient.On("GenerateProjectRequestBody", mock.Anything).Return([]byte(`{"some":"data"}`), nil)
		newClient.On("GenerateCreateProjectRequestURL", mock.Anything).Return("test-url" + projectSuffix)
		newClient.On("ResolveCreateProjectResponse", mock.Anything, mock.Anything).Return(mockClient.CreateProjectResponseMock{}, nil)
		newClient.On("ShouldWaitForCreateCompletion").Return(true)
		newClient.On("GetJobIdUrl", mock.Anything, mock.Anything).Return("test-url" + getCreateProjectSuffix)
		newClient.On("ValidateJobState", mock.Anything, mock.Anything, mock.Anything).
			Return(func(_ context.Context, jobResponse leader.JobResponse, _ string) error {
				if jobResponse.GetState() != leader.JobStateCompleted {
					return fmt.Errorf("job failed: expected state %s", jobResponse.GetState())
				}
				return nil
			})
		newClient.On("HandleCreateResponseErr", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("Failed to send request to leader"))
	case getTestSuite:
		newClient.On("GenerateGetProjectsRequestURL", mock.Anything, mock.Anything).Return("some-url")
		newClient.On("ResolveGetProjectResponse", mock.Anything, mock.Anything).Return([]platform.Project{testProject}, nil)
	case getUpdatedAfter:
		newClient.On("GenerateGetProjectsRequestURL", mock.Anything, mock.Anything).Return("some-url")
		newClient.On("ResolveGetProjectResponse", mock.Anything, mock.Anything).Return([]platform.Project{testProject}, nil)
		newClient.On("GenerateGetUpdatedAfterRequestURL", mock.Anything).Return("some-url")
	case updateTestSuite:
		newClient.On("GenerateProjectRequestBody", mock.Anything).Return([]byte(`{"some":"data"}`), nil)
		newClient.On("GenerateUpdateProjectRequestURL", mock.Anything, mock.Anything).Return("test-url" + projectSuffix)
		newClient.On("HandleCreateResponseErr", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
			func(_ context.Context, _ []byte, resp *http.Response, _ error) error {
				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("update failed")
				}
				return nil
			},
		)
	case deleteTestSuite:
		newClient.On("GenerateDeleteProjectRequestURL", mock.Anything, mock.Anything).Return("test-url" + projectSuffix)
		newClient.On("GenerateProjectDeletionRequestBody", mock.Anything).Return([]byte(`{"some":"data"}`), nil)
		newClient.On("AddDeleteStrategyHeader", mock.Anything, mock.Anything).Return()
		newClient.On("GetDeleteExpectedStatusCode").Return(statusCode)
	}

	return newClient
}

func TestClientTestSuite(t *testing.T) {
	suite.Run(t, new(ClientTestSuite))
}
