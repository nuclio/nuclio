package test

import (
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"
	"path"
	"net/http"
	"github.com/nuclio/nuclio/pkg/platform"
	"regexp"
	"testing"
	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	httpsuite.TestSuite
}

func (suite *TestSuite) SetupTest() {
	suite.TestSuite.SetupTest()

	suite.Runtime = "ruby"
	suite.FunctionDir = path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "runtime", "ruby", "test")
}

func (suite *TestSuite) TestOutputs() {
	statusOK := http.StatusOK
	headersContentTypeTextPlain := map[string]string{"content-type": "text/plain"}
	statusCreated := http.StatusCreated
	headersFromResponse := map[string]string{
		"h1":           "v1",
		"h2":           "v2",
		"content-type": "text/plain",
	}
	statusInternalError := http.StatusInternalServerError
	logLevelDebug := "debug"
	logLevelWarn := "warn"
	testPath := "/path/to/nowhere"

	createFunctionOptions := suite.GetDeployOptions("outputter",
		suite.GetFunctionPath("outputter"))
	createFunctionOptions.FunctionConfig.Spec.Handler = "outputter#main"

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		testRequests := []httpsuite.Request{
			{
				Name:                       "return string",
				RequestBody:                "return_string",
				ExpectedResponseHeaders:    headersContentTypeTextPlain,
				ExpectedResponseBody:       "a string",
				ExpectedResponseStatusCode: &statusOK,
			},
			{
				Name:                       "bytes",
				RequestBody:                "return_bytes",
				ExpectedResponseHeaders:    headersContentTypeTextPlain,
				ExpectedResponseBody:       "bytes",
				ExpectedResponseStatusCode: &statusOK,
			},
			{
				Name:                       "return response",
				RequestHeaders:             map[string]interface{}{"a": "1", "b": "2"},
				RequestBody:                "return_response",
				ExpectedResponseHeaders:    headersFromResponse,
				ExpectedResponseBody:       "response body",
				ExpectedResponseStatusCode: &statusCreated,
			},
			{
				// function raises an exception. we want to make sure it
				// continues functioning afterwards
				Name:                       "raise exception",
				RequestBody:                "something invalid",
				ExpectedResponseHeaders:    headersContentTypeTextPlain,
				ExpectedResponseStatusCode: &statusInternalError,
			},
			{
				Name:                       "logs - debug",
				RequestBody:                "log",
				RequestLogLevel:            &logLevelDebug,
				ExpectedResponseHeaders:    headersContentTypeTextPlain,
				ExpectedResponseBody:       "returned logs",
				ExpectedResponseStatusCode: &statusCreated,
				ExpectedLogMessages: []string{
					"Debug message",
					"Info message",
					"Warn message",
					"Error message",
					"Response is",	// request debug log
				},
			},
			{
				Name:                       "logs - warn",
				RequestBody:                "log",
				RequestLogLevel:            &logLevelWarn,
				ExpectedResponseHeaders:    headersContentTypeTextPlain,
				ExpectedResponseBody:       "returned logs",
				ExpectedResponseStatusCode: &statusCreated,
				ExpectedLogMessages: []string{
					"Warn message",
					"Error message",
				},
			},
			{
				Name:                       "logs - with",
				RequestBody:                "log_with",
				RequestLogLevel:            &logLevelWarn,
				ExpectedResponseHeaders:    headersContentTypeTextPlain,
				ExpectedResponseBody:       "returned logs with",
				ExpectedResponseStatusCode: &statusCreated,
				ExpectedLogRecords: []map[string]interface{}{
					{
						"level":   "error",
						"message": "Error message",
						// extra with
						"source": "rabbit",
						"weight": 7.0, // encoding/json return float64 for all numbers
					},
				},
			},
			{
				Name:                       "get",
				RequestMethod:              "GET",
				RequestBody:                "",
				RequestLogLevel:            &logLevelWarn,
				ExpectedResponseHeaders:    headersContentTypeTextPlain,
				ExpectedResponseBody:       "GET",
				ExpectedResponseStatusCode: &statusOK,
			},
			{
				Name:                       "fields",
				RequestMethod:              "POST",
				RequestPath:                "/?x=1&y=2",
				RequestBody:                "return_fields",
				RequestLogLevel:            &logLevelWarn,
				ExpectedResponseHeaders:    headersContentTypeTextPlain,
				ExpectedResponseBody:       "x=1,y=2",
				ExpectedResponseStatusCode: &statusOK,
			},
			{
				Name:                       "path",
				RequestMethod:              "POST",
				RequestPath:                testPath,
				RequestBody:                "return_path",
				RequestLogLevel:            &logLevelWarn,
				ExpectedResponseHeaders:    headersContentTypeTextPlain,
				ExpectedResponseBody:       testPath,
				ExpectedResponseStatusCode: &statusOK,
			},
			{
				Name:                       "error",
				RequestBody:                "return_error",
				RequestLogLevel:            &logLevelWarn,
				ExpectedResponseHeaders:    headersContentTypeTextPlain,
				ExpectedResponseStatusCode: &statusInternalError,
				ExpectedResponseBody:       regexp.MustCompile("some error"),
			},
		}

		for _, testRequest := range testRequests {
			suite.Logger.DebugWith("Running sub test", "name", testRequest.Name)

			// set defaults
			if testRequest.RequestPort == 0 {
				testRequest.RequestPort = deployResult.Port
			}

			if testRequest.RequestMethod == "" {
				testRequest.RequestMethod = "POST"
			}

			if testRequest.RequestPath == "" {
				testRequest.RequestPath = "/"
			}

			if !suite.SendRequestVerifyResponse(&testRequest) {
				return false
			}
		}

		return true
	})
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestSuite))
}