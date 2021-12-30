//go:build test_unit

package iguazio

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/nuclio/nuclio/pkg/common/testutils"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
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
	suite.client, err = NewClient(suite.logger, &platformconfig.Config{
		ProjectsLeader: &platformconfig.ProjectsLeader{
			APIAddress: "somewhere.com",
		},
	})
	suite.Require().NoError(err)
}

func (suite *ClientTestSuite) TestCreate() {

	for _, testCase := range []struct {
		name                         string
		createProjectResponse        *http.Response
		getProjectCreationJobResults *http.Response
		expectedFailure              bool
	}{
		{
			name: "create-ok-job-success",
			createProjectResponse: &http.Response{
				StatusCode: http.StatusCreated,
				Body: ioutil.NopCloser(bytes.NewBufferString(`{
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
}`)),
			},
			getProjectCreationJobResults: &http.Response{
				StatusCode: http.StatusOK,
				Body: ioutil.NopCloser(bytes.NewBufferString(`{
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
			name:            "create-failed",
			expectedFailure: true,
			createProjectResponse: &http.Response{
				StatusCode: http.StatusBadRequest,
				Body: ioutil.NopCloser(bytes.NewBufferString(`{
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
			name: "create-ok-job-failed",
			createProjectResponse: &http.Response{
				StatusCode: http.StatusCreated,
				Body: ioutil.NopCloser(bytes.NewBufferString(`{
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
}`)),
			},
			getProjectCreationJobResults: &http.Response{
				StatusCode: http.StatusOK,
				Body: ioutil.NopCloser(bytes.NewBufferString(`{
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
			expectedFailure: true,
		},
	} {
		suite.Run(testCase.name, func() {
			suite.client.httpClient = testutils.CreateDummyHTTPClient(func(r *http.Request) *http.Response {

				// post to create the project
				if r.Method == http.MethodPost && strings.HasSuffix(r.URL.String(), "/projects") {
					return testCase.createProjectResponse
				}

				if r.Method == http.MethodGet && strings.HasSuffix(r.URL.String(), "/jobs/some-job-id") {
					return testCase.getProjectCreationJobResults
				}

				panic(fmt.Sprintf("Unexpected request %s", r.RequestURI))
			})

			err := suite.client.Create(&platform.CreateProjectOptions{
				ProjectConfig: &platform.ProjectConfig{
					Meta: platform.ProjectMeta{
						Name: "dummy-project",
					},
				},
				WaitForCreateCompletion: true,
			})
			if testCase.expectedFailure {
				suite.Require().Error(err)
				return
			}
			suite.Require().NoError(err)
		})

	}
}

func (suite *ClientTestSuite) TestGet() {
	for _, testCase := range []struct {
		name   string
		detail bool
	}{
		{
			name:   "detail",
			detail: true,
		},
		{
			name:   "list",
			detail: false,
		},
	} {
		suite.Run(testCase.name, func() {
			suite.client.httpClient = testutils.CreateDummyHTTPClient(func(r *http.Request) *http.Response {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       suite.mockIgzAPIGetProject(testCase.detail),
				}
			})

			getProjectOptions := &platform.GetProjectsOptions{}
			if testCase.detail {
				getProjectOptions.Meta = platform.ProjectMeta{
					Name: "some-project",
				}
			}
			projects, err := suite.client.Get(getProjectOptions)
			suite.Require().NoError(err)
			suite.Require().Len(projects, 1)
			suite.Require().Equal(projects[0].GetConfig().Spec.Owner, "admin")

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
            "updated_at": "2021-08-12T07:13:29.845000+00:00"
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
		return ioutil.NopCloser(bytes.NewBufferString(fmt.Sprintf(responseTemplate, projectData)))
	}

	return ioutil.NopCloser(bytes.NewBufferString(fmt.Sprintf(responseTemplate, "["+projectData+"]")))
}

func TestClientTestSuite(t *testing.T) {
	suite.Run(t, new(ClientTestSuite))
}
