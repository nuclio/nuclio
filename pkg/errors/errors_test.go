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

func reverse(slice []string) []string {
	newSlice := make([]string, len(slice))
	for left, right := 0, len(slice)-1; left < right; left, right = left+1, right-1 {
		newSlice[left], newSlice[right] = slice[right], slice[left]
	}
	if len(slice)%2 == 1 {
		newSlice[len(slice)/2] = slice[len(slice)/2]
	}
	return newSlice
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
	suite.Require().Equal(messageStack[0].Error(), messages[len(messages)-1])

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

func TestErrors(t *testing.T) {
	suite.Run(t, new(ErrorsTestSuite))
}
