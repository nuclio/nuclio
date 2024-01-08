package nexus

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

type HelperSuite struct {
	suite.Suite
	*http.Client
	testServer  *httptest.Server
	nexusRouter *NexusRouter
}

func (helperSuite *HelperSuite) SetupTest() {
	nexus := Initialize()

	helperSuite.nexusRouter = NewNexusRouter(nexus)
	helperSuite.nexusRouter.Initialize()
	helperSuite.Client = &http.Client{}

	helperSuite.testServer = httptest.NewServer(helperSuite.nexusRouter.Router)
}

func (helperSuite *HelperSuite) TearDownTest() {
	helperSuite.testServer.Close()
}

func (helperSuite *HelperSuite) TestInitialize() {
	testCases := []struct {
		name       string
		method     string
		path       string
		statusCode int
	}{
		{"StartScheduler", http.MethodPost, "/scheduler/deadline/start", http.StatusOK},
		{"StopScheduler", http.MethodPost, "/scheduler/deadline/stop", http.StatusOK},
		{"GetAllSchedulersWithStatus", http.MethodGet, "/scheduler", http.StatusOK},
		{"ModifyNexusConfig", http.MethodPut, "/config", http.StatusOK},
	}

	for _, tc := range testCases {
		helperSuite.Run(tc.name, func() {
			req, err := http.NewRequest(tc.method, helperSuite.testServer.URL+tc.path, nil)
			assert.NoError(helperSuite.T(), err)

			resp, respErr := helperSuite.Client.Do(req)
			assert.NoError(helperSuite.T(), respErr)

			assert.Equal(helperSuite.T(), tc.statusCode, resp.StatusCode)
		})
	}

}

func (helperSuite *HelperSuite) TestModifyNexusConfig() {
	queryParams := url.Values{}
	queryParams.Add("maxParallelRequests", "10")

	pathWithQuery := fmt.Sprintf("/config?%s", queryParams.Encode())
	req, err := http.NewRequest(http.MethodPut, helperSuite.testServer.URL+pathWithQuery, nil)
	assert.NoError(helperSuite.T(), err)

	resp, respErr := helperSuite.Client.Do(req)
	assert.NoError(helperSuite.T(), respErr)

	assert.Equal(helperSuite.T(), http.StatusAccepted, resp.StatusCode)

	body, readErr := io.ReadAll(resp.Body)
	assert.NoError(helperSuite.T(), readErr)
	assert.Equal(helperSuite.T(), "Max parallel requests set to 10", string(body))
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(HelperSuite))
}
