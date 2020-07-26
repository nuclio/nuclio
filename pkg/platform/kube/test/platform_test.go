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
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	nuclioioclient "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	"github.com/nuclio/nuclio/pkg/platform/kube/controller"
	"github.com/nuclio/nuclio/pkg/platform/kube/functionres"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	processorsuite "github.com/nuclio/nuclio/pkg/processor/test/suite"

	"github.com/stretchr/testify/suite"
	"k8s.io/client-go/kubernetes"
)

type TestSuite struct {
	processorsuite.TestSuite
	cmdRunner   cmdrunner.CmdRunner
	registryURL string
}

func (suite *TestSuite) SetupSuite() {
	var err error
	suite.Namespace = common.GetEnvOrDefaultString("NUCLIO_TEST_NAMESPACE", "default")
	suite.PlatformType = "kube"
	suite.PlatformConfiguration = &platformconfig.Config{
		Kind: suite.PlatformType,
	}
	suite.TestSuite.SetupSuite()

	// TODO: ensure crd are installed
	// helm install
	//	--set controller.enabled=false
	//	--set dashboard.enabled=false
	//	--set autoscaler.enabled=false
	//	--set dlx.enabled=false
	//	--set rbac.create=false
	//	--set crd.create=true
	//	--debug
	//	--wait
	//	--namespace nuclio nuclio hack/k8s/helm/nuclio/
	//suite.cmdRunner.Run(nil, "docker run --name nuclio-kube-test-registry --detach --publish 5000:5000 registry:2")

	// TODO: run registry
	suite.cmdRunner, err = cmdrunner.NewShellRunner(suite.Logger)
	suite.Require().NoError(err, "Failed to create shell runner")

	suite.registryURL = common.GetEnvOrDefaultString("NUCLIO_TEST_REGISTRY_URL", "localhost:5000")

	// do not rename function name
	suite.FunctionNameUniquify = false

	// start controller in background
	suite.createAndStartController()
}

func (suite *TestSuite) TestDeployWithStaleResourceVersion() {
	var resourceVersion string

	functionSourceCodeFirst := base64.StdEncoding.EncodeToString([]byte(`
def handler(context, event): return "first"`))
	functionSourceCodeSecond := base64.StdEncoding.EncodeToString([]byte(`
def handler(context, event): return "second"`))

	createFunctionOptions := suite.compileCreateFunctionOptions("resource-schema")

	createFunctionOptions.FunctionConfig.Spec.Handler = "main:handler"
	createFunctionOptions.FunctionConfig.Spec.Runtime = "python:3.6"
	createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode = functionSourceCodeFirst

	afterFirstDeploy := func(deployResult *platform.CreateFunctionResult) bool {

		// get the function
		functions, err := suite.Platform.GetFunctions(&platform.GetFunctionsOptions{
			Name:      createFunctionOptions.FunctionConfig.Meta.Name,
			Namespace: createFunctionOptions.FunctionConfig.Meta.Namespace,
		})
		suite.Require().NoError(err)
		function := functions[0]

		// save resource version
		resourceVersion = function.GetConfig().Meta.ResourceVersion
		suite.Require().NotEmpty(resourceVersion)

		// ensure using newest resource version on second deploy
		createFunctionOptions.FunctionConfig.Meta.ResourceVersion = resourceVersion

		// change source code
		createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode = functionSourceCodeSecond
		return true
	}

	afterSecondDeploy := func(deployResult *platform.CreateFunctionResult) bool {

		// get the function
		functions, err := suite.Platform.GetFunctions(&platform.GetFunctionsOptions{
			Name:      createFunctionOptions.FunctionConfig.Meta.Name,
			Namespace: createFunctionOptions.FunctionConfig.Meta.Namespace,
		})
		suite.Require().NoError(err, "Failed to get functions")

		function := functions[0]
		suite.Require().NotEqual(resourceVersion,
			function.GetConfig().Meta.ResourceVersion,
			"Resource version should be changed between deployments")

		// we expect a failure due to a stale resource version
		suite.DeployFunctionExpectError(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
			suite.Require().Nil(deployResult, "Deployment results is nil when creation failed")
			return true
		})

		return true
	}

	suite.DeployFunctionAndRedeploy(createFunctionOptions, afterFirstDeploy, afterSecondDeploy)
}

func (suite *TestSuite) compileCreateFunctionOptions(functionName string) *platform.CreateFunctionOptions {
	createFunctionOptions := &platform.CreateFunctionOptions{
		Logger: suite.Logger,
		FunctionConfig: functionconfig.Config{
			Meta: functionconfig.Meta{
				Name:      functionName,
				Namespace: suite.Namespace,
			},
			Spec: functionconfig.Spec{
				Build: functionconfig.Build{
					Registry: suite.registryURL,
				},
			},
		},
	}
	createFunctionOptions.FunctionConfig.Meta.Namespace = suite.Namespace
	createFunctionOptions.FunctionConfig.Spec.Build.Registry = "localhost:5000"
	return createFunctionOptions
}

func (suite *TestSuite) createAndStartController() {
	testKubeconfigPath := common.GetEnvOrDefaultString("NUCLIO_TEST_KUBECONFIG", "")
	restConfig, err := common.GetClientConfig(common.GetKubeconfigPath(testKubeconfigPath))
	suite.Require().NoError(err)

	kubeClientSet, err := kubernetes.NewForConfig(restConfig)
	suite.Require().NoError(err)

	nuclioClientSet, err := nuclioioclient.NewForConfig(restConfig)
	suite.Require().NoError(err)

	// create a client for function deployments
	functionresClient, err := functionres.NewLazyClient(suite.Logger, kubeClientSet, nuclioClientSet)
	suite.Require().NoError(err)

	controllerInstance, err := controller.NewController(suite.Logger,
		suite.Namespace,
		"",
		kubeClientSet,
		nuclioClientSet,
		functionresClient,
		time.Second*5,
		time.Second*30,
		suite.PlatformConfiguration,
		4,
		4,
		4)
	suite.Require().NoError(err)
	go controllerInstance.Start() // nolint: errcheck
}

func TestPlatformTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}
	//if !common.GetEnvOrDefaultBool("NUCLIO_K8S_TESTS_ENABLED", false) {
	//	t.Skip("Test can only run when `NUCLIO_K8S_TESTS_ENABLED` environ is enabled")
	//}
	suite.Run(t, new(TestSuite))
}
