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

package restful

import (
	"strings"

	"github.com/nuclio/logger"
)

// ErrorContainsVerifier verifies contents of returned errors
type ErrorContainsVerifier struct {
	logger          logger.Logger
	expectedStrings []string
}

// NewErrorContainsVerifier returns a new ErrorContainsVerifier
func NewErrorContainsVerifier(logger logger.Logger, expectedStrings []string) *ErrorContainsVerifier {
	return &ErrorContainsVerifier{
		logger,
		expectedStrings,
	}
}

// Verify verifies that the returned response contains the given errors
func (ecv *ErrorContainsVerifier) Verify(response map[string]interface{}) bool {

	// get the "error" key. expect it to be a string
	responseErrorInterface, found := response["error"]
	if !found {
		ecv.logger.WarnWith("Response does not contain an error key", "response", response)

		return false
	}

	// get the "error" key. expect it to be a string
	responseError, responseErrorInterfaceIsString := responseErrorInterface.(string)
	if !responseErrorInterfaceIsString {
		ecv.logger.WarnWith("Response error is not a string")

		return false
	}

	return ecv.foundExpectedString(responseError)
}

func (ecv *ErrorContainsVerifier) foundExpectedString(responseError string) bool {

	// iterate over expected strings, look for one of them
	for _, expectedString := range ecv.expectedStrings {
		if strings.Contains(responseError, expectedString) {
			return true
		}
	}

	ecv.logger.WarnWith("Expected string not found",
		"responseError", responseError)
	return false
}
