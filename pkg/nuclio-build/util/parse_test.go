package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"testing"
	"text/template"

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

var badCode = `
package
`

var codeTemplate = template.Must(template.New("code").Parse(`
package handler

import (
    "github.com/nuclio/nuclio-sdk"
)

func Handler{{.}}(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
}

// Not handler - lower case
func handler{{.}}(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
}
`))

type ParseSuite struct {
	suite.Suite
}

func (suite *ParseSuite) failOnError(err error, fmt string, args ...interface{}) {
	if err != nil {
		suite.FailNowf(err.Error(), fmt, args...)
	}
}

func (suite *ParseSuite) parseCode(code string) ([]string, error) {
	tmp, err := ioutil.TempFile("", "test-parse")
	suite.failOnError(err, "can't create code file")
	defer os.Remove(tmp.Name())

	fmt.Fprint(tmp, code)
	err = tmp.Close()
	suite.failOnError(err, "can't sync")
	return HandlerNames(tmp.Name())
}

func (suite *ParseSuite) TestHandlerNames() {
	handlers, err := suite.parseCode(code)
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

func (suite *ParseSuite) TestBadCode() {
	_, err := suite.parseCode(badCode)
	if err == nil {
		suite.FailNow("no error on bad code")
	}
}

func (suite *ParseSuite) TestFindHandlers() {
	path, err := ioutil.TempDir("", "parse-test")
	suite.failOnError(err, "can't create temp directory")
	n := 3

	for i := 0; i < n; i++ {
		goFile := fmt.Sprintf("%s/hdlr%d.go", path, i)
		file, err := os.Create(goFile)
		suite.failOnError(err, "can't create %s", goFile)
		err = codeTemplate.Execute(file, i)
		suite.failOnError(err, "can't write to %s", goFile)
		file.Close()
	}

	handlers, err := FindHandlers(path)
	suite.failOnError(err, "can't find handlers in %s", path)
	if !suite.Equal(len(handlers), n) {
		suite.FailNow("bad number of handlers")
	}
}

func TestParse(t *testing.T) {
	suite.Run(t, new(ParseSuite))
}
