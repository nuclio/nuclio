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

package readinessTimeout

import (
	"path"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

	"github.com/stretchr/testify/suite"
)

type readinessTimeoutTestSuite struct { // nolint
	httpsuite.TestSuite
}

// Deploys a failing Python function. Expect the function to fail after 30 seconds
func (suite *readinessTimeoutTestSuite) TestPythonNoReadinessTimeout() {

	beforeTime := time.Now()
	suite.deployFailingPythonFunction(0)

	// the default timeout is 30 seconds - it must have timed out after that
	suite.Require().True(time.Since(beforeTime) >= 30*time.Second)
}

// Deploys a failing Python function. Expect the function to fail after 10 seconds
func (suite *readinessTimeoutTestSuite) TestPythonSpecifiedReadinessTimeout() {

	beforeTime := time.Now()
	suite.deployFailingPythonFunction(10)

	// the default timeout is 30 seconds - it must have timed out after that
	suite.Require().True(time.Since(beforeTime) <= 29*time.Second)
}

func (suite *readinessTimeoutTestSuite) deployFailingPythonFunction(readinessTimeoutSeconds int) {
	createFunctionOptions := suite.GetDeployOptions("reverser",
		path.Join(suite.GetTestFunctionsDir(), "common", "reverser", "python"))

	// configure the function to connect to some invalid kafka - it will fail after coming up and never
	// reach healthy
	createFunctionOptions.FunctionConfig.Spec.ReadinessTimeoutSeconds = readinessTimeoutSeconds
	createFunctionOptions.FunctionConfig.Spec.Runtime = "python:3.6"
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		"badkafka": {
			Kind: "kafka",
			URL:  "127.0.0.1:9999",
		},
	}

	// add some commonly used options to createFunctionOptions
	suite.PopulateDeployOptions(createFunctionOptions)

	// deploy the function - it's OK for it to time out
	_, err := suite.Platform.CreateFunction(createFunctionOptions)
	suite.Require().Error(err)

	// delete the function when done
	defer suite.Platform.DeleteFunction(&platform.DeleteFunctionOptions{ // nolint: errcheck
		FunctionConfig: createFunctionOptions.FunctionConfig,
	})
}

func TestOfflineSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(readinessTimeoutTestSuite))
}
