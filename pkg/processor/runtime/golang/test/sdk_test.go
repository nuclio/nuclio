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

package test

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/test/suite"
)

type TestSuite struct {
	suite.NuclioTestSuite
}

func (suite *TestSuite) TestGenErrors() {
	dirPath, err := ioutil.TempDir("", "gen-errors-test")
	suite.Require().NoError(err)

	suite.Logger.DebugWith("Temp directory created", "path", dirPath)

	runOptions := &cmdrunner.RunOptions{WorkingDir: &dirPath}
	goFilePath := fmt.Sprintf("%s/vendor/github.com/nuclio/nuclio-sdk/gen_errors.go", suite.NuclioRootPath)

	_, err = suite.Cmd.Run(runOptions, "go run %s", goFilePath)
	suite.Require().NoError(err)

	// Make sure file exists
	outFilePath := fmt.Sprintf("%s/errors.go", dirPath)

	goCode, err := ioutil.ReadFile(outFilePath)
	suite.Require().NoError(err, "errors.go not created")

	// Make sure file is valid
	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, outFilePath, string(goCode), parser.AllErrors)
	suite.Require().NoError(err, "Not a valid Go file")
}

func (suite *TestSuite) TestErrors() {
	message := "some message"
	err := nuclio.NewErrBadGateway(message)
	suite.Require().Equal(message, err.Error())

	wsErr, ok := err.(nuclio.WithStatusCode)
	suite.Require().True(ok)
	suite.Require().Equal(http.StatusBadGateway, wsErr.StatusCode())
}

func TestSDK(t *testing.T) {
	suite.Run(t, new(TestSuite))
}
