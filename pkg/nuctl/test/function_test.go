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
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/version"

	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
)

type FunctionTestSuite struct {
	Suite
}

func (suite *FunctionTestSuite) SetupSuite() {
	suite.Suite.SetupSuite()

	// Update version so that linker doesn't need to inject it
	version.Set(&version.Info{
		GitCommit: "c",
		Label:     "latest",
		Arch:      "amd64",
		OS:        "linux",
	})
}

func (suite *FunctionTestSuite) TestDeploy() {
	imageName := fmt.Sprintf("nuclio/deploy-test-%s", xid.New().String())

	namedArgs := map[string]string{
		"path":  path.Join(suite.GetFunctionsDir(), "common", "json-parser-with-function-config", "golang"),
		"image": imageName,
	}

	err := suite.ExecuteNutcl([]string{"deploy", "json-parser-with-function-config", "--verbose", "--no-pull"}, namedArgs)

	suite.Require().NoError(err)

	// Make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName)

	// Use nutctl to delete the function when we're done
	defer suite.ExecuteNutcl([]string{"delete", "fu", "json-parser-with-function-config"}, nil)

	// Try a few times to invoke, until it succeeds
	err = common.RetryUntilSuccessful(60*time.Second, 1*time.Second, func() bool {

		// Invoke the function
		err = suite.ExecuteNutcl([]string{"invoke", "json-parser-with-function-config"},
			map[string]string{
				"method": "POST",
				"body":   `{"first_return": "some Return", "return_this": "my Value"}`,
			})

		return err == nil
	})

	suite.Require().NoError(err)

	// Make sure reverser worked
	suite.Require().Contains(suite.outputBuffer.String(), "my Value")
}

func TestFunctionTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(FunctionTestSuite))
}
