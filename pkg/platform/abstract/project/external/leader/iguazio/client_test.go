// +build test_unit

package iguazio

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
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
