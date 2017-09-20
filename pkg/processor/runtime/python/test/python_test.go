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

	"github.com/nuclio/nuclio/pkg/processor/eventsource/http/test/suite"

	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	httpsuite.TestSuite
}

func (suite *TestSuite) TestOutputs() {
	// suite.T().Skip()

	statusOK := http.StatusOK
	statusCreated := http.StatusCreated

	headersContentTypeTextPlain := map[string]string{"content-type": "text/plain"}
	headersContentTypeApplicationJSON := map[string]string{"content-type": "application/json"}

	suite.BuildAndRunFunction("parser",
		path.Join(suite.getPythonDir(), "outputter"),
		"python",
		map[int]int{8080: 8080},
		func() bool {
			requestPort := 8080

			// function returns a string
			if !suite.SendRequestVerifyResponse(requestPort,
				"POST",
				"/",
				nil,
				"return_string",
				nil,
				headersContentTypeTextPlain,
				"a string",
				&statusOK,
				nil) {
				return false
			}

			// function returns status and a string
			if !suite.SendRequestVerifyResponse(requestPort,
				"POST",
				"/",
				nil,
				"return_status_and_string",
				nil,
				headersContentTypeTextPlain,
				"a string after status",
				&statusCreated,
				nil) {
				return false
			}

			// function returns a dict - should be converted to JSON
			if !suite.SendRequestVerifyResponse(requestPort,
				"POST",
				"/",
				nil,
				"return_dict",
				nil,
				headersContentTypeApplicationJSON,
				map[string]interface{}{"a": "dict", "b": "foo"},
				&statusOK,
				nil) {
				return false
			}

			// function returns a "response" object. TODO: check headers
			// map[string]string{"a": "1", "b": "2", "h1": "v1", "h2": "v2", "Content-Type": "text/plain"}
			if !suite.SendRequestVerifyResponse(requestPort,
				"POST",
				"/",
				map[string]string{"a": "1", "b": "2"},
				"response",
				nil,
				nil,
				"response body",
				&statusCreated,
				nil) {
				return false
			}

			return true
		})
}

func (suite *TestSuite) getPythonDir() string {
	return path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "runtime", "python", "test")
}

func TestTestSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}
