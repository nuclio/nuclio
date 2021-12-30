//go:build test_integration && test_local

/*
Copyright 2017 The Nuclio Authors.
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

package test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"path"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

	"github.com/stretchr/testify/suite"
)

var (
	requestHeaders = map[string]interface{}{
		"Content-Type": "application/json",
	}
)

// TimeoutTestSuite is a golang timeout test suite
type TimeoutTestSuite struct {
	httpsuite.TestSuite
}

// SetupTest sets up the test
func (suite *TimeoutTestSuite) SetupTest() {
	suite.TestSuite.SetupTest()

	suite.Runtime = "golang"
	suite.FunctionDir = path.Join(suite.GetNuclioSourceDir(), "test", "_functions", "golang")
}

func (suite *TimeoutTestSuite) TestTimeout() {
	eventTimeout := 300 * time.Millisecond
	createFunctionOptions := suite.GetDeployOptions("timeout",
		suite.GetFunctionPath("timeout"))
	createFunctionOptions.FunctionConfig.Spec.EventTimeout = eventTimeout.String()
	okStatusCode := http.StatusOK
	timeoutStatusCode := http.StatusRequestTimeout

	suite.DeployFunctionAndRequests(createFunctionOptions, []*httpsuite.Request{
		{
			RequestBody:    suite.genTimeoutRequest(time.Millisecond),
			RequestHeaders: requestHeaders,

			ExpectedResponseStatusCode: &okStatusCode,
		},
		{
			RequestBody:    suite.genTimeoutRequest(time.Second),
			RequestHeaders: requestHeaders,

			ExpectedResponseStatusCode: &timeoutStatusCode,
		},
	})
}

func (suite *TimeoutTestSuite) createRequest(timeout time.Duration) io.Reader {
	var buf bytes.Buffer
	request := map[string]string{
		"timeout": timeout.String(),
	}

	err := json.NewEncoder(&buf).Encode(request)
	suite.Require().NoError(err, "Can't encode request")
	return &buf
}

func (suite *TimeoutTestSuite) genTimeoutRequest(timeout time.Duration) string {
	request := map[string]interface{}{
		"timeout": timeout.String(),
	}
	data, err := json.Marshal(request)
	suite.Require().NoErrorf(err, "Can't encode request - %#v", request)

	return string(data)
}

func TestTimeout(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TimeoutTestSuite))
}
