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
