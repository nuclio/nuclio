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
	"github.com/nuclio/nuclio/pkg/platform/kube"
	"github.com/nuclio/nuclio/pkg/platform/kube/apigatewayres"
	nuclioioclient "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	"github.com/nuclio/nuclio/pkg/platform/kube/controller"
	"github.com/nuclio/nuclio/pkg/platform/kube/functionres"
	"github.com/nuclio/nuclio/pkg/platform/kube/ingress"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	processorsuite "github.com/nuclio/nuclio/pkg/processor/test/suite"

	"github.com/ghodss/yaml"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type OnAfterIngressCreated func(*extensionsv1beta1.Ingress)

type KubeTestSuite struct {
	processorsuite.TestSuite
	CmdRunner     cmdrunner.CmdRunner
	RegistryURL   string
	Controller    *controller.Controller
	KubeClientSet *kubernetes.Clientset
}

// To run this test suite you should:
// - set NUCLIO_K8S_TESTS_ENABLED env to true
// - have Nuclio CRDs installed (you can install them by running "test/k8s/ci_assets/install_nuclio_crds.sh")
// - have docker registry running (you can run docker registry by running "docker run --rm -d -p 5000:5000 registry:2")
// - use "(kube) - platform test" run configuration via GoLand to run your test
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

	// only set up parent AFTER we set platform's type
	suite.TestSuite.SetupSuite()

	suite.RegistryURL = common.GetEnvOrDefaultString("NUCLIO_TEST_REGISTRY_URL", "localhost:5000")

	suite.CmdRunner, err = cmdrunner.NewShellRunner(suite.Logger)
	suite.Require().NoError(err, "Failed to create shell runner")

	// do not rename function name
	suite.FunctionNameUniquify = false

	// create kube client set
	restConfig, err := common.GetClientConfig(common.GetKubeconfigPath(""))
	suite.Require().NoError(err)

	suite.KubeClientSet, err = kubernetes.NewForConfig(restConfig)
	suite.Require().NoError(err)

	// create controller instance
	suite.Controller = suite.createController()
}

func (suite *KubeTestSuite) TearDownTest() {
	suite.TestSuite.TearDownTest()

	// remove nuclio function leftovers
	defer func() {
		_, err := suite.executeKubectl([]string{"delete", "nucliofunctions", "--all", "--force"},
			map[string]string{
				"grace-period": "0",
			})
		suite.Require().NoError(err)
	}()

	// remove nuclio apigateway leftovers
	defer func() {
		_, err := suite.executeKubectl([]string{"delete", "nuclioapigateways", "--all", "--force"},
			map[string]string{
				"grace-period": "0",
			})
		suite.Require().NoError(err)
	}()

	// wait until controller remove it all
	err := common.RetryUntilSuccessful(5*time.Minute,
		5*time.Second,
		func() bool {
			results, err := suite.executeKubectl([]string{"get", "all"},
				map[string]string{
					"selector": "nuclio.io/app",
				})
			if err != nil {
				return false
			}
			return strings.Contains(results.Output, "No resources found in")
		})
	suite.Require().NoError(err)
}

func (suite *KubeTestSuite) deployAPIGateway(createAPIGatewayOptions *platform.CreateAPIGatewayOptions,
	onAfterIngressCreated OnAfterIngressCreated,
	expectError bool) {

	// deploy the api gateway
	err := suite.Platform.CreateAPIGateway(createAPIGatewayOptions)

	if !expectError {
		suite.Require().NoError(err)
	} else {
		suite.Require().Error(err)
	}

	// delete the api gateway when done
	defer func() {

		if err == nil {
			suite.Logger.Debug("Deleting deployed api gateway")
			err := suite.Platform.DeleteAPIGateway(&platform.DeleteAPIGatewayOptions{
				Meta: createAPIGatewayOptions.APIGatewayConfig.Meta,
			})
			suite.Require().NoError(err)

			suite.verifyAPIGatewayIngress(createAPIGatewayOptions, false)
		}
	}()

	// verify ingress created
	ingressObject := suite.verifyAPIGatewayIngress(createAPIGatewayOptions, true)

	onAfterIngressCreated(ingressObject)
}

func (suite *KubeTestSuite) verifyAPIGatewayIngress(createAPIGatewayOptions *platform.CreateAPIGatewayOptions, exist bool) *extensionsv1beta1.Ingress {
	deadline := time.Now().Add(10 * time.Second)

	var ingressObject *extensionsv1beta1.Ingress
	var err error

	for {

		// stop after 10 seconds
		if time.Now().After(deadline) {
			suite.FailNow("API gateway ingress didn't create in time")
		}

		ingressObject, err = suite.KubeClientSet.
			ExtensionsV1beta1().
			Ingresses(suite.Namespace).
			Get(
				// TODO: consider canary ingress as well
				kube.IngressNameFromAPIGatewayName(createAPIGatewayOptions.APIGatewayConfig.Meta.Name, false),
				metav1.GetOptions{})
		if err != nil && !exist && errors.IsNotFound(err) {
			suite.Logger.DebugWith("API gateway ingress removed")
			break
		}
		if err == nil && exist {
			suite.Logger.DebugWith("API gateway ingress created")
			break
		}
	}
	return ingressObject
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

func (suite *KubeTestSuite) getResourceAndUnmarshal(resourceKind, resourceName string, resource interface{}) {
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

	nuclioClientSet, err := nuclioioclient.NewForConfig(restConfig)
	suite.Require().NoError(err)

	// create a client for function deployments
	functionresClient, err := functionres.NewLazyClient(suite.Logger, suite.KubeClientSet, nuclioClientSet)
	suite.Require().NoError(err)

	// create ingress manager
	ingressManager, err := ingress.NewManager(suite.Logger, suite.KubeClientSet, suite.PlatformConfiguration)
	suite.Require().NoError(err)

	// create api-gateway provisioner
	apigatewayresClient, err := apigatewayres.NewLazyClient(suite.Logger,
		suite.KubeClientSet,
		nuclioClientSet,
		ingressManager)
	suite.Require().NoError(err)

	controllerInstance, err := controller.NewController(suite.Logger,
		suite.Namespace,
		"",
		suite.KubeClientSet,
		nuclioClientSet,
		functionresClient,
		apigatewayresClient,
		time.Second*5,  // resync interval
		time.Second*5,  // monitor interval
		time.Second*30, // cronjob stale duration
		suite.PlatformConfiguration,
		"nuclio-platform-config",
		4,
		4,
		4,
		4)
	suite.Require().NoError(err)
	return controllerInstance
}
