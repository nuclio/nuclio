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

package processor

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/common/headers"

	"github.com/nuclio/nuclio-sdk-go"
	"github.com/stretchr/testify/suite"
)

type TypesTestSuite struct {
	suite.Suite
}

func (suite *TypesTestSuite) TestEventDiscardedDuringDrain() {
	for _, testCase := range []struct {
		name     string
		response nuclio.ProcessingResult
		expected bool
	}{
		{
			name:     "NilResponse",
			response: nil,
			expected: false,
		},
		{
			name:     "NoHeaders",
			response: &nuclio.Response{},
			expected: false,
		},
		{
			name: "DiscardedHeaderTrue",
			response: &nuclio.Response{
				Headers: map[string]interface{}{headers.StreamEventDiscarded: true},
			},
			expected: true,
		},
		{
			name: "DiscardedHeaderFalse",
			response: &nuclio.Response{
				Headers: map[string]interface{}{headers.StreamEventDiscarded: false},
			},
			expected: false,
		},
		{
			name: "DiscardedHeaderNonBool",
			response: &nuclio.Response{
				Headers: map[string]interface{}{headers.StreamEventDiscarded: "true"},
			},
			expected: false,
		},
		{
			name: "UnrelatedHeader",
			response: &nuclio.Response{
				Headers: map[string]interface{}{headers.StreamNoAck: true},
			},
			expected: false,
		},
	} {
		suite.Run(testCase.name, func() {
			suite.Equal(testCase.expected, EventDiscardedDuringDrain(testCase.response))
		})
	}
}

func TestTypesTestSuite(t *testing.T) {
	suite.Run(t, new(TypesTestSuite))
}
