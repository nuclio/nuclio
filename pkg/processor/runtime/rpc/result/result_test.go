//go:build test_unit

/*
Copyright 2024 The Nuclio Authors.

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
package result

import (
	"encoding/base64"
	"net/http"
	"testing"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type ResultSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *ResultSuite) SuiteSetup() {
	suite.logger = suite.createLogger()
}

func (suite *ResultSuite) createLogger() logger.Logger {
	loggerInstance, err := nucliozap.NewNuclioZapTest("result-test")
	suite.Require().NoError(err, "Can't create logger")

	return loggerInstance
}
func (suite *ResultSuite) TestNewResultFromDataRawInputs() {
	streamBodyValue := "stream-body"
	bodyBase64 := base64.StdEncoding.EncodeToString([]byte(streamBodyValue))

	testCases := []struct {
		name           string
		rawData        []byte
		expectedResult any
		expectedError  bool
	}{
		{
			name:    "single result (text body)",
			rawData: []byte(`r{"body": "123", "content_type": "123", "headers": {}, "status_code": 200, "body_encoding": "text"}`),
			expectedResult: &SingleResult{
				Response: &nuclio.Response{
					StatusCode:  200,
					ContentType: "123",
					Headers:     map[string]interface{}{},
					Body:        []byte("123"),
				},
			},
		},
		{
			name:    "batch result (text body)",
			rawData: []byte(`r[{"body": "123", "content_type": "123", "headers": {}, "status_code": 200, "body_encoding": "text"}]`),
			expectedResult: &BatchedResults{
				Results: []*SingleResult{
					{
						Response: &nuclio.Response{
							StatusCode:  200,
							ContentType: "123",
							Headers:     map[string]interface{}{},
							Body:        []byte("123"),
						},
					},
				},
			},
		},
		{
			name:           "body only (base64)",
			rawData:        []byte("b" + bodyBase64),
			expectedResult: &BodyOnly{Body: []byte(streamBodyValue)},
		},
		{
			name:           "stream end",
			rawData:        []byte("e"),
			expectedResult: &StreamEnd{},
		},
		{
			name:    "stream start (text body)",
			rawData: []byte(`c{"body": "123", "content_type": "123", "headers": {}, "status_code": 200, "body_encoding": "text"}`),
			expectedResult: func() any {
				sr := &SingleResult{
					Response: &nuclio.Response{
						StatusCode:  200,
						ContentType: "123",
						Headers:     map[string]interface{}{},
						Body:        []byte("123"),
					},
				}
				return newStreamStartFromSingleResult(sr)
			}(),
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			result := NewResultFromData(testCase.rawData)
			suite.Require().NotNil(result, "expected result, got nil")

			if testCase.expectedError {
				suite.Error(result.Error())
			} else {
				suite.NoError(result.Error())
			}

			switch expected := testCase.expectedResult.(type) {
			case *SingleResult:
				actual, ok := result.(*SingleResult)
				suite.True(ok)
				suite.Equal(expected.StatusCode, actual.StatusCode)
				suite.Equal(expected.ContentType, actual.ContentType)
				suite.Equal(expected.Body, actual.Body)
			case *BatchedResults:
				actual, ok := result.(*BatchedResults)
				suite.True(ok)
				suite.Len(actual.Results, len(expected.Results))
				for i := range actual.Results {
					suite.Equal(expected.Results[i].Body, actual.Results[i].Body)
					suite.Equal(expected.Results[i].StatusCode, actual.Results[i].StatusCode)
				}
			case *BodyOnly:
				actual, ok := result.(*BodyOnly)
				suite.True(ok)
				suite.Equal(expected, actual)
			case *StreamEnd:
				_, ok := result.(*StreamEnd)
				suite.True(ok)
			case *StreamStart:
				actual, ok := result.(*StreamStart)
				suite.True(ok)
				suite.Equal(expected.GetStatusCode(), actual.GetStatusCode())
				suite.Equal(expected.GetContentType(), actual.GetContentType())
			default:
				suite.Fail("unhandled result type")
			}
		})
	}
}

func (suite *ResultSuite) TestSetStatusCodeFromError() {
	responseStream := nuclio.NewResponseStream("", nil, 200)
	streamStart := NewStreamStart(responseStream)
	defer responseStream.StopStreaming()

	streamStart.SetStatusCode(200)
	err := nuclio.NewErrRequestTimeout("Request timed out")
	firstWrapping := nuclio.WrapErrProcessing(err)
	secondWrapping := errors.Wrap(firstWrapping, "Failed to process request")

	streamStart.SetStatusCodeFromError(secondWrapping)
	suite.Require().Equal(http.StatusProcessing, streamStart.GetStatusCode())

}

func TestResultSuite(t *testing.T) {
	suite.Run(t, new(ResultSuite))
}
