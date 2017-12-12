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

package errors

import (
	"bytes"
	"fmt"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ErrorsTestSuite struct {
	suite.Suite
}

func (suite *ErrorsTestSuite) TestNew() {
	var err error // Make sure we conform to Error interface
	message := "hello"

	err = New(message)
	suite.Require().Equal(message, err.Error())
}

func (suite *ErrorsTestSuite) TestErrorf() {
	var err error // Make sure we conform to Error interface
	format := "bad math: %d != %q"
	args := []interface{}{1, "2"}

	err = Errorf(format, args...)
	suite.Require().Equal(fmt.Sprintf(format, args...), err.Error())
}

func (suite *ErrorsTestSuite) TestWrap() {
	cause := fmt.Errorf("first error")
	message := "second error"
	err := Wrap(cause, message)

	suite.Require().Equal(cause, Cause(err))
	suite.Require().Equal(message, err.Error())
}

func (suite *ErrorsTestSuite) TestWrapf() {
	cause := fmt.Errorf("first error")
	format := "bad math: %d != %q"
	args := []interface{}{1, "2"}

	err := Wrapf(cause, format, args...)

	suite.Require().Equal(cause, Cause(err))
	suite.Require().Equal(fmt.Sprintf(format, args...), err.Error())
}

func (suite *ErrorsTestSuite) TestFormat_s() {
	cause := fmt.Errorf("first error")
	message := "second error"
	err := Wrap(cause, message)

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s", err)

	suite.Require().Equal(message, buf.String())
}

func (suite *ErrorsTestSuite) TestFormat_q() {
	cause := fmt.Errorf("first error")
	message := "second error"
	err := Wrap(cause, message)

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%q", err)

	suite.Require().Equal(fmt.Sprintf("%q", message), buf.String())
}

func genError() error {
	e1 := New("e1")
	e2 := Wrap(e1, "e2")
	e3 := Wrap(e2, "e3")

	return e3
}

func (suite *ErrorsTestSuite) TestFormat_v() {
	err := genError()
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%+v", err)

	for _, err := range GetErrorStack(err, -1) {
		errObj := err.(*Error)
		fileName, lineNumber := errObj.LineInfo()
		lineInfo := fmt.Sprintf("%s:%d", path.Base(fileName), lineNumber)
		suite.Require().True(strings.Contains(buf.String(), lineInfo))
		suite.Require().True(strings.Contains(buf.String(), err.Error()))
	}
}

func (suite *ErrorsTestSuite) TestGetErrorStack() {
	total := 10

	var messages []string
	for i := 0; i < total; i++ {
		messages = append(messages, fmt.Sprintf("message #%d", i))
	}
	err := Errorf(messages[0])

	for _, message := range messages[1:] {
		err = Wrap(err, message)
	}

	// Check partial
	size := 4
	messageStack := GetErrorStack(err, size)
	suite.Require().Equal(size, len(messageStack))
	suite.Require().Equal(messageStack[0].Error(), messages[0])

	// Check too much
	messageStack = GetErrorStack(err, total+200)
	suite.Require().Equal(total, len(messageStack))

	// Check regular error
	message := "hello there"
	stdErr := fmt.Errorf(message)
	messageStack = GetErrorStack(stdErr, 7)
	suite.Require().Equal(1, len(messageStack))
	suite.Require().Equal(messageStack[0].Error(), message)
}

func (suite *ErrorsTestSuite) TestReverse() {
	e1, e2, e3, e4 := New("1"), New("2"), New("3"), New("4")

	errors := []error{}
	reverse(errors)

	errors = []error{e1, e2, e3, e4}
	reverse(errors)
	suite.Require().Equal([]error{e4, e3, e2, e1}, errors)

	errors = []error{e1, e2, e3}
	reverse(errors)
	suite.Require().Equal([]error{e3, e2, e1}, errors)

	errors = []error{e1}
	reverse(errors)
	suite.Require().Equal([]error{e1}, errors)
}

func (suite *ErrorsTestSuite) TestPrintErrorStack() {
	err := genError()
	var buf bytes.Buffer

	PrintErrorStack(&buf, err, -1)

	for _, err := range GetErrorStack(err, -1) {
		errObj := err.(*Error)
		fileName, lineNumber := errObj.LineInfo()
		lineInfo := fmt.Sprintf("%s:%d", path.Base(fileName), lineNumber)
		suite.Require().True(strings.Contains(buf.String(), lineInfo))
		suite.Require().True(strings.Contains(buf.String(), err.Error()))
	}

	depth := 2
	buf.Reset()
	PrintErrorStack(&buf, err, depth)
	for _, err := range GetErrorStack(err, depth) {
		errObj := err.(*Error)
		fileName, lineNumber := errObj.LineInfo()
		lineInfo := fmt.Sprintf("%s:%d", path.Base(fileName), lineNumber)
		suite.Require().True(strings.Contains(buf.String(), lineInfo))
		suite.Require().True(strings.Contains(buf.String(), err.Error()))
	}

	suite.Require().False(strings.Contains(buf.String(), "e3"))
}

func TestErrors(t *testing.T) {
	suite.Run(t, new(ErrorsTestSuite))
}
