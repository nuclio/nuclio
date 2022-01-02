//go:build test_unit

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

package common

import (
	"net/http"
	"testing"

	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/stretchr/testify/suite"
)

type ErrorsTestSuite struct {
	suite.Suite
}

func (ets *ErrorsTestSuite) TestResolveErrorStatusCodeOrDefault() {
	for _, testCase := range []struct {
		inputError         error
		expectedStatusCode int
		defaultStatusCode  int
	}{

		// test get status code from deepest error cause
		{
			inputError:         errors.Wrap(errors.Wrap(nuclio.NewErrBadRequest("err"), "err"), "err"),
			expectedStatusCode: http.StatusBadRequest,
		},

		// test get status code from middle error cause
		{
			inputError:         errors.Wrap(errors.Wrap(nuclio.WrapErrConflict(nuclio.NewErrBadRequest("err")), "err"), "err"),
			expectedStatusCode: http.StatusConflict,
		},

		// test get status code from the top
		{
			inputError:         nuclio.WrapErrMethodNotAllowed(errors.Wrap(nuclio.WrapErrConflict(nuclio.NewErrBadRequest("err")), "err")),
			expectedStatusCode: http.StatusMethodNotAllowed,
		},

		// test get general internal status code when error has no status code
		{
			inputError:         errors.Wrap(errors.Wrap(errors.New("err"), "err"), "err"),
			expectedStatusCode: http.StatusInternalServerError,
			defaultStatusCode:  http.StatusOK,
		},

		// test get default status code when error is nil
		{
			inputError:         nil,
			expectedStatusCode: http.StatusOK,
			defaultStatusCode:  http.StatusOK,
		},
	} {

		// set default status code
		defaultStatusCode := http.StatusInternalServerError
		if testCase.defaultStatusCode != 0 {
			defaultStatusCode = testCase.defaultStatusCode
		}

		// run the tested function
		statusCode := ResolveErrorStatusCodeOrDefault(testCase.inputError, defaultStatusCode)

		// validate we got the expected status code
		ets.Require().Equal(testCase.expectedStatusCode, statusCode)
	}
}

func TestErrorsTestSuite(t *testing.T) {
	suite.Run(t, new(ErrorsTestSuite))
}
