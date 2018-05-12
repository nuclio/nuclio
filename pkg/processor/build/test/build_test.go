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
	"encoding/base64"
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

func (suite *TestSuite) TestBuildFuncFromSourceWithInlineConfig() {
	createFunctionOptions := &platform.CreateFunctionOptions{
		Logger: suite.Logger,
	}

	functionSourceCode := `
# @nuclio.configure
#
# function.yaml:
#   metadata:
#     name: echo-foo-inline
#     namespace: default
#   spec:
#     env:
#     - name: MESSAGE
#       value: foo

echo $MESSAGE`

	createFunctionOptions.FunctionConfig.Spec.Handler = "echo-foo-inline.sh"
	createFunctionOptions.FunctionConfig.Spec.Runtime = "shell"
	createFunctionOptions.FunctionConfig.Spec.Build.Path = ""
	createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode = base64.StdEncoding.EncodeToString(
		[]byte(functionSourceCode))

	suite.DeployFunctionAndRequest(createFunctionOptions,
		&httpsuite.Request{
			RequestMethod:        "POST",
			RequestBody:          "",
			ExpectedResponseBody: "foo\n",
		})
}

func (suite *TestSuite) TestBuildInvalidFunctionPath() {
	var err error

	createFunctionOptions := suite.GetDeployOptions("invalid", "invalidpath")

	_, err = suite.Platform.CreateFunctionBuild(&platform.CreateFunctionBuildOptions{
		Logger:         createFunctionOptions.Logger,
		FunctionConfig: createFunctionOptions.FunctionConfig,
		PlatformName:   suite.Platform.GetName(),
	})

	suite.Require().Contains(errors.Cause(err).Error(), "invalidpath")
}

func (suite *TestSuite) TestBuildJessiePassesNonInteractiveFlag() {

	createFunctionOptions := suite.GetDeployOptions("printer",
		path.Join(suite.GetNuclioSourceDir(), "test", "_functions", "python", "py2-printer"))

	createFunctionOptions.FunctionConfig.Spec.Runtime = "python:2.7"
	createFunctionOptions.FunctionConfig.Spec.Handler = "printer:handler"
	createFunctionOptions.FunctionConfig.Spec.Build.BaseImage = "jessie"

	createFunctionOptions.FunctionConfig.Spec.Build.Commands = append(createFunctionOptions.FunctionConfig.Spec.Build.Commands, "apt-get -qq update")
	createFunctionOptions.FunctionConfig.Spec.Build.Commands = append(createFunctionOptions.FunctionConfig.Spec.Build.Commands, "apt-get -qq install curl")

	suite.DeployFunctionAndRequest(createFunctionOptions,
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
