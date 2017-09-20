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

package golang

import (
	"fmt"
	"path"
	"testing"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/build"
	"github.com/nuclio/nuclio/pkg/processor/test/suite"

	"github.com/stretchr/testify/suite"
	"bytes"
)

type GolangBuildTestSuite struct {
	processorsuite.ProcessorTestSuite
}

func (suite *GolangBuildTestSuite) TestBuildFile() {
	// suite.T().Skip()

	suite.BuildAndRunFunction("incrementor",
		path.Join(suite.getProcessorTestGolangDir(), "incrementor", "incrementor.go"),
		"",
		map[int]int{8080: 8080},
		8080,
		"abcdef",
		"bcdefg")
}

func (suite *GolangBuildTestSuite) TestBuildDir() {
	// suite.T().Skip()

	suite.BuildAndRunFunction("incrementor",
		path.Join(suite.getProcessorTestGolangDir(), "incrementor"),
		"",
		map[int]int{8080: 8080},
		8080,
		"abcdef",
		"bcdefg")
}

func (suite *GolangBuildTestSuite) TestBuildDirWithProcessorYAML() {
	// suite.T().Skip()

	suite.BuildAndRunFunction("incrementor",
		path.Join(suite.getProcessorTestGolangDir(), "incrementor-with-processor"),
		"",
		map[int]int{9999: 9999},
		9999,
		"abcdef",
		"bcdefg")
}

// until errors are fixed
func (suite *GolangBuildTestSuite) TestBuildWithCompilationError() {
	// suite.T().Skip()

	var err error

	functionName := fmt.Sprintf("%s-%s", "compilationerror", suite.TestID)

	suite.Builder, err = build.NewBuilder(suite.Logger, &build.Options{
		FunctionName:    functionName,
		FunctionPath:    path.Join(suite.getProcessorTestGolangDir(), "compilation-error"),
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

func (suite *GolangBuildTestSuite) TestBuildURL() {
	// suite.T().Skip()

	// start an HTTP server to serve the reverser py
	// TODO: needs to be made unique (find a free port)
	httpServer := processorsuite.HTTPFileServer{}
	httpServer.Start(":6666",
		path.Join(suite.getProcessorTestGolangDir(), "incrementor", "incrementor.go"),
		"/some/path/incrementor.go")

	defer httpServer.Shutdown(nil)

	suite.BuildAndRunFunction("incrementor",
		"http://localhost:6666/some/path/incrementor.go",
		"",
		map[int]int{8080: 8080},
		8080,
		"abcdef",
		"bcdefg")
}

func (suite *GolangBuildTestSuite) getProcessorTestGolangDir() string {
	return path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "test", "golang")
}

func TestGolangBuildTestSuite(t *testing.T) {
	suite.Run(t, new(GolangBuildTestSuite))
}
