/*
Copyright 2025 The Nuclio Authors.

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

package clients

import (
	"context"
	stderrors "errors"
	"strings"
	"time"

	"github.com/nuclio/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// default retry parameters
const MaxRetries = 5
const Delay = 1 * time.Second

func RequestWithRetry[T any](fn func() (T, error), maxRetries int, delay time.Duration) (T, error) {
	var result T
	var err error

	for i := 0; i < maxRetries; i++ {
		result, err = fn()
		if err == nil {
			return result, nil
		}
		if !IsK8sRetryableError(err) {
			return result, err // Not retryable, fail fast
		}

		time.Sleep(delay)
	}
	return result, errors.Wrapf(err, "Kubernetes call failed after %d retries", maxRetries)
}

// IsK8sRetryableError reports whether err describes a transient Kubernetes API,
// etcd, or network condition where the operation may have succeeded server-side
// but the response never reached the client — i.e. it is safe to retry.
//
// It first tries typed apierror predicates and context.DeadlineExceeded, then
// falls back to substring matching for cases where the error has been wrapped
// (the project's errors library produces plain strings via Error(), so typed
// checks against the wrapped value fail).
func IsK8sRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Typed checks — preferred when the original apierror is still in the
	// chain (e.g. fresh from the kubernetes client, before nuclio wrapping).
	if apierrors.IsServerTimeout(err) ||
		apierrors.IsTimeout(err) ||
		apierrors.IsTooManyRequests(err) ||
		apierrors.IsInternalError(err) ||
		apierrors.IsServiceUnavailable(err) {
		return true
	}
	if stderrors.Is(err, context.DeadlineExceeded) {
		return true
	}

	errStr := err.Error()
	return strings.Contains(errStr, "Error: etcdserver: ") ||
		strings.Contains(errStr, "etcdserver: leader changed") ||
		strings.Contains(errStr, "rpc error") ||
		strings.Contains(errStr, "Kubernetes cluster unreachable") ||
		strings.Contains(errStr, "Unable to connect to the server") ||
		strings.Contains(errStr, "Internal error occurred: resource quota evaluation timed out") ||
		strings.Contains(errStr, ":443: i/o timeout") ||
		strings.Contains(errStr, "Timeout: request did not complete within") ||
		strings.Contains(errStr, "the server was unable to return a response in the time allotted") ||
		strings.Contains(errStr, "context deadline exceeded")
}
