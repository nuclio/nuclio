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

package functiondep

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform/kube/functioncr"
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/nuclio/nuclio-sdk"
	"github.com/stretchr/testify/suite"
	"k8s.io/api/core/v1"
)

type FunctiondepTestSuite struct {
	suite.Suite
	logger nuclio.Logger
	client *Client
}

func (suite *FunctiondepTestSuite) SetupTest() {
	var err error

	suite.logger, _ = nucliozap.NewNuclioZapTest("test")

	suite.client, err = NewClient(suite.logger, nil)
	suite.Require().NoError(err)
}

// TODO: test multi data binding (requires sorting stuff)
func (suite *FunctiondepTestSuite) TestGetEnv() {
	labels := map[string]string{
		"name":    "function_name",
		"version": "function_version",
	}

	functioncrInstance := &functioncr.Function{
		Spec: functionconfig.Spec{},
	}

	envs := suite.client.getFunctionEnvironment(labels, functioncrInstance)

	expected := []v1.EnvVar{
		{Name: "NUCLIO_FUNCTION_NAME", Value: "function_name"},
		{Name: "NUCLIO_FUNCTION_VERSION", Value: "function_version"},
	}

	suite.Require().Equal(expected, envs)
}

func TestFunctiondepTestSuite(t *testing.T) {
	suite.Run(t, new(FunctiondepTestSuite))
}
