//go:build test_unit

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
	"fmt"
	"testing"
	"time"

	"github.com/nuclio/errors"
	"github.com/stretchr/testify/suite"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type RetryTestSuite struct {
	suite.Suite
}

func (s *RetryTestSuite) TestIsK8sRetryableErrorClassification() {
	groupResource := schema.GroupResource{Group: "", Resource: "services"}

	cases := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "nil",
			err:       nil,
			retryable: false,
		},
		{
			name:      "typed_server_timeout",
			err:       apierrors.NewServerTimeout(groupResource, "update", 0),
			retryable: true,
		},
		{
			name:      "typed_timeout",
			err:       apierrors.NewTimeoutError("request timed out", 0),
			retryable: true,
		},
		{
			name:      "typed_too_many_requests",
			err:       apierrors.NewTooManyRequestsError("calm down"),
			retryable: true,
		},
		{
			name:      "typed_internal",
			err:       apierrors.NewInternalError(fmt.Errorf("boom")),
			retryable: true,
		},
		{
			name:      "typed_service_unavailable",
			err:       apierrors.NewServiceUnavailable("nope"),
			retryable: true,
		},
		{
			name:      "typed_wrapped_server_timeout",
			err:       errors.Wrap(apierrors.NewServerTimeout(groupResource, "update", 0), "Failed to update resource"),
			retryable: true,
		},
		{
			name:      "context_deadline_exceeded",
			err:       context.DeadlineExceeded,
			retryable: true,
		},
		{
			name:      "context_deadline_wrapped",
			err:       errors.Wrap(context.DeadlineExceeded, "Failed to update resource"),
			retryable: true,
		},
		{
			name:      "string_apiserver_504",
			err:       fmt.Errorf("Timeout: request did not complete within requested timeout - context deadline exceeded"),
			retryable: true,
		},
		{
			name:      "string_unable_to_return_response",
			err:       fmt.Errorf("the server was unable to return a response in the time allotted, but may still be processing the request"),
			retryable: true,
		},
		{
			name:      "string_etcd_leader_changed",
			err:       fmt.Errorf("etcdserver: leader changed"),
			retryable: true,
		},
		{
			name:      "string_rpc_error",
			err:       fmt.Errorf("rpc error: code = Unavailable"),
			retryable: true,
		},
		{
			name:      "string_i_o_timeout",
			err:       fmt.Errorf("dial tcp 10.0.0.1:443: i/o timeout"),
			retryable: true,
		},
		{
			name: "non_retryable_not_found",
			err: apierrors.NewNotFound(groupResource,
				"some-resource"),
			retryable: false,
		},
		{
			name:      "non_retryable_bad_request",
			err:       apierrors.NewBadRequest("malformed"),
			retryable: false,
		},
		{
			name:      "non_retryable_conflict",
			err:       apierrors.NewConflict(groupResource, "some-resource", fmt.Errorf("conflict")),
			retryable: false,
		},
		{
			name:      "non_retryable_generic",
			err:       fmt.Errorf("something else broke"),
			retryable: false,
		},
		{
			name: "non_retryable_status_error_other_reason",
			err: &apierrors.StatusError{
				ErrStatus: metav1.Status{
					Status:  metav1.StatusFailure,
					Reason:  metav1.StatusReasonForbidden,
					Message: "forbidden",
				},
			},
			retryable: false,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.Assert().Equal(tc.retryable, IsK8sRetryableError(tc.err))
		})
	}
}

func (s *RetryTestSuite) TestRequestWithRetrySucceedsAfterTransientFailures() {
	calls := 0
	fn := func() (string, error) {
		calls++
		if calls < 3 {
			return "", apierrors.NewServerTimeout(schema.GroupResource{Resource: "services"}, "update", 0)
		}
		return "ok", nil
	}

	result, err := RequestWithRetry[string](fn, 5, time.Millisecond)
	s.Require().NoError(err)
	s.Assert().Equal("ok", result)
	s.Assert().Equal(3, calls)
}

func (s *RetryTestSuite) TestRequestWithRetryFailsFastOnNonRetryable() {
	calls := 0
	fn := func() (string, error) {
		calls++
		return "", apierrors.NewBadRequest("malformed")
	}

	_, err := RequestWithRetry[string](fn, 5, time.Millisecond)
	s.Require().Error(err)
	s.Assert().Equal(1, calls, "non-retryable error should not be retried")
}

func (s *RetryTestSuite) TestRequestWithRetryExhaustsThenWraps() {
	calls := 0
	fn := func() (string, error) {
		calls++
		return "", apierrors.NewServerTimeout(schema.GroupResource{Resource: "services"}, "update", 0)
	}

	_, err := RequestWithRetry[string](fn, 3, time.Millisecond)
	s.Require().Error(err)
	s.Assert().Equal(3, calls)
	s.Assert().Contains(err.Error(), "Kubernetes call failed after 3 retries")
}

func TestRetryTestSuite(t *testing.T) {
	suite.Run(t, new(RetryTestSuite))
}
