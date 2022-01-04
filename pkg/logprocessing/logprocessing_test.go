//go:build test_unit

/*
Copyright 2021 The Nuclio Authors.

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

package logprocessing

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type LogProcessorTestSuite struct {
	suite.Suite
}

func (suite *LogProcessorTestSuite) TestCreateKeyValuePairs() {
	for _, testCase := range []struct {
		name          string
		args          map[string]string
		expectedOneOf []string
	}{
		{
			name: "sanity",
			args: map[string]string{
				"a": "b",
				"c": "d",
			},
			expectedOneOf: []string{"a=\"b\" || c=\"d\"", "c=\"d\" || a=\"b\""},
		},
	} {
		suite.Run(testCase.name, func() {
			encodedKeyValuePairs := createKeyValuePairs(testCase.args)
			suite.Require().Contains(testCase.expectedOneOf, encodedKeyValuePairs)
		})
	}
}

func TestLogProcessorTestSuite(t *testing.T) {
	suite.Run(t, new(LogProcessorTestSuite))
}
