package common

import (
	"context"
	"reflect"
	"runtime"
	"time"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

func getFunctionName(fn interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
}

// give either retryInterval or backoff
func RetryFunc(ctx context.Context,
	loggerInstance logger.Logger,
	attempts int,
	retryInterval *time.Duration,
	backoff *Backoff,
	fn func(int) (bool, error)) error {

	var err error
	var retry bool

	for attempt := 1; attempt <= attempts; attempt++ {
		retry, err = fn(attempt)

		// if there's no need to retry - we're done
		if !retry {
			return err
		}

		// are we out of time?
		if ctx.Err() != nil {

			loggerInstance.WarnWithCtx(ctx,
				"Context error detected during retries",
				"ctxErr", ctx.Err(),
				"previousErr", err,
				"function", getFunctionName(fn),
				"attempt", attempt)

			// return the error if one was provided
			if err != nil {
				return err
			}

			return ctx.Err()
		}

		if backoff != nil {
			time.Sleep(backoff.Duration())
		} else {
			if retryInterval == nil {
				return errors.New("Either retry interval or backoff must be given")
			}
			time.Sleep(*retryInterval)
		}
	}

	// attempts exhausted and we're unsuccessful
	// Return the original error for later checking
	loggerInstance.WarnWithCtx(ctx,
		"Failed final attempt to invoke function",
		"function", getFunctionName(fn),
		"err", err,
		"attempts", attempts)

	// this shouldn't happen
	if err == nil {
		loggerInstance.ErrorWithCtx(ctx,
			"Failed final attempt to invoke function, but error is nil. This shouldn't happen",
			"function", getFunctionName(fn),
			"err", err,
			"attempts", attempts)
		return errors.New("Failed final attempt to invoke function without proper error supplied")
	}
	return err
}

func MakeRange(min, max int) []int {
	a := make([]int, max-min+1)
	for i := range a {
		a[i] = min + i
	}
	return a
}

func IntSliceContainsInt(slice []int, number int) bool {
	for _, intInSlice := range slice {
		if intInSlice == number {
			return true
		}
	}

	return false
}

func IntSlicesEqual(slice1 []int, slice2 []int) bool {
	if len(slice1) != len(slice2) {
		return false
	}

	for intIndex := 0; intIndex < len(slice1); intIndex++ {
		if slice1[intIndex] != slice2[intIndex] {
			return false
		}
	}

	return true
}

func Uint64SlicesEqual(slice1 []uint64, slice2 []uint64) bool {
	if len(slice1) != len(slice2) {
		return false
	}

	for intIndex := 0; intIndex < len(slice1); intIndex++ {
		if slice1[intIndex] != slice2[intIndex] {
			return false
		}
	}

	return true
}

func StringSlicesEqual(slice1 []string, slice2 []string) bool {
	if len(slice1) != len(slice2) {
		return false
	}

	for stringIndex := 0; stringIndex < len(slice1); stringIndex++ {
		if slice1[stringIndex] != slice2[stringIndex] {
			return false
		}
	}

	return true
}
