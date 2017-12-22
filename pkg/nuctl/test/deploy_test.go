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

type DeployTestSuite struct {
	Suite
}

func (suite *DeployTestSuite) SetupSuite() {
	suite.Suite.SetupSuite()

	// update version so that linker doesn't need to inject it
	version.Set(&version.Info{
		GitCommit: "c",
		Label:     "latest",
		Arch:      "amd64",
		OS:        "linux",
	})
}

func (suite *DeployTestSuite) TestDeploy() {
	imageName := fmt.Sprintf("nuclio/deploy-test-%s", xid.New().String())

	namedArgs := map[string]string{
		"path":    path.Join(suite.GetFunctionsDir(), "common", "reverser", "golang"),
		"image":   imageName,
		"runtime": "golang",
		"handler": "main:Reverse",
	}

	err := suite.ExecuteNutcl([]string{"deploy", "reverser", "--verbose", "--no-pull"}, namedArgs)

	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName)

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNutcl([]string{"delete", "fu", "reverser"}, nil)

	// try a few times to invoke, until it succeeds
	err = common.RetryUntilSuccessful(60*time.Second, 1*time.Second, func() bool {

		// invoke the function
		err = suite.ExecuteNutcl([]string{"invoke", "reverser"},
			map[string]string{
				"method": "POST",
				"body":   "-reverse this string+",
			})

		return err == nil
	})

	suite.Require().NoError(err)

	// make sure reverser worked
	suite.Require().Contains(suite.outputBuffer.String(), "+gnirts siht esrever-")
}

func (suite *DeployTestSuite) TestDeployWithMetadata() {
	imageName := fmt.Sprintf("nuclio/deploy-test-%s", xid.New().String())

	err := suite.ExecuteNutcl([]string{"deploy", "env", "--verbose", "--no-pull"},
		map[string]string{
			"path":    path.Join(suite.GetFunctionsDir(), "common", "envprinter", "python"),
			"env":     "FIRST_ENV=11223344,SECOND_ENV=0099887766",
			"labels":  "label1=first,label2=second",
			"runtime": "python",
			"handler": "envprinter:handler",
		})

	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName)

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNutcl([]string{"delete", "fu", "env"}, nil)

	// try a few times to invoke, until it succeeds
	err = common.RetryUntilSuccessful(60*time.Second, 1*time.Second, func() bool {

		// invoke the function
		err = suite.ExecuteNutcl([]string{"invoke", "env"},
			map[string]string{
				"method": "POST",
				"body":   "-reverse this string+",
			})

		return err == nil
	})

	suite.Require().NoError(err)

	// make sure reverser worked
	suite.Require().Contains(suite.outputBuffer.String(), "11223344")
	suite.Require().Contains(suite.outputBuffer.String(), "0099887766")
}

func (suite *DeployTestSuite) TestDeployFailsOnMissingPath() {
	imageName := fmt.Sprintf("nuclio/deploy-test-%s", xid.New().String())

	err := suite.ExecuteNutcl([]string{"deploy", "reverser", "--verbose", "--no-pull"},
		map[string]string{
			"image":   imageName,
			"runtime": "golang",
			"handler": "main:Reverse",
		})

	suite.Require().Error(err, "Function code must be provided either in the path or inline in a spec file; alternatively, an image or handler may be provided")
}

func (suite *DeployTestSuite) TestDeployFailsOnShellMissingPathAndHandler() {
	imageName := fmt.Sprintf("nuclio/deploy-test-%s", xid.New().String())

	err := suite.ExecuteNutcl([]string{"deploy", "reverser", "--verbose", "--no-pull"},
		map[string]string{
			"image":   imageName,
			"runtime": "shell",
		})

	suite.Require().Error(err, "Function code must be provided either in the path or inline in a spec file; alternatively, an image or handler may be provided")
}

func (suite *DeployTestSuite) TestDeployShellViaHandler() {
	imageName := fmt.Sprintf("nuclio/deploy-test-%s", xid.New().String())

	err := suite.ExecuteNutcl([]string{"deploy", "reverser", "--verbose", "--no-pull"},
		map[string]string{
			"image":   imageName,
			"runtime": "shell",
			"handler": "rev",
		})

	suite.Require().NoError(err)

	// try a few times to invoke, until it succeeds
	err = common.RetryUntilSuccessful(60*time.Second, 1*time.Second, func() bool {

		err = suite.ExecuteNutcl([]string{"invoke", "reverser"},
			map[string]string{
				"method": "POST",
				"body":   "-reverse this string+",
			})

		return err == nil
	})

	suite.Require().NoError(err)

	// make sure reverser worked
	suite.Require().Contains(suite.outputBuffer.String(), "+gnirts siht esrever-")
}

func TestDeployTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(DeployTestSuite))
}
