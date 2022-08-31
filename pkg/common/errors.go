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
	"reflect"

	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
)

func ResolveErrorStatusCodeOrDefault(err error, defaultStatusCode int) int {

	// iterate over error stack to find the result status code (top down scan - return the first one)
	currentErr := err
	for {

		// if this error has status code, return it
		switch typedError := currentErr.(type) {
		case nuclio.ErrorWithStatusCode:
			return typedError.StatusCode()
		case *nuclio.ErrorWithStatusCode:
			return typedError.StatusCode()
		}

		// in case the previous level didn't have a status code - get the next cause in the error stack
		cause := errors.Cause(currentErr)

		// if there's no cause, we're done
		// if the cause is not comparable, it's not an Error and we're done
		// if the cause == the error, we're done since that's what Cause() returns
		if cause == nil || !reflect.TypeOf(cause).Comparable() || cause == currentErr {
			break
		}

		currentErr = cause
	}

	if _, ok := err.(*errors.Error); ok {
		return http.StatusInternalServerError
	}

	// unable to resolve, returning default
	return defaultStatusCode
}
