package k8s

import (
	"strings"
	"time"

	"github.com/nuclio/errors"
)

// default retry parameters
const maxRetries = 5
const delay = 1 * time.Second

func requestWithRetry[T any](fn func() (T, error), maxRetries int, delay time.Duration) (T, error) {
	var result T
	var err error

	for i := 0; i < maxRetries; i++ {
		result, err = fn()
		if err == nil {
			return result, nil
		}
		if !isK8sRetryableErrors(err) {
			return result, err // Not retryable, fail fast
		}

		time.Sleep(delay)
	}
	return result, errors.Wrapf(err, "Kubernetes call failed after %d retries", maxRetries)
}

func isK8sRetryableErrors(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "Error: etcdserver: ") ||
		strings.Contains(errStr, "etcdserver: leader changed") ||
		strings.Contains(errStr, "rpc error") ||
		strings.Contains(errStr, "Kubernetes cluster unreachable") ||
		strings.Contains(errStr, "Unable to connect to the server") ||
		strings.Contains(errStr, "Internal error occurred: resource quota evaluation timed out") ||
		strings.Contains(errStr, ":443: i/o timeout")
}
