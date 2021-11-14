// +build test_integration
// +build test_kube

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
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/test"

	"github.com/stretchr/testify/suite"
	v1 "k8s.io/api/core/v1"
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
	function, _ := suite.buildTestFunction(false)

	// creating function CRD record
	functionCRDRecord, err := suite.FunctionClientSet.
		NuclioV1beta1().
		NuclioFunctions(suite.Namespace).
		Create(&nuclioio.NuclioFunction{
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
		})
	suite.Require().NoError(err)
	suite.Require().NotEmpty(functionCRDRecord.ResourceVersion)

	// ensure no resync interval (sanity)
	suite.Require().Equal(0, int(suite.Controller.GetResyncInterval()))

	// start controller
	err = suite.Controller.Start()
	suite.Require().NoError(err)

	suite.WaitForFunctionState(&platform.GetFunctionsOptions{
		Namespace: functionCRDRecord.Namespace,
		Name:      functionCRDRecord.Name,
	}, functionconfig.FunctionStateReady, 5*time.Minute)
}

func (suite *ControllerTestSuite) TestFunctionRedeploymentFailureInvocationURLs() {

	// build function
	functionConfig, createFunctionOptions := suite.buildTestFunction(true)

	// create function CRD record
	functionCRDRecord, err := suite.FunctionClientSet.
		NuclioV1beta1().
		NuclioFunctions(suite.Namespace).
		Create(&nuclioio.NuclioFunction{
			ObjectMeta: metav1.ObjectMeta{
				Name:            functionConfig.Meta.Name,
				Namespace:       functionConfig.Meta.Namespace,
				Labels:          functionConfig.Meta.Labels,
				Annotations:     functionConfig.Meta.Annotations,
			},
			Spec: functionConfig.Spec,
			Status: functionconfig.Status{
				State: functionconfig.FunctionStateWaitingForResourceConfiguration,
			},
		})
	suite.Require().NoError(err)
	suite.Require().NotEmpty(functionCRDRecord.ResourceVersion)
	suite.Logger.DebugWith("Function CRD created", "resourceVersion", functionCRDRecord.ResourceVersion)

	// start controller
	err = suite.Controller.Start()
	suite.Require().NoError(err)

	// wait for function to be ready
	suite.WaitForFunctionState(&platform.GetFunctionsOptions{
		Namespace: functionCRDRecord.Namespace,
		Name:      functionCRDRecord.Name,
	}, functionconfig.FunctionStateReady, 5*time.Minute)

	// update function with errors
	suite.Logger.Debug("Updating function`s source code with typos")

	badFunctionConfig := suite.buildErroneousTestFunction(createFunctionOptions)

	// Get functions
	function, err := suite.FunctionClientSet.NuclioV1beta1().
		NuclioFunctions(functionConfig.Meta.Namespace).
		Get(functionConfig.Meta.Name, metav1.GetOptions{})
	suite.Require().NoError(err)

	// Update spec and status
	function.Spec.Image = badFunctionConfig.Spec.Image
	function.Spec.ImageHash = badFunctionConfig.Spec.ImageHash
	function.Status.State = functionconfig.FunctionStateWaitingForResourceConfiguration

	newFunctionCRDRecord, err := suite.FunctionClientSet.
		NuclioV1beta1().
		NuclioFunctions(suite.Namespace).
		Update(function)
	suite.Require().NoError(err)

	// redeploy function
	newFunctionOptions := &platform.GetFunctionsOptions{
		Namespace: newFunctionCRDRecord.Namespace,
		Name:      newFunctionCRDRecord.Name,
	}

	// wait for function to become unhealthy (the state might be Building / Error / Ready)
	suite.WaitForFunctionState(newFunctionOptions, functionconfig.FunctionStateUnhealthy, 2*time.Minute)

	// check function's invocation urls
	newFunction := suite.GetFunction(newFunctionOptions)
	internalInvocationURLs := newFunction.GetStatus().InternalInvocationURLs
	externalInvocationURLs := newFunction.GetStatus().ExternalInvocationURLs
	newFuncSourceCode := newFunction.GetConfig().Spec.Build.FunctionSourceCode

	suite.Logger.DebugWith("Got function invocation URLs", "internalInvocationURLs",
		internalInvocationURLs, "externalInvocationURLs", externalInvocationURLs,
		"sourceCode", newFuncSourceCode, "Status", newFunction.GetStatus())
}

func (suite *ControllerTestSuite) buildTestFunction(httpTrigger bool) (*functionconfig.Config, *platform.CreateFunctionOptions) {

	// create function options
	createFunctionOptions := suite.CompileCreateFunctionOptions(fmt.Sprintf("test-%s", suite.TestID))

	if httpTrigger {
		if createFunctionOptions.FunctionConfig.Spec.Triggers == nil {
			createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{}
		}
		defaultHTTPTrigger := functionconfig.GetDefaultHTTPTrigger()
		createFunctionOptions.FunctionConfig.Spec.Triggers[defaultHTTPTrigger.Name] = defaultHTTPTrigger

		createFunctionOptions.FunctionConfig.Spec.ServiceType = v1.ServiceTypeNodePort
	}

	// enrich with defaults
	err := suite.Platform.EnrichFunctionConfig(&createFunctionOptions.FunctionConfig)
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
	return &buildFunctionResults.UpdatedFunctionConfig, createFunctionOptions
}

func (suite *ControllerTestSuite) buildErroneousTestFunction(funcOptions *platform.CreateFunctionOptions) *functionconfig.Config {

	// update function with an erroneous source code (update the CRD)
	funcOptions.FunctionConfig.Spec.Build.FunctionSourceCode = base64.StdEncoding.EncodeToString([]byte(`
def handler(context, event):
  retur "hello world'q
`))

	// build function
	newFunction, err := suite.Platform.CreateFunctionBuild(&platform.CreateFunctionBuildOptions{
		Logger:              suite.Logger,
		FunctionConfig:      funcOptions.FunctionConfig,
		PlatformName:        suite.Platform.GetName(),
		OnAfterConfigUpdate: nil,
	})
	suite.Require().NoError(err)
	suite.Require().NotEmpty(newFunction.Image)

	// update function's image
	newFunction.UpdatedFunctionConfig.Spec.Image = fmt.Sprintf("%s/%s",
		suite.RegistryURL,
		newFunction.Image)

	return &newFunction.UpdatedFunctionConfig
}

func TestControllerTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}
	suite.Run(t, new(ControllerTestSuite))
}
