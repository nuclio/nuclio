package util

import (
	"fmt"
	"io/ioutil"
	"sort"
	"testing"

	"github.com/stretchr/testify/suite"
)

var code = `
package handler

import (
    "github.com/nuclio/nuclio-sdk"
)

// OK
func Handler1(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
}

// Not handler - lower case
func handler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
}

// Not handler - bad signature
func Handler(context *nuclio.Context, event nuclio.Event) error {
}

// OK
func AlsoHandler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
}
`

type ParseSuite struct {
	suite.Suite
}

func (suite *ParseSuite) TestHandlerNames() {
	tmp, err := ioutil.TempFile("", "test-parse")
	if err != nil {
		suite.FailNow("can't create code file", "error: %s", err)
	}
	fmt.Fprint(tmp, code)
	if err != nil {
		suite.FailNow("can't sync", "error: %s", err)
	}

	handlers, err := HandlerNames(tmp.Name())
	if err != nil {
		suite.FailNow("can't find handlers", "error: %s", err)
	}
	if !suite.Len(handlers, 2, "bad length") {
		suite.FailNow("")
	}
	sort.Strings(handlers)
	fmt.Println(handlers)
	if !suite.Equal(handlers[0], "AlsoHandler", "first handler") {
		suite.FailNow("")
	}
	if !suite.Equal(handlers[1], "Handler1", "first handler") {
		suite.FailNow("")
	}

}

func TestParse(t *testing.T) {
	suite.Run(t, new(ParseSuite))
}
