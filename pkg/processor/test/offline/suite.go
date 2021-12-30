//go:build test_integration && test_local

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

package offline

import (
	"os"
	"path"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"
)

// TestSuite has common functions for offline testings
// Other runtime suites may embed this suite and in their SetupSuite while setting HTTPSuite and FunctionHandler
type TestSuite struct {
	HTTPSuite *httpsuite.TestSuite

	// handler may vary from runtime to another
	FunctionHandler string

	// env mutating
	oldNuclioDockerBuildNetworkValue string
}

func (suite *TestSuite) SetupTest() {

	// since we build offline, force docker build command to run with --network none
	suite.oldNuclioDockerBuildNetworkValue = os.Getenv("NUCLIO_DOCKER_BUILD_NETWORK")

	// run docker commands with no internet access
	err := os.Setenv("NUCLIO_DOCKER_BUILD_NETWORK", "none")
	suite.HTTPSuite.Require().NoError(err)
}

func (suite *TestSuite) TearDownTest() {

	// restore value
	err := os.Setenv("NUCLIO_DOCKER_BUILD_NETWORK", suite.oldNuclioDockerBuildNetworkValue)
	suite.HTTPSuite.Require().NoError(err)
}

func (suite *TestSuite) TestOffline() {
	createFunctionOptions := suite.getFunctionCreateOptions()
	suite.HTTPSuite.DeployFunctionAndRequest(createFunctionOptions, &httpsuite.Request{
		RequestBody:          "abcd",
		RequestMethod:        "POST",
		ExpectedResponseBody: "dcba",
	})
}

func (suite *TestSuite) getFunctionCreateOptions() *platform.CreateFunctionOptions {
	options := suite.HTTPSuite.GetDeployOptions(
		"reverser",
		path.Join(
			suite.HTTPSuite.GetTestFunctionsDir(),
			"common",
			"reverser",
			suite.HTTPSuite.GetRuntimeDir(),
		),
	)

	// assume offline, no images pull, etc
	options.FunctionConfig.Spec.Build.Offline = true

	// force rebuild
	options.FunctionConfig.Spec.Build.NoCache = true

	if suite.FunctionHandler != "" {
		options.FunctionConfig.Spec.Handler = suite.FunctionHandler
	}

	return options
}
