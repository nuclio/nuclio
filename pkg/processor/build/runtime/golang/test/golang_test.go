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

	"github.com/nuclio/nuclio/pkg/processor/build"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime/suite"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/suite"
)

type GolangBuildTestSuite struct {
	runtimesuite.RuntimeTestSuite
}

func (suite *GolangBuildTestSuite) TestBuildFile() {
	// suite.T().Skip()

	suite.BuildAndRunFunction("incrementor",
		path.Join(suite.getGolangRuntimeDir(), "test", "incrementor", "incrementor.go"),
		"",
		map[int]int{8080: 8080},
		8080,
		"abcdef",
		"bcdefg")
}

func (suite *GolangBuildTestSuite) TestBuildDir() {
	// suite.T().Skip()

	suite.BuildAndRunFunction("incrementor",
		path.Join(suite.getGolangRuntimeDir(), "test", "incrementor"),
		"",
		map[int]int{8080: 8080},
		8080,
		"abcdef",
		"bcdefg")
}

func (suite *GolangBuildTestSuite) TestBuildDirWithProcessorYAML() {
	// suite.T().Skip()

	suite.BuildAndRunFunction("incrementor",
		path.Join(suite.getGolangRuntimeDir(), "test", "incrementor-with-processor"),
		"",
		map[int]int{9999: 9999},
		9999,
		"abcdef",
		"bcdefg")
}

func (suite *GolangBuildTestSuite) TestBuildWithCompilationError() {
	// suite.T().Skip()

	var err error

	functionName := fmt.Sprintf("%s-%s", "compilationerror", suite.TestID)

	suite.Builder, err = build.NewBuilder(suite.Logger, &build.Options{
		FunctionName:    functionName,
		FunctionPath:    path.Join(suite.getGolangRuntimeDir(), "test", "compilation-error"),
		NuclioSourceDir: suite.GetNuclioSourceDir(),
		Verbose:         true,
	})

	suite.Require().NoError(err)

	// do the build
	err = suite.Builder.Build()
	suite.Require().Error(err)

	// error should yell about "fmt.NotAFunction" not existing
	suite.Require().Contains(errors.Cause(err).Error(), "fmt.NotAFunction")
}

func (suite *GolangBuildTestSuite) TestBuildURL() {
	// suite.T().Skip()

	// start an HTTP server to serve the reverser py
	// TODO: needs to be made unique (find a free port)
	httpServer := runtimesuite.HTTPFileServer{}
	httpServer.Start(":6666",
		path.Join(suite.getGolangRuntimeDir(), "test", "incrementor", "incrementor.go"),
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

func (suite *GolangBuildTestSuite) getGolangRuntimeDir() string {
	return path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "build", "runtime", "golang")
}

func TestGolangBuildTestSuite(t *testing.T) {
	suite.Run(t, new(GolangBuildTestSuite))
}
