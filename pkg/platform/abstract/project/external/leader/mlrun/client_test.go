package mlrun

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/nuclio/nuclio/pkg/auth/nop"
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

func (suite *ClientTestSuite) SetupSuite() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("test-mlrun-client")
	suite.Require().NoError(err)
}

func (suite *ClientTestSuite) SetupTest() {
	var err error
	suite.client, err = NewClient(suite.logger, &platformconfig.Config{
		ProjectsLeader: &platformconfig.ProjectsLeader{
			APIAddress: "mlrun.com",
			Kind:       platformconfig.ProjectsLeaderKindMlrun,
		},
	})
	suite.Require().NoError(err)
}

func (suite *ClientTestSuite) TestCreate() {
	tests := []struct {
		name               string
		httpResponse       *http.Response
		httpError          error
		expectedErrMsg     string
		includeAuthSession bool
	}{
		{
			name: "success",
			httpResponse: &http.Response{
				StatusCode: http.StatusCreated,
				Body: io.NopCloser(bytes.NewBufferString(`{
					"metadata": {"name": "testCase-project", "namespace": "testCase-namespace"},
					"spec": {"description": "desc"}
				}`)),
			},
		},
		{
			name: "positive case - with auth session",
			httpResponse: &http.Response{
				StatusCode: http.StatusCreated,
				Body: io.NopCloser(bytes.NewBufferString(`{
					"metadata": {"name": "testCase-project", "namespace": "testCase-namespace"},
					"spec": {"description": "desc"}
				}`)),
			},
			includeAuthSession: true,
		},
		{
			name:           "http client error",
			httpError:      errors.New("http error"),
			expectedErrMsg: "Failed to send request to leader",
		},
		{
			name: "bad response body",
			httpResponse: &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(bytes.NewBufferString(`{invalid json`)),
			},
			expectedErrMsg: "Failed to resolve project from response body",
		},
	}

	for _, testCase := range tests {
		suite.Run(testCase.name, func() {
			*suite.client.httpClient.GetHTTPClient() = *testutils.CreateDummyHTTPClientWithError(func(r *http.Request) (*http.Response, error) {
				if testCase.httpError != nil {
					return nil, testCase.httpError
				}
				return testCase.httpResponse, nil
			})

			createProjectOptions := &platform.CreateProjectOptions{
				ProjectConfig: &platform.ProjectConfig{
					Meta: platform.ProjectMeta{
						Name:      "mlrun-create-project",
						Namespace: "test-create-namespace",
					},
				},
			}

			if testCase.includeAuthSession {
				createProjectOptions.AuthSession = &nop.Session{}
			}

			err := suite.client.Create(context.TODO(), createProjectOptions)

			if testCase.expectedErrMsg != "" {
				suite.Require().Error(err)
				suite.Contains(err.Error(), testCase.expectedErrMsg)
			} else {
				suite.Require().NoError(err)
			}
		})
	}
}

func (suite *ClientTestSuite) TestUpdate() {
	tests := []struct {
		name           string
		httpResponse   *http.Response
		httpError      error
		expectedErrMsg string
	}{
		{
			name: "success",
			httpResponse: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
			},
		},
		{
			name:           "http client error",
			httpError:      errors.New("http error"),
			expectedErrMsg: "Failed to send update project request to leader",
		},
	}

	for _, testCase := range tests {
		suite.Run(testCase.name, func() {
			*suite.client.httpClient.GetHTTPClient() = *testutils.CreateDummyHTTPClientWithError(func(r *http.Request) (*http.Response, error) {
				if testCase.httpError != nil {
					return nil, testCase.httpError
				}
				return testCase.httpResponse, nil
			})

			err := suite.client.Update(context.TODO(), &platform.UpdateProjectOptions{
				ProjectConfig: platform.ProjectConfig{
					Meta: platform.ProjectMeta{
						Name:      "mlrun-update-project",
						Namespace: "test-update-namespace",
					},
				},
			})

			if testCase.expectedErrMsg != "" {
				suite.Require().Error(err)
				suite.Contains(err.Error(), testCase.expectedErrMsg)
			} else {
				suite.Require().NoError(err)
			}
		})
	}
}

func (suite *ClientTestSuite) TestDelete() {
	tests := []struct {
		name           string
		httpResponse   *http.Response
		httpError      error
		expectedErrMsg string
	}{
		{
			name: "positive case",
			httpResponse: &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
			},
		},
		{
			name:           "http client error",
			httpError:      errors.New("http error"),
			expectedErrMsg: "Failed to send delete project request to leader",
		},
	}

	for _, testCase := range tests {
		suite.Run(testCase.name, func() {
			*suite.client.httpClient.GetHTTPClient() = *testutils.CreateDummyHTTPClientWithError(func(r *http.Request) (*http.Response, error) {
				if testCase.httpError != nil {
					return nil, testCase.httpError
				}
				return testCase.httpResponse, nil
			})

			err := suite.client.Delete(context.TODO(), &platform.DeleteProjectOptions{
				Meta: platform.ProjectMeta{
					Name:      "mlrun-delete-project",
					Namespace: "test-delete-namespace",
				},
			})

			if testCase.expectedErrMsg != "" {
				suite.Require().Error(err)
				suite.Equal(err.Error(), testCase.expectedErrMsg)
			} else {
				suite.Require().NoError(err)
			}
		})
	}
}

func (suite *ClientTestSuite) TestGenerateProjectRequestBody() {
	tests := []struct {
		name         string
		project      *platform.ProjectConfig
		expectErr      bool
		expectedResult string
	}{
		{
			name: "positive case",
			project: &platform.ProjectConfig{
				Meta: platform.ProjectMeta{
					Name:      "test-project",
					Namespace: "test-namespace",
				},
				Spec: platform.ProjectSpec{
					Description: "desc",
				},
			},
			expectedResult: `{"metadata":{"name":"test-project","namespace":"test-namespace"},"spec":{"description":"desc"},"status":{}}`,
		},
		{
			name:           "nil project config",
			project:        nil,
			expectErr:      true,
			expectedResult: "Failed to create project from project config",
		},
		{
			name: "empty project config",
			project: &platform.ProjectConfig{
				Meta: platform.ProjectMeta{},
				Spec: platform.ProjectSpec{},
			},
			expectedResult: `{"metadata":{"name":"","namespace":""},"spec":{},"status":{}}`,
		},
	}

	for _, testCase := range tests {
		suite.Run(testCase.name, func() {
			result, err := suite.client.generateProjectRequestBody(testCase.project)
			if testCase.expectErr {
				suite.Require().Error(err)
				suite.Equal(err.Error(), testCase.expectedResult)
			} else {
				suite.Require().NoError(err)
				suite.Equal(testCase.expectedResult, string(result))
			}
		})
	}
}

func (suite *ClientTestSuite) TestGenerateProjectDeletionRequestBody() {
	tests := []struct {
		name         string
		projectName  string
		expectString string
	}{
		{
			name:         "positive case",
			projectName:  "delete-test-project",
			expectString: `{"metadata":{"name":"delete-test-project","namespace":""},"spec":{},"status":{}}`,
		},
		{
			name:         "empty name",
			projectName:  "",
			expectString: `{"metadata":{"name":"","namespace":""},"spec":{},"status":{}}`,
		},
		{
			name:         "long name",
			projectName:  "a-very-long-project-name-1234567890",
			expectString: `{"metadata":{"name":"a-very-long-project-name-1234567890","namespace":""},"spec":{},"status":{}}`,
		},
	}

	for _, testCase := range tests {
		suite.Run(testCase.name, func() {
			result, err := suite.client.generateProjectDeletionRequestBody(testCase.projectName)
			suite.Require().NoError(err)
			suite.Equal(string(result), testCase.expectString)
		})
	}
}

func (suite *ClientTestSuite) TestResolveCreateProjectResponse() {
	tests := []struct {
		name           string
		body           []byte
		expectedErrMsg string
		expectedResult *Project
		description    string
	}{
		{
			name: "valid response",
			body: []byte(`{"metadata":{"name":"test-project","namespace":"test-namespace"},"spec":{"description":"desc"}}`),
			expectedResult: &Project{
				Metadata: ProjectMetadata{
					Name:      "test-project",
					Namespace: "test-namespace",
				},
				Spec: ProjectSpec{
					Description: "desc",
				},
			},
			description: "desc",
		},
		{
			name:           "invalid json",
			body:           []byte("{invalid json"),
			expectedErrMsg: "Failed to unmarshal response body",
		},
		{
			name:           "empty body",
			body:           []byte(""),
			expectedErrMsg: "Failed to unmarshal response body",
		},
	}

	for _, testCase := range tests {
		suite.Run(testCase.name, func() {
			result, err := suite.client.resolveCreateProjectResponse(testCase.body)
			if testCase.expectedErrMsg != "" {
				suite.Require().Error(err)
				suite.Equal(err.Error(), testCase.expectedErrMsg)
			} else {
				suite.Require().NoError(err)
				suite.Equal(result, testCase.expectedResult)
			}
		})
	}
}

func TestClientTestSuite(t *testing.T) {
	suite.Run(t, new(ClientTestSuite))
}
