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
	"testing"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type ResultSuite struct {
	suite.Suite
}

func (suite *ResultSuite) createLogger() logger.Logger {
	loggerInstance, err := nucliozap.NewNuclioZapTest("result-test")
	suite.Require().NoError(err, "Can't create logger")

	return loggerInstance
}

func (suite *ResultSuite) TestUnmarshalResponseData() {
	for _, testCase := range []struct {
		name               string
		data               []byte
		unmarshalledResult []*Result
	}{
		{
			name: "single-result",
			data: []byte("{\"body\": \"123\", \"content_type\": \"123\", \"headers\": {}, \"status_code\": 200, \"body_encoding\": \"text\"}"),
			unmarshalledResult: []*Result{{
				StatusCode:   200,
				ContentType:  "123",
				Body:         "123",
				BodyEncoding: "text",
				DecodedBody:  []uint8{49, 50, 51},
				Headers:      map[string]interface{}{},
			}},
		},
		{
			name: "batch-result",
			data: []byte("[{\"body\": \"123\", \"content_type\": \"123\", \"headers\": {}, \"status_code\": 200, \"body_encoding\": \"text\"}]"),
			unmarshalledResult: []*Result{{
				StatusCode:   200,
				ContentType:  "123",
				Body:         "123",
				BodyEncoding: "text",
				DecodedBody:  []uint8{49, 50, 51},
				Headers:      map[string]interface{}{},
			}},
		},
	} {
		suite.Run(testCase.name, func() {
			unmarshalledResults := NewBatchedResults()
			unmarshalledResults.UnmarshalResponseData(suite.createLogger(), testCase.data)
			suite.Require().Equal(unmarshalledResults.Results, testCase.unmarshalledResult)
		})
	}
}

func TestRuntime(t *testing.T) {
	suite.Run(t, new(ResultSuite))
}
