package eventhandlerparser

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"text/template"

	"github.com/stretchr/testify/suite"

	nucliozap "github.com/nuclio/nuclio/pkg/zap"
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

	parser *EventHandlerParser
}

func (suite *ParseSuite) SetupSuite() {
	var level nucliozap.Level

	if testing.Verbose() {
		level = nucliozap.DebugLevel
	} else {
		level = nucliozap.InfoLevel
	}

	zap, err := nucliozap.NewNuclioZap("parsereventhandler-test", level)
	suite.failOnError(err, "Can't craete logger")
	suite.parser = NewEventHandlerParser(zap)
}

func (suite *ParseSuite) failOnError(err error, format string, args ...interface{}) {
	if err != nil {
		msg := fmt.Sprintf(format, args...)
		suite.FailNow(err.Error(), msg)
	}
}

func (suite *ParseSuite) parseCode(code string) ([]string, []string, error) {
	tmp, err := ioutil.TempDir("", "test-parse")
	suite.failOnError(err, "Can't create temp directory file")
	defer os.RemoveAll(tmp)

	fileName := filepath.Join(tmp, "handler.go")

	file, err := os.Create(fileName)
	suite.failOnError(err, "Can't create %s", fileName)
	fmt.Fprint(file, code)
	err = file.Close()
	suite.failOnError(err, "Can't sync")
	return suite.parser.ParseEventHandlers(tmp)
}

func (suite *ParseSuite) TestHandlerNames() {
	pkgs, handlers, err := suite.parseCode(code)
	if err != nil {
		suite.FailNow("Can't find handlers", "error: %s", err)
	}
	if !suite.Len(pkgs, 1, "Bad length") {
		suite.FailNow("")
	}
	if !suite.Len(handlers, 2, "Bad length") {
		suite.FailNow("")
	}
	sort.Strings(handlers)
	fmt.Println(handlers)
	if !suite.Equal(handlers[0], "AlsoHandler", "First handler") {
		suite.FailNow("")
	}
	if !suite.Equal(handlers[1], "Handler1", "Second handler") {
		suite.FailNow("")
	}
}

func (suite *ParseSuite) TestBadCode() {
	_, _, err := suite.parseCode(badCode)
	if err == nil {
		suite.FailNow("No error on bad code")
	}
}

func (suite *ParseSuite) TestFindHandlers() {
	path, err := ioutil.TempDir("", "parse-test")
	suite.failOnError(err, "Can't create temp directory")
	n := 3

	for i := 0; i < n; i++ {
		goFile := fmt.Sprintf("%s/hdlr%d.go", path, i)
		file, err := os.Create(goFile)
		suite.failOnError(err, "Can't create %s", goFile)
		err = codeTemplate.Execute(file, i)
		suite.failOnError(err, "Can't write to %s", goFile)
		file.Close()
	}

	pkgs, handlers, err := suite.parser.ParseEventHandlers(path)
	suite.failOnError(err, "Can't find handlers in %s", path)
	if !suite.Equal(len(handlers), n) {
		suite.FailNow("Bad number of handlers")
	}
	if !suite.Equal(len(pkgs), 1) {
		suite.FailNow("Bad number of packages")
	}
}

func TestParse(t *testing.T) {
	suite.Run(t, new(ParseSuite))
}
