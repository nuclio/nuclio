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
	"path"
	"testing"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/build"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime/test/suite"

	"github.com/stretchr/testify/suite"
	"bytes"
)

type TestSuite struct {
	buildsuite.TestSuite
}

func (suite *TestSuite) TestBuildFile() {
	// suite.T().Skip()

	suite.FunctionBuildRunAndRequest("incrementor",
		path.Join(suite.getGolangDir(), "incrementor", "incrementor.go"),
		"",
		map[int]int{8080: 8080},
		8080,
		"",
		nil,
		"abcdef",
		nil,
		"bcdefg",
		nil)
}

func (suite *TestSuite) TestBuildDir() {
	// suite.T().Skip()

	suite.FunctionBuildRunAndRequest("incrementor",
		path.Join(suite.getGolangDir(), "incrementor"),
		"",
		map[int]int{8080: 8080},
		8080,
		"",
		nil,
		"abcdef",
		nil,
		"bcdefg",
		nil)
}

func (suite *TestSuite) TestBuildDirWithProcessorYAML() {
	// suite.T().Skip()

	suite.FunctionBuildRunAndRequest("incrementor",
		path.Join(suite.getGolangDir(), "incrementor-with-processor"),
		"",
		map[int]int{9999: 9999},
		9999,
		"",
		nil,
		"abcdef",
		nil,
		"bcdefg",
		nil)
}

// until errors are fixed
func (suite *TestSuite) TestBuildWithCompilationError() {
	// suite.T().Skip()

	var err error

	functionName := fmt.Sprintf("%s-%s", "compilationerror", suite.TestID)

	suite.Builder, err = build.NewBuilder(suite.Logger, &build.Options{
		FunctionName:    functionName,
		FunctionPath:    path.Join(suite.getGolangDir(), "compilation-error"),
		NuclioSourceDir: suite.GetNuclioSourceDir(),
		Verbose:         true,
	})

	suite.Require().NoError(err)

	// do the build
	_, err = suite.Builder.Build()
	suite.Require().Error(err)

	buffer := bytes.Buffer{}

	// write an err stack
	errors.PrintErrorStack(&buffer, err, 10)

	// error should yell about "fmt.NotAFunction" not existing
	suite.Require().Contains(buffer.String(), "fmt.NotAFunction")
}

func (suite *TestSuite) TestBuildURL() {
	// suite.T().Skip()

	// start an HTTP server to serve the reverser py
	// TODO: needs to be made unique (find a free port)
	httpServer := buildsuite.HTTPFileServer{}
	httpServer.Start(":6666",
		path.Join(suite.getGolangDir(), "incrementor", "incrementor.go"),
		"/some/path/incrementor.go")

	defer httpServer.Shutdown(nil)

	suite.FunctionBuildRunAndRequest("incrementor",
		"http://localhost:6666/some/path/incrementor.go",
		"",
		map[int]int{8080: 8080},
		8080,
		"",
		nil,
		"abcdef",
		nil,
		"bcdefg",
		nil)
}

func (suite *TestSuite) getGolangDir() string {
	return path.Join(suite.GetProcessorBuildDir(), "golang", "test")
}

func TestTestSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}
