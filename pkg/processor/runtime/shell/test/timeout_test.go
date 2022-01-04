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
	"fmt"
	"net/http"
	"path"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

	"github.com/stretchr/testify/suite"
)

// TimeoutTestSuite is a shell timeout test suite
type TimeoutTestSuite struct {
	httpsuite.TestSuite
}

// SetupTest sets up the test
func (suite *TimeoutTestSuite) SetupTest() {
	suite.TestSuite.SetupTest()

	suite.Runtime = "shell"
	suite.FunctionDir = path.Join(suite.GetNuclioSourceDir(), "test", "_functions", "shell")
}

func (suite *TimeoutTestSuite) TestTimeout() {
	eventTimeout := 1 * time.Second
	createFunctionOptions := suite.GetDeployOptions("timeout",
		suite.GetFunctionPath("timeout"))

	createFunctionOptions.FunctionConfig.Spec.EventTimeout = eventTimeout.String()
	createFunctionOptions.FunctionConfig.Spec.Handler = "timeout.sh:main"
	okStatusCode := http.StatusOK
	timeoutStatusCode := http.StatusRequestTimeout
	suite.DeployFunctionAndRequests(createFunctionOptions, []*httpsuite.Request{
		{
			Name:                       "handler timeout",
			RequestBody:                "",
			ExpectedResponseBody:       "Failed waiting for function execution",
			ExpectedResponseStatusCode: &timeoutStatusCode,
		},
		{
			Name:                       "do not timeout",
			RequestBody:                fmt.Sprintf("sleep %f", (eventTimeout / 2).Seconds()),
			ExpectedResponseStatusCode: &okStatusCode,
		},
	})
}

func TestTimeout(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TimeoutTestSuite))
}
