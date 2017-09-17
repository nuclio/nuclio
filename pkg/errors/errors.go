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

// Package errors provides an api similar to github.com/pkg/errors
package errors

// All error values returned from this package implement fmt.Formatter and can
// be formatted by the fmt package. The following verbs are supported
//
//     %s    print the error
//     %v   extended format. Will print stack trace of errors

import (
	"fmt"
)

// Error implements error interface with call stack
type Error struct {
	message string
	cause   error
}

// New returns a new error
func New(message string) error {
	return &Error{message: message}
}

// Errorf returns a new Error
func Errorf(format string, args ...interface{}) error {
	message := fmt.Sprintf(format, args...)
	return &Error{message: message}
}

// Wrap returns a new error with err as cause, if err is nil will return nil
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}

	return &Error{
		message: message,
		cause:   err,
	}
}

// Wrapf returns a new error with err as cause, if err is nil will return nil
func Wrapf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}

	message := fmt.Sprintf(format, args...)
	return &Error{
		message: message,
		cause:   err,
	}
}

// Error is the string representation of the error
func (err *Error) Error() string {
	return err.message
}

func asError(err error) *Error {
	errObj, ok := err.(*Error)
	if !ok {
		return nil
	}
	return errObj
}

// GetMessageStack return stack of messges (newest on top)
func GetMessageStack(err error, n int) []string {
	errObj := asError(err)

	if errObj == nil {
		return []string{err.Error()}
	}

	messages := make([]string, n)
	var i int

	for ; errObj != nil && i < n; errObj, i = asError(errObj.cause), i+1 {
		messages[i] = errObj.Error()
	}

	return messages[:i]
}

// Cause is the cause of the error
func Cause(err error) error {
	errObj := asError(err)
	if errObj == nil {
		return nil
	}
	return errObj.cause
}

// Format formats an error
func (err *Error) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		var current error
		first := true

		current = err
		for current != nil {
			errObj, ok := current.(*Error)
			if !first {
				fmt.Fprintf(s, "\n")
			}
			fmt.Fprintf(s, "%s", current.Error())

			first = false
			if !ok {
				break
			}
			current = Cause(errObj)
		}
	case 's':
		fmt.Fprintf(s, err.Error())
	case 'q':
		fmt.Fprintf(s, "%q", err.Error())
	}
}
