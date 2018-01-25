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

package buildsuite

import (
	"path"
	"testing"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	httpsuite.TestSuite
}

func (suite *TestSuite) TestBuildInvalidFunctionPath() {
	var err error

	deployOptions := suite.GetDeployOptions("invalid", "invalidpath")

	_, err = suite.Platform.BuildFunction(&platform.BuildOptions{
		Logger:         deployOptions.Logger,
		FunctionConfig: deployOptions.FunctionConfig,
		PlatformName:   suite.Platform.GetName(),
	})

	suite.Require().Contains(errors.Cause(err).Error(), "invalidpath")
}

func (suite *TestSuite) TestBuildJessiePassesNonInteractiveFlag() {

	deployOptions := suite.GetDeployOptions("printer",
		path.Join(suite.GetNuclioSourceDir(), "test", "_functions", "python", "py2-printer"))

	deployOptions.FunctionConfig.Spec.Runtime = "python:2.7"
	deployOptions.FunctionConfig.Spec.Handler = "printer:handler"
	deployOptions.FunctionConfig.Spec.Build.BaseImageName = "jessie"

	deployOptions.FunctionConfig.Spec.Build.Commands = append(deployOptions.FunctionConfig.Spec.Build.Commands, "apt-get -qq update")
	deployOptions.FunctionConfig.Spec.Build.Commands = append(deployOptions.FunctionConfig.Spec.Build.Commands, "apt-get -qq install curl")

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestMethod:        "POST",
			RequestBody:          "",
			ExpectedResponseBody: "printed",
		})
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestSuite))
}
