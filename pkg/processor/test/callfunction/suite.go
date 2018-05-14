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

package callfunction

import (
	"fmt"
	"path"

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"
)

// CallFunctionTestSuite tests calling functions from within a function using context.platform.call_function
// or equivalent
type CallFunctionTestSuite struct { // nolint
	HTTPSuite *httpsuite.TestSuite
}

// TestStructuredCloudEvent tests a structured cloud event
func (suite *CallFunctionTestSuite) TestCallFunction() {
	networkName := "test-network-" + suite.HTTPSuite.TestID

	// create a docker network so that the functions can communicate
	err := suite.HTTPSuite.DockerClient.CreateNetwork(&dockerclient.CreateNetworkOptions{
		Name: networkName,
	})
	suite.HTTPSuite.Require().NoError(err, "Failed to create docker network")

	defer suite.HTTPSuite.DockerClient.DeleteNetwork(networkName) // nolint: errcheck

	functionCallerPath := path.Join(suite.HTTPSuite.GetTestFunctionsDir(),
		"common",
		"function-caller",
		suite.HTTPSuite.RuntimeDir)

	calleeDeployOptions := suite.HTTPSuite.GetDeployOptions("callee", path.Join(functionCallerPath, "callee"))
	callerDeployOptions := suite.HTTPSuite.GetDeployOptions("caller", path.Join(functionCallerPath, "caller"))

	// set network of both functions to the same so that they can communicate
	calleeDeployOptions.FunctionConfig.Spec.Platform.Attributes = map[string]interface{}{"network": networkName}
	callerDeployOptions.FunctionConfig.Spec.Platform.Attributes = map[string]interface{}{"network": networkName}

	// deploy the callee function
	suite.HTTPSuite.DeployFunction(calleeDeployOptions, func(deployResult *platform.CreateFunctionResult) bool {

		// now deploy the caller function
		suite.HTTPSuite.DeployFunction(callerDeployOptions, func(deployResult *platform.CreateFunctionResult) bool {

			bodyVerifier := func(body []byte) {
				suite.HTTPSuite.Require().Equal(`{"from_callee": "returned_value"}`, string(body))
			}

			testRequest := httpsuite.Request{
				RequestBody:          fmt.Sprintf(`{"callee_name": "%s"}`, calleeDeployOptions.FunctionConfig.Meta.Name),
				RequestHeaders:       map[string]interface{}{"Content-Type": "application/json"},
				RequestMethod:        "POST",
				RequestPort:          deployResult.Port,
				ExpectedResponseBody: bodyVerifier,
			}

			return suite.HTTPSuite.SendRequestVerifyResponse(&testRequest)
		})

		return true
	})
}
