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
	"testing"
	"path"
	// "fmt"

	// "github.com/nuclio/nuclio/pkg/processor/build"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime/suite"

	"github.com/stretchr/testify/suite"
	// "github.com/pkg/errors"
)

type PythonBuildTestSuite struct {
	runtimesuite.RuntimeTestSuite
}

func (suite *PythonBuildTestSuite) TestBuildFile() {
	// suite.T().Skip()

	suite.BuildAndRunFunction("reverser",
		path.Join(suite.getPythonRuntimeDir(), "test", "reverser", "reverser.py"),
		"",
		map[int]int{8080: 8080},
		8080,
		"abcdef",
		"fedcba")
}

func (suite *PythonBuildTestSuite) TestBuildDir() {
	// suite.T().Skip()

	suite.BuildAndRunFunction("reverser",
		path.Join(suite.getPythonRuntimeDir(), "test", "reverser"),
		"python",
		map[int]int{8080: 8080},
		8080,
		"abcdef",
		"fedcba")
}

func (suite *PythonBuildTestSuite) TestBuildDirWithProcessorYAML() {
	// suite.T().Skip()

	suite.BuildAndRunFunction("reverser",
		path.Join(suite.getPythonRuntimeDir(), "test", "reverser-with-processor"),
			"python",
		map[int]int{8888: 8888},
		8888,
		"abcdef",
		"fedcba")
}

func (suite *PythonBuildTestSuite) getPythonRuntimeDir() string {
	return path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "build", "runtime", "python")
}

func TestPythonBuildTestSuite(t *testing.T) {
	suite.Run(t, new(PythonBuildTestSuite))
}
