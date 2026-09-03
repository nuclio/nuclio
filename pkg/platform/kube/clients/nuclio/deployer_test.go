//go:build test_unit

/*
Copyright 2026 The Nuclio Authors.

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

package nuclio

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"

	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DeployerTestSuite struct {
	suite.Suite
	deployer *Deployer
}

func (suite *DeployerTestSuite) SetupSuite() {
	suite.deployer = &Deployer{}
}

// TestPopulateFunctionKeepsStoredLabelsOnUpdate pins the behavior the migration label relies on: an update
// keeps the stored labels, so internal bookkeeping is never erased by a client request. Annotations are
// deliberately replaced.
func (suite *DeployerTestSuite) TestPopulateFunctionKeepsStoredLabelsOnUpdate() {
	for _, testCase := range []struct {
		name            string
		functionExisted bool
		expectedLabels  map[string]string
	}{
		{
			name:            "an update keeps the stored labels",
			functionExisted: true,
			expectedLabels: map[string]string{
				common.NuclioLabelKeyMigrationFunctionAuth: common.NuclioLabelValueMigrationApplied,
			},
		},
		{
			name:            "a create takes the request's labels",
			functionExisted: false,
			expectedLabels:  map[string]string{"user-label": "value"},
		},
	} {
		suite.Run(testCase.name, func() {
			functionInstance := &nuclioio.NuclioFunction{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						common.NuclioLabelKeyMigrationFunctionAuth: common.NuclioLabelValueMigrationApplied,
					},
				},
			}

			functionConfig := &functionconfig.Config{
				Meta: functionconfig.Meta{
					Name:      "func",
					Namespace: "namespace",

					// as a real client sends it: user labels only, no internal migration label
					Labels: map[string]string{"user-label": "value"},
				},
			}

			err := suite.deployer.populateFunction(functionConfig,
				&functionconfig.Status{State: functionconfig.FunctionStateWaitingForResourceConfiguration},
				functionInstance,
				testCase.functionExisted)
			suite.Require().NoError(err)
			suite.Require().Equal(testCase.expectedLabels, functionInstance.Labels)
		})
	}
}

func TestDeployerTestSuite(t *testing.T) {
	suite.Run(t, new(DeployerTestSuite))
}
