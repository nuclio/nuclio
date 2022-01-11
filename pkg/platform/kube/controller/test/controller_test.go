//go:build test_integration && test_kube

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
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/test"

	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ControllerTestSuite struct {
	test.KubeTestSuite
}

func (suite *ControllerTestSuite) SetupSuite() {
	suite.KubeTestSuite.DisableControllerStart = true
	suite.KubeTestSuite.SetupSuite()
}

func (suite *ControllerTestSuite) TestStaleResourceVersion() {

	// build function
	function := suite.buildTestFunction()

	// creating function CRD record
	functionCRDRecord, err := suite.FunctionClientSet.
		NuclioV1beta1().
		NuclioFunctions(suite.Namespace).
		Create(suite.Ctx,
			&nuclioio.NuclioFunction{
				ObjectMeta: metav1.ObjectMeta{
					Name:        function.Meta.Name,
					Namespace:   function.Meta.Namespace,
					Labels:      function.Meta.Labels,
					Annotations: function.Meta.Annotations,
				},
				Spec: function.Spec,
				Status: functionconfig.Status{
					State: functionconfig.FunctionStateWaitingForResourceConfiguration,
				},
			},
			metav1.CreateOptions{})
	suite.Require().NoError(err)
	suite.Require().NotEmpty(functionCRDRecord.ResourceVersion)

	// ensure no resync interval (sanity)
	suite.Require().Equal(0, int(suite.Controller.GetResyncInterval()))

	// start controller
	err = suite.Controller.Start(suite.KubeTestSuite.Ctx)
	suite.Require().NoError(err)

	suite.WaitForFunctionState(&platform.GetFunctionsOptions{
		Namespace: functionCRDRecord.Namespace,
		Name:      functionCRDRecord.Name,
	}, functionconfig.FunctionStateReady, 5*time.Minute)
}

func (suite *ControllerTestSuite) buildTestFunction() *functionconfig.Config {

	// create function options
	createFunctionOptions := suite.CompileCreateFunctionOptions(fmt.Sprintf("test-%s", suite.TestID))

	// enrich with defaults
	err := suite.Platform.EnrichFunctionConfig(suite.KubeTestSuite.Ctx, &createFunctionOptions.FunctionConfig)
	suite.Require().NoError(err)

	// build function
	buildFunctionResults, err := suite.Platform.CreateFunctionBuild(&platform.CreateFunctionBuildOptions{
		Logger:              suite.Logger,
		FunctionConfig:      createFunctionOptions.FunctionConfig,
		PlatformName:        suite.Platform.GetName(),
		OnAfterConfigUpdate: nil,
	})
	suite.Require().NoError(err)
	suite.Require().NotEmpty(buildFunctionResults.Image)

	// update function's image
	buildFunctionResults.UpdatedFunctionConfig.Spec.Image = fmt.Sprintf("%s/%s",
		suite.RegistryURL,
		buildFunctionResults.Image)
	return &buildFunctionResults.UpdatedFunctionConfig
}

func TestControllerTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}
	suite.Run(t, new(ControllerTestSuite))
}
