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
	"net/http"
	"path"
	"testing"

	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	httpsuite.TestSuite
}

func (suite *TestSuite) SetupTest() {
	suite.TestSuite.SetupTest()

	suite.Runtime = "dotnetcore"
	suite.FunctionDir = path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "runtime", "dotnetcore", "test")
}

func (suite *TestSuite) TestOutputs() {
	statusOK := http.StatusOK
	statusCreated := http.StatusCreated
	statusInternalError := http.StatusInternalServerError
	logLevelDebug := "debug"
	logLevelWarn := "warn"
	testPath := "/path/to/nowhere"
	longTestBody := `long body: Lorem ipsum dolor sit amet, consectetuer adipiscing elit. Aenean commodo ligula eget dolor. Aenean massa. 
	Cum sociis natoque penatibus et magnis dis parturient montes, nascetur ridiculous mus. Donec quam felis, ultricies nec, 
	pellentesque eu, pretium quis, sem. Nulla consequat massa quis enim. Donec pede justo, fringilla vel, aliquet nec, 
	vulputate eget, arcu. In enim justo, rhoncus ut, imperdiet a, venenatis vitae, justo. Nullam dictum felis eu pede mollis pretium.`

	headersContentTypeTextPlain := map[string]string{"content-type": "text/plain"}

	// headersContentTypeApplicationJSON := map[string]string{"content-type": "application/json"}
	deployOptions := suite.GetDeployOptions("outputter",
		suite.GetFunctionPath("_outputter"))

	deployOptions.FunctionConfig.Spec.Handler = "nuclio:outputter"
	suite.DeployFunctionAndRequests(deployOptions, []*httpsuite.Request{
		{
			Name:                       "string",
			RequestBody:                "return_string",
			ExpectedResponseHeaders:    headersContentTypeTextPlain,
			ExpectedResponseBody:       "a string",
			ExpectedResponseStatusCode: &statusOK,
		},
		{
			Name:                       "json_convert",
			RequestBody:                "json_convert",
			ExpectedResponseHeaders:    headersContentTypeTextPlain,
			ExpectedResponseBody:       "{\n  \"Name\": \"John Doe\",\n  \"Email\": \"john@iguazio.com\"\n}",
			ExpectedResponseStatusCode: &statusOK,
		},
		{
			Name:                       "long body",
			RequestBody:                longTestBody,
			ExpectedResponseHeaders:    headersContentTypeTextPlain,
			ExpectedResponseBody:       longTestBody,
			ExpectedResponseStatusCode: &statusOK,
		},
		{
			Name:                       "panic",
			RequestBody:                "panic",
			ExpectedResponseStatusCode: &statusInternalError,
		},
		{
			Name:           "response object",
			RequestHeaders: map[string]interface{}{"a": "1", "b": "2"},
			RequestBody:    "return_response",
			ExpectedResponseHeaders: map[string]string{
				"a":            "1",
				"b":            "2",
				"h1":           "v1",
				"h2":           "v2",
				"Content-Type": "text/plain; charset=utf-8",
			},
			ExpectedResponseBody:       "response body",
			ExpectedResponseStatusCode: &statusCreated,
		},
		{
			Name:                       "logs - debug",
			RequestBody:                "log",
			RequestLogLevel:            &logLevelDebug,
			ExpectedResponseHeaders:    headersContentTypeTextPlain,
			ExpectedResponseBody:       "returned logs",
			ExpectedResponseStatusCode: &statusOK,
			ExpectedLogMessages: []string{
				"Debug message",
				"Info message",
				"Warn message",
				"Error message",
			},
		},
		{
			Name:                       "logs - warn",
			RequestBody:                "log",
			RequestLogLevel:            &logLevelWarn,
			ExpectedResponseHeaders:    headersContentTypeTextPlain,
			ExpectedResponseBody:       "returned logs",
			ExpectedResponseStatusCode: &statusOK,
			ExpectedLogMessages: []string{
				"Warn message",
				"Error message",
			},
		},
		{
			Name:                 "GET",
			RequestMethod:        "GET",
			ExpectedResponseBody: "GET",
		},
		{
			Name:                       "fields",
			RequestPath:                "/?x=1&y=2",
			RequestBody:                "return_fields",
			RequestLogLevel:            &logLevelWarn,
			ExpectedResponseHeaders:    headersContentTypeTextPlain,
			ExpectedResponseBody:       "x=1,y=2",
			ExpectedResponseStatusCode: &statusOK,
		},
		{
			Name:                 "path",
			RequestBody:          "return_path",
			RequestPath:          testPath,
			ExpectedResponseBody: testPath,
		},
	})
}

func (suite *TestSuite) TestStress() {
	// Create blastConfiguration using default configurations + changes for dotnet specification
	blastConfiguration := suite.NewBlastConfiguration()
	blastConfiguration.FunctionPath = "_outputter"
	blastConfiguration.Handler = "nuclio:outputter"

	// Create stress test using suite.BlastHTTP
	suite.BlastHTTP(blastConfiguration)
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestSuite))
}
