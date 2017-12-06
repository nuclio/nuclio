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

// Package errors provides an api similar to github.com/nuclio/nuclio/pkg/errors
package errors

// All error values returned from this package implement fmt.Formatter and can
// be formatted by the fmt package. The following verbs are supported
//
//     %s    print the error
//     %+v   extended format. Will print stack trace of errors

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
)

var (
	// ShowLineInfo sets if we collect location information (file, line)
	// (getting location information makes creating error slower ~550ns vs 2ns)
	ShowLineInfo bool
)

// Error implements error interface with call stack
type Error struct {
	message    string
	cause      error
	fileName   string
	lineNumber int
}

func init() {
	ShowLineInfo = len(os.Getenv("NUCLIO_NO_ERROR_LINE_INFO")) == 0
}

// caller return the caller informatin (file, line)
// Note this is sensitive to where it's called
func caller() (string, int) {
	pcs := make([]uintptr, 1)
	// skip 3 levels to get to the caller
	n := runtime.Callers(3, pcs)
	if n == 0 {
		return "", 0
	}

	pc := pcs[0] - 1
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "", 0
	}

	return fn.FileLine(pc)
}

// New returns a new error
func New(message string) error {
	err := &Error{message: message}
	if ShowLineInfo {
		err.fileName, err.lineNumber = caller()
	}
	return err
}

// Errorf returns a new Error
func Errorf(format string, args ...interface{}) error {
	err := &Error{message: fmt.Sprintf(format, args...)}
	if ShowLineInfo {
		err.fileName, err.lineNumber = caller()
	}
	return err
}

// Wrap returns a new error with err as cause, if err is nil will return nil
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}

	errObj := &Error{
		message: message,
		cause:   err,
	}

	if ShowLineInfo {
		errObj.fileName, errObj.lineNumber = caller()
	}
	return errObj
}

// Wrapf returns a new error with err as cause, if err is nil will return nil
func Wrapf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}

	message := fmt.Sprintf(format, args...)
	errObj := &Error{
		message: message,
		cause:   err,
	}
	if ShowLineInfo {
		errObj.fileName, errObj.lineNumber = caller()
	}
	return errObj
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

// LineInfo info returns the location (file, line) where the error was created
func (err *Error) LineInfo() (string, int) {
	return err.fileName, err.lineNumber
}

// reverse reverses a slice in place
func reverse(slice []error) {
	for left, right := 0, len(slice)-1; left < right; left, right = left+1, right-1 {
		slice[left], slice[right] = slice[right], slice[left]
	}
}

// GetErrorStack return stack of messges (oldest on top)
// if n == -1 returns the whole stack
func GetErrorStack(err error, depth int) []error {
	errors := []error{err}

	errObj := asError(err)
	if errObj == nil {
		return errors
	}

	for errObj = asError(errObj.cause); errObj != nil; errObj = asError(errObj.cause) {
		errors = append(errors, errObj)
	}

	reverse(errors)
	if depth > 0 {
		if depth > len(errors) {
			depth = len(errors)
		}
		errors = errors[:depth]
	}
	return errors
}

// PrintErrorStack prints the error stack into out upto depth levels
// If n == 1 then prints the whole stack
func PrintErrorStack(out io.Writer, err error, depth int) {
	pathLen := 40

	stack := GetErrorStack(err, depth)
	errObj := asError(stack[0])

	if errObj != nil && errObj.lineNumber != 0 {
		cause := errObj.Error()
		if errObj.cause != nil {
			cause = errObj.cause.Error()
		}

		fmt.Fprintf(out, "\nError - %s", cause)
		fmt.Fprintf(out, "\n    %s:%d\n", trimPath(errObj.fileName, pathLen), errObj.lineNumber)
	} else {
		fmt.Fprintf(out, "\nError - %s", stack[0].Error())
	}

	fmt.Fprintf(out, "\nCall stack:")

	for _, e := range stack {
		errObj := asError(e)
		fmt.Fprintf(out, "\n%s", e.Error())
		if errObj != nil && errObj.lineNumber != 0 {
			fmt.Fprintf(out, "\n    %s:%d", trimPath(errObj.fileName, pathLen), errObj.lineNumber)
		}
	}
	out.Write([]byte{'\n'})
}

// Cause is the cause of the error
func Cause(err error) error {
	errObj := asError(err)
	if errObj == nil {
		return nil
	}
	return errObj.cause
}

// sumLengths return sum of lengths of strings
func sumLengths(parts []string) int {
	total := 0
	for _, s := range parts {
		total += len(s)
	}
	return total
}

// trimPath shortens fileName to be at most size characters
func trimPath(fileName string, size int) string {
	if len(fileName) <= size {
		return fileName
	}

	// We'd like to cut at directory boundary
	parts := strings.Split(fileName, "/")
	for sumLengths(parts) > size && len(parts) > 1 {
		parts = parts[1:]
	}

	return ".../" + strings.Join(parts, "/")
}

// Format formats an error
func (err *Error) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			PrintErrorStack(s, err, -1)
		}
		fallthrough
	case 's':
		fmt.Fprintf(s, err.Error())
	case 'q':
		fmt.Fprintf(s, "%q", err.Error())
	}
}
