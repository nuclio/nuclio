package common

import (
	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
)

func ResolveErrorStatusCodeOrDefault(err error, defaultStatusCode int) int {

	// resolve from top level
	if errWithStatus, ok := err.(*nuclio.ErrorWithStatusCode); ok {
		return errWithStatus.StatusCode()
	}

	// resolve from root cause
	if rootCauseWithStatus, ok := errors.RootCause(err).(*nuclio.ErrorWithStatusCode); ok {
		return rootCauseWithStatus.StatusCode()
	}

	// unable to resolve, returning default
	return defaultStatusCode
}
