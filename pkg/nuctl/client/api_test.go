//go:build test_unit

/*
Copyright 2023 The Nuclio Authors.

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

package client

import (
	"testing"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type apiClientTestSuite struct {
	suite.Suite

	logger logger.Logger
}

func (suite *apiClientTestSuite) SetupTest() {
	var err error

	// create logger
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)
}

func (suite *apiClientTestSuite) TestParseValidResourceAllocation() {
	for _, testCase := range []struct {
		name           string
		apiURL         string
		requestTimeout string
		accessKey      string
		username       string
		expectError    bool
		expectedError  string
		expectedURL    string
	}{
		{
			name:           "wrong-url",
			apiURL:         "http://[invalid-url",
			requestTimeout: "5s",
			username:       "name",
			accessKey:      "access-key",
			expectError:    true,
			expectedError:  "Failed to parse API URL",
		},
		{
			name:           "no-access-key",
			apiURL:         "http://api-url.com",
			requestTimeout: "5s",
			username:       "name",
			accessKey:      "",
			expectError:    true,
			expectedError:  "Access key must be provided",
		},
		{
			name:           "no-api-suffix-in-url",
			apiURL:         "https://api-url.com",
			requestTimeout: "5s",
			username:       "name",
			accessKey:      "access-key",
			expectedURL:    "https://api-url.com/api",
		},
	} {
		suite.Run(testCase.name, func() {
			client, err := NewNuclioAPIClient(suite.logger, testCase.apiURL, testCase.requestTimeout, testCase.username, testCase.accessKey, true)
			if testCase.expectError {
				suite.Require().Error(err)
				suite.Require().Contains(err.Error(), testCase.expectedError)
			} else {
				suite.Require().NoError(err)
				if testCase.expectedURL != "" {
					suite.Require().Equal(testCase.expectedURL, client.apiURL)
				}
			}
		})
	}
}

func TestAPIClientTestSuite(t *testing.T) {
	suite.Run(t, new(apiClientTestSuite))
}
