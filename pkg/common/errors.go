package common

import (
	"net/http"

	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
)

func ResolveErrorStatusCodeOrDefault(err error, defaultStatusCode int) int {

	// resolve from top level
	switch typedError := err.(type) {
	case nuclio.ErrorWithStatusCode:
		return typedError.StatusCode()
	case *nuclio.ErrorWithStatusCode:
		return typedError.StatusCode()
	}

	// resolve from root cause
	switch rootCauseTypedError := errors.Cause(err).(type) {
	case nuclio.ErrorWithStatusCode:
		return rootCauseTypedError.StatusCode()
	case *nuclio.ErrorWithStatusCode:
		return rootCauseTypedError.StatusCode()
	}

	switch err.(type) {
	case *errors.Error:
		return http.StatusInternalServerError
	}

	// unable to resolve, returning default
	return defaultStatusCode
}
