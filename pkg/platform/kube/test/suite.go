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
	"strings"
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

	"github.com/ghodss/yaml"
	"k8s.io/client-go/kubernetes"
)

type KubeTestSuite struct {
	processorsuite.TestSuite
	CmdRunner   cmdrunner.CmdRunner
	RegistryURL string
	Controller  *controller.Controller
}

func (suite *KubeTestSuite) SetupSuite() {
	if !common.GetEnvOrDefaultBool("NUCLIO_K8S_TESTS_ENABLED", false) {
		suite.T().Skip("Test can only run when `NUCLIO_K8S_TESTS_ENABLED` environ is enabled")
	}
	var err error
	suite.Namespace = common.GetEnvOrDefaultString("NUCLIO_TEST_NAMESPACE", "default")
	suite.PlatformType = "kube"

	if suite.PlatformConfiguration == nil {
		suite.PlatformConfiguration = &platformconfig.Config{}
	}

	suite.PlatformConfiguration.Kind = suite.PlatformType

	suite.TestSuite.SetupSuite()

	suite.RegistryURL = common.GetEnvOrDefaultString("NUCLIO_TEST_REGISTRY_URL", "localhost:5000")

	suite.CmdRunner, err = cmdrunner.NewShellRunner(suite.Logger)
	suite.Require().NoError(err, "Failed to create shell runner")

	// do not rename function name
	suite.FunctionNameUniquify = false

	// create controller instance
	suite.Controller = suite.createController()
}

func (suite *KubeTestSuite) executeKubectl(positionalArgs []string,
	namedArgs map[string]string) (cmdrunner.RunResult, error) {

	argsStringSlice := []string{
		"kubectl",
	}

	// add positional arguments
	argsStringSlice = append(argsStringSlice, positionalArgs...)

	for argName, argValue := range namedArgs {
		argsStringSlice = append(argsStringSlice, fmt.Sprintf("--%s %s", argName, argValue))
	}

	encodedCommand := strings.Join(argsStringSlice, " ")

	suite.Logger.DebugWith("Running kubectl", "encodedCommand", encodedCommand)
	return suite.CmdRunner.Run(nil, encodedCommand)

}

func (suite *KubeTestSuite) getResource(resourceKind, resourceName string) string {
	results, err := suite.executeKubectl([]string{
		"get", resourceKind, resourceName},
		map[string]string{
			"namespace": suite.Namespace,
			"output":    "yaml",
		})
	suite.Require().NoError(err)
	return results.Output
}

func (suite *KubeTestSuite) populateResource(resourceKind, resourceName string, resource interface{}) {
	resourceContent := suite.getResource(resourceKind, resourceName)
	err := yaml.Unmarshal([]byte(resourceContent), resource)
	suite.Require().NoError(err)
}

func (suite *KubeTestSuite) compileCreateFunctionOptions(functionName string) *platform.CreateFunctionOptions {
	createFunctionOptions := &platform.CreateFunctionOptions{
		Logger: suite.Logger,
		FunctionConfig: functionconfig.Config{
			Meta: functionconfig.Meta{
				Name:      functionName,
				Namespace: suite.Namespace,
			},
			Spec: functionconfig.Spec{
				Build: functionconfig.Build{
					Registry: suite.RegistryURL,
				},
			},
		},
	}
	createFunctionOptions.FunctionConfig.Meta.Namespace = suite.Namespace
	createFunctionOptions.FunctionConfig.Spec.Build.Registry = suite.RegistryURL
	return createFunctionOptions
}

func (suite *KubeTestSuite) createController() *controller.Controller {
	restConfig, err := common.GetClientConfig(common.GetKubeconfigPath(""))
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
	return controllerInstance
}
