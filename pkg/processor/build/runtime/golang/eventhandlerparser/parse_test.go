//go:build test_unit

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

package eventhandlerparser

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"text/template"

	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

var code = `
package handler

import (
    "github.com/nuclio/nuclio-sdk-go"
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
    "github.com/nuclio/nuclio-sdk-go"
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
	zap, err := nucliozap.NewNuclioZapTest("parsereventhandler-test")
	suite.Require().NoError(err, "Can't create logger")
	suite.parser = NewEventHandlerParser(zap)
}

func (suite *ParseSuite) TestHandlerNames() {
	pkgs, handlers, err := suite.parseCode(code)
	suite.Require().NoErrorf(err, "Can't find handlers", "error: %s", err)
	suite.Require().Len(pkgs, 1)
	suite.Require().Len(handlers, 2)

	sort.Strings(handlers)
	suite.Require().Equal("AlsoHandler", handlers[0])
	suite.Require().Equal("Handler1", handlers[1])
}

func (suite *ParseSuite) TestBadCode() {
	_, _, err := suite.parseCode(badCode)
	suite.Require().Error(err, "No error on bad code")
}

func (suite *ParseSuite) TestFindHandlersInDirectory() {
	handlerDir, err := ioutil.TempDir("", "parse-test")
	suite.Require().NoError(err, "Can't create temporary directory")
	n := 3

	for i := 0; i < n; i++ {
		suite.createHandler(handlerDir, i)
	}

	pkgs, handlers, err := suite.parser.ParseEventHandlers(handlerDir)
	suite.Require().NoError(err, "Can't find handlers in %s", handlerDir)
	suite.Require().Equal(n, len(handlers))
	suite.Require().Equal(1, len(pkgs))
}

func (suite *ParseSuite) TestFindHandlersInFile() {
	handlerDir, err := ioutil.TempDir("", "parse-test")
	suite.Require().NoError(err, "Can't create temporary directory")

	handlerPath := suite.createHandler(handlerDir, 0)

	pkgs, handlers, err := suite.parser.ParseEventHandlers(handlerPath)
	suite.Require().NoError(err, "Can't find handlers in %s", handlerPath)
	suite.Require().Equal(1, len(handlers))
	suite.Require().Equal(1, len(pkgs))
}

func (suite *ParseSuite) parseCode(code string) ([]string, []string, error) {
	tmp, err := ioutil.TempDir("", "test-parse")
	suite.Require().NoError(err, "Can't create temporary directory file")
	defer os.RemoveAll(tmp)

	fileName := filepath.Join(tmp, "handler.go")

	file, err := os.Create(fileName)
	suite.Require().NoError(err, "Can't create %s", fileName)
	fmt.Fprint(file, code)
	err = file.Close()
	suite.Require().NoError(err, "Can't sync")
	return suite.parser.ParseEventHandlers(tmp)
}

func (suite *ParseSuite) createHandler(handlerDir string, index int) string {
	handlerPath := fmt.Sprintf("%s/hdlr%d.go", handlerDir, index)

	file, err := os.Create(handlerPath)
	suite.Require().NoError(err, "Can't create %s", handlerPath)
	err = codeTemplate.Execute(file, index)
	suite.Require().NoError(err, "Can't write to %s", handlerPath)
	file.Close()

	return handlerPath
}

func TestParse(t *testing.T) {
	suite.Run(t, new(ParseSuite))
}
