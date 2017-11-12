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

	"github.com/nuclio/nuclio/pkg/version"

	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
)

type BuildTestSuite struct {
	Suite
}

func (suite *BuildTestSuite) SetupSuite() {
	suite.Suite.SetupSuite()

	// update version so that linker doesn't need to inject it
	version.Set(&version.Info{
		GitCommit: "c",
		Label:     "latest",
		Arch:      "amd64",
		OS:        "linux",
	})
}

func (suite *BuildTestSuite) TestBuild() {
	imageName := fmt.Sprintf("nuclio/build-test-%s", xid.New().String())

	err := suite.ExecuteNutcl([]string{"build", "example", "--verbose", "--no-pull"},
		map[string]string{
			"path":           path.Join(suite.GetNuclioSourceDir(), "pkg", "nuctl", "test", "_reverser"),
			"nuclio-src-dir": suite.GetNuclioSourceDir(),
			"image":          imageName,
		})

	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName)

	// use deploy with the image we just created
	err = suite.ExecuteNutcl([]string{"deploy", "example", "--verbose"},
		map[string]string{
			"run-image": imageName,
			"runtime":   "golang",
			"handler":   "main:Reverse",
		})

	suite.Require().NoError(err)

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNutcl([]string{"delete", "fu", "example"}, nil)

	// invoke the function
	err = suite.ExecuteNutcl([]string{"invoke", "example"},
		map[string]string{
			"method": "POST",
			"body":   "-reverse this string+",
		})

	suite.Require().NoError(err)

	// make sure reverser worked
	suite.Require().Contains(suite.outputBuffer.String(), "+gnirts siht esrever-")
}

func TestBuildTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(BuildTestSuite))
}
