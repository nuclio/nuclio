/*
 * Copyright (c) 2018 VMware, Inc.
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of this software and
 * associated documentation files (the "Software"), to deal in the Software without restriction, including
 * without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is furnished to do
 * so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all copies or substantial
 * portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT
 * NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
 * IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
 * WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
 * SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */
package common

import (
	"fmt"
	"net/http"
)

// ErrorCode is unified definition of numerical error codes
type ErrorCode int32

// pre-defined error codes
const (
	// System Wide      41000 - 42000
	KinesisClientLibError ErrorCode = 41000

	// KinesisClientLibrary Retryable Errors 41001 - 41100
	KinesisClientLibRetryableError ErrorCode = 41001

	KinesisClientLibIOError         ErrorCode = 41002
	BlockedOnParentShardError       ErrorCode = 41003
	KinesisClientLibDependencyError ErrorCode = 41004
	ThrottlingError                 ErrorCode = 41005

	// KinesisClientLibrary NonRetryable Errors 41100 - 41200
	KinesisClientLibNonRetryableException ErrorCode = 41100

	InvalidStateError ErrorCode = 41101
	ShutdownError     ErrorCode = 41102

	// Kinesis Lease Errors 41200 - 41300
	LeasingError ErrorCode = 41200

	LeasingInvalidStateError          ErrorCode = 41201
	LeasingDependencyError            ErrorCode = 41202
	LeasingProvisionedThroughputError ErrorCode = 41203

	// Misc Errors 41300 - 41400
	// NotImplemented
	KinesisClientLibNotImplemented ErrorCode = 41301

	// Error indicates passing illegal or inappropriate argument
	IllegalArgumentError ErrorCode = 41302
)

var errorMap = map[ErrorCode]ClientLibraryError{
	KinesisClientLibError: {ErrorCode: KinesisClientLibError, Retryable: true, Status: http.StatusServiceUnavailable, Msg: "Top level error of Kinesis Client Library"},

	// Retryable
	KinesisClientLibRetryableError:  {ErrorCode: KinesisClientLibRetryableError, Retryable: true, Status: http.StatusServiceUnavailable, Msg: "Retryable exceptions (e.g. transient errors). The request/operation is expected to succeed upon (back off and) retry."},
	KinesisClientLibIOError:         {ErrorCode: KinesisClientLibIOError, Retryable: true, Status: http.StatusServiceUnavailable, Msg: "Error in reading/writing information (e.g. shard information from Kinesis may not be current/complete)."},
	BlockedOnParentShardError:       {ErrorCode: BlockedOnParentShardError, Retryable: true, Status: http.StatusServiceUnavailable, Msg: "Cannot start processing data for a shard because the data from the parent shard has not been completely processed (yet)."},
	KinesisClientLibDependencyError: {ErrorCode: KinesisClientLibDependencyError, Retryable: true, Status: http.StatusServiceUnavailable, Msg: "Cannot talk to its dependencies (e.g. fetching data from Kinesis, DynamoDB table reads/writes, emitting metrics to CloudWatch)."},
	ThrottlingError:                 {ErrorCode: ThrottlingError, Retryable: true, Status: http.StatusTooManyRequests, Msg: "Requests are throttled by a service (e.g. DynamoDB when storing a checkpoint)."},

	// Non-Retryable
	KinesisClientLibNonRetryableException: {ErrorCode: KinesisClientLibNonRetryableException, Retryable: false, Status: http.StatusServiceUnavailable, Msg: "Non-retryable exceptions. Simply retrying the same request/operation is not expected to succeed."},
	InvalidStateError:                     {ErrorCode: InvalidStateError, Retryable: false, Status: http.StatusServiceUnavailable, Msg: "Kinesis Library has issues with internal state (e.g. DynamoDB table is not found)."},
	ShutdownError:                         {ErrorCode: ShutdownError, Retryable: false, Status: http.StatusServiceUnavailable, Msg: "The RecordProcessor instance has been shutdown (e.g. and attempts a checkpiont)."},

	// Leasing
	LeasingError:                      {ErrorCode: LeasingError, Retryable: true, Status: http.StatusServiceUnavailable, Msg: "Top-level error type for the leasing code."},
	LeasingInvalidStateError:          {ErrorCode: LeasingInvalidStateError, Retryable: true, Status: http.StatusServiceUnavailable, Msg: "Error in a lease operation has failed because DynamoDB is an invalid state"},
	LeasingDependencyError:            {ErrorCode: LeasingDependencyError, Retryable: true, Status: http.StatusServiceUnavailable, Msg: "Error in a lease operation has failed because a dependency of the leasing system has failed."},
	LeasingProvisionedThroughputError: {ErrorCode: LeasingProvisionedThroughputError, Retryable: false, Status: http.StatusServiceUnavailable, Msg: "Error in a lease operation has failed due to lack of provisioned throughput for a DynamoDB table."},

	// IllegalArgumentError
	IllegalArgumentError: {ErrorCode: IllegalArgumentError, Retryable: false, Status: http.StatusBadRequest, Msg: "Error indicates that a method has been passed an illegal or inappropriate argument."},

	// Not Implemented
	KinesisClientLibNotImplemented: {ErrorCode: KinesisClientLibNotImplemented, Retryable: false, Status: http.StatusNotImplemented, Msg: "Not Implemented"},
}

// Message returns the message of the error code
func (c ErrorCode) Message() string {
	return errorMap[c].Msg
}

// MakeErr makes an error with default message
func (c ErrorCode) MakeErr() *ClientLibraryError {
	e := errorMap[c]
	return &e
}

// MakeError makes an error with message and data
func (c ErrorCode) MakeError(detail string) error {
	e := errorMap[c]
	return e.WithDetail(detail)
}

// ClientLibraryError is unified error
type ClientLibraryError struct {
	// ErrorCode is the numerical error code.
	ErrorCode `json:"code"`
	// Retryable is a bool flag to indicate the whether the error is retryable or not.
	Retryable bool `json:"tryable"`
	// Status is the HTTP status code.
	Status int `json:"status"`
	// Msg provides a terse description of the error. Its value is defined in errorMap.
	Msg string `json:"msg"`
	// Detail provides a detailed description of the error. Its value is set using WithDetail.
	Detail string `json:"detail"`
}

// Error implements error
func (e *ClientLibraryError) Error() string {
	var prefix string
	if e.Retryable {
		prefix = "Retryable"
	} else {
		prefix = "NonRetryable"
	}
	msg := fmt.Sprintf("%v Error [%d]: %s", prefix, int32(e.ErrorCode), e.Msg)
	if e.Detail != "" {
		msg = fmt.Sprintf("%s, detail: %s", msg, e.Detail)
	}
	return msg
}

// WithMsg overwrites the default error message
func (e *ClientLibraryError) WithMsg(format string, v ...interface{}) *ClientLibraryError {
	e.Msg = fmt.Sprintf(format, v...)
	return e
}

// WithDetail adds a detailed message to error
func (e *ClientLibraryError) WithDetail(format string, v ...interface{}) *ClientLibraryError {
	if len(e.Detail) == 0 {
		e.Detail = fmt.Sprintf(format, v...)
	} else {
		e.Detail += ", " + fmt.Sprintf(format, v...)
	}
	return e
}

// WithCause adds CauseBy to error
func (e *ClientLibraryError) WithCause(err error) *ClientLibraryError {
	if err != nil {
		// Store error message in Detail, so the info can be preserved
		// when CascadeError is marshaled to json.
		if len(e.Detail) == 0 {
			e.Detail = err.Error()
		} else {
			e.Detail += ", cause: " + err.Error()
		}
	}
	return e
}
