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
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/factory"
	"github.com/nuclio/nuclio/pkg/platform/kube"
	"github.com/nuclio/nuclio/pkg/platform/kube/apigatewayres"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	nuclioioclient "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	"github.com/nuclio/nuclio/pkg/platform/kube/controller"
	"github.com/nuclio/nuclio/pkg/platform/kube/functionres"
	"github.com/nuclio/nuclio/pkg/platform/kube/ingress"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	processorsuite "github.com/nuclio/nuclio/pkg/processor/test/suite"

	"github.com/ghodss/yaml"
	"github.com/nuclio/errors"
	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	kubeapierrors "k8s.io/apimachinery/pkg/api/errors"
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

	// fill test external ip addresses
	err = suite.Platform.SetExternalIPAddresses(strings.Split(suite.GetTestHost(), ","))
	suite.Require().NoError(err, "Failed to set platform external ip addresses")

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

	// start controller
	if err := suite.Controller.Start(); err != nil {
		suite.Require().NoError(err, "Failed to start controller")
	}
}

func (suite *KubeTestSuite) SetupTest() {
	suite.TestSuite.SetupTest()

	// default project gets deleted during testings, ensure it is being recreated
	err := factory.EnsureDefaultProjectExistence(suite.Logger, suite.Platform, suite.Namespace)
	suite.Require().NoError(err, "Failed to ensure default project exists")
}

func (suite *KubeTestSuite) TearDownTest() {
	suite.TestSuite.TearDownTest()

	defer func() {

		// delete leftovers if controller was not able to delete them
		suite.executeKubectl([]string{"delete", "all"}, // nolint: errcheck
			map[string]string{
				"selector": "nuclio.io/app",
			})
	}()

	// remove nuclio function leftovers
	var errGroup errgroup.Group
	for _, resourceKind := range []string{
		"nucliofunctions",
		"nuclioprojects",
		"nucliofunctionevents",
		"nuclioapigateways",
	} {
		resourceKind := resourceKind
		errGroup.Go(func() error {
			return suite.deleteAllResourcesByKind(resourceKind)
		})
	}

	// wait and ensure no error occurred during CRDs deletion
	suite.Require().NoError(errGroup.Wait(), "Failed waiting for CRDs deletion")

	// wait until controller delete all CRD resources (deployments, ingresses, etc)
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
	suite.Require().NoError(err, "Not all nuclio resources were deleted")
}

func (suite *KubeTestSuite) CompileCreateFunctionOptions(functionName string) *platform.CreateFunctionOptions {
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
	createFunctionOptions.FunctionConfig.Spec.Handler = "main:handler"
	createFunctionOptions.FunctionConfig.Spec.Runtime = "python:3.6"
	createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode = base64.StdEncoding.EncodeToString([]byte(`
def handler(context, event):
  return "hello world"
`))

	// expose function for testing purposes
	createFunctionOptions.FunctionConfig.Spec.ServiceType = v1.ServiceTypeNodePort
	return createFunctionOptions
}

func (suite *KubeTestSuite) GetFunctionAndExpectState(getFunctionOptions *platform.GetFunctionsOptions,
	expectedState functionconfig.FunctionState) platform.Function {
	function := suite.GetFunction(getFunctionOptions)
	suite.Require().Equal(expectedState,
		function.GetStatus().State,
		"Function is in unexpected state. Expected: %s, Existing: %s",
		expectedState, function.GetStatus().State)
	return function
}

func (suite *KubeTestSuite) GetFunction(getFunctionOptions *platform.GetFunctionsOptions) platform.Function {

	// get the function
	functions, err := suite.Platform.GetFunctions(getFunctionOptions)
	suite.Require().NoError(err)
	return functions[0]
}

func (suite *KubeTestSuite) GetAPIGateway(getAPIGatewayOptions *platform.GetAPIGatewaysOptions) platform.APIGateway {

	// get the function
	apiGateways, err := suite.Platform.GetAPIGateways(getAPIGatewayOptions)
	suite.Require().NoError(err)
	return apiGateways[0]
}

func (suite *KubeTestSuite) GetProject(getProjectFunctions *platform.GetProjectsOptions) platform.Project {
	projects, err := suite.Platform.GetProjects(getProjectFunctions)
	suite.Require().NoError(err, "Failed to get projects")
	return projects[0]
}

func (suite *KubeTestSuite) GetFunctionDeployment(functionName string) *appsv1.Deployment {
	deploymentInstance := &appsv1.Deployment{}
	suite.GetResourceAndUnmarshal("deployment",
		kube.DeploymentNameFromFunctionName(functionName),
		deploymentInstance)
	return deploymentInstance
}

func (suite *KubeTestSuite) GetAPIGatewayIngress(apiGatewayName string, canary bool) *extensionsv1beta1.Ingress {
	ingressInstance := &extensionsv1beta1.Ingress{}
	suite.GetResourceAndUnmarshal("ingress",
		kube.IngressNameFromAPIGatewayName(apiGatewayName, canary),
		ingressInstance)
	return ingressInstance
}

func (suite *KubeTestSuite) GetFunctionPods(functionName string) []v1.Pod {
	pods, err := suite.KubeClientSet.CoreV1().Pods(suite.Namespace).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("nuclio.io/function-name=%s", functionName),
	})

	suite.Require().NoError(err, "Failed to list function pods")
	return pods.Items
}

func (suite *KubeTestSuite) GetResourceAndUnmarshal(resourceKind, resourceName string, resource interface{}) {
	resourceContent := suite.getResource(resourceKind, resourceName)
	err := yaml.Unmarshal([]byte(resourceContent), resource)
	suite.Require().NoError(err)
}

func (suite *KubeTestSuite) WaitForFunctionDeployment(functionName string,
	duration time.Duration,
	callback func(*appsv1.Deployment) bool) {
	err := common.RetryUntilSuccessful(duration,
		time.Second,
		func() bool {
			return callback(suite.GetFunctionDeployment(functionName))
		})
	suite.Require().NoError(err, "Failed to wait on deployment callback")
}

func (suite *KubeTestSuite) WaitForFunctionState(getFunctionOptions *platform.GetFunctionsOptions,
	desiredFunctionState functionconfig.FunctionState,
	duration time.Duration) {

	err := common.RetryUntilSuccessful(duration,
		1*time.Second,
		func() bool {
			function := suite.GetFunction(getFunctionOptions)
			suite.Logger.InfoWith("Waiting for function state",
				"currentFunctionState", function.GetStatus().State,
				"desiredFunctionState", desiredFunctionState)
			return function.GetStatus().State == desiredFunctionState
		})
	suite.Require().NoError(err, "Function did not reach its desired state")
}

func (suite *KubeTestSuite) WaitForAPIGatewayState(getAPIGatewayOptions *platform.GetAPIGatewaysOptions,
	desiredAPIGatewayState platform.APIGatewayState,
	duration time.Duration) {

	err := common.RetryUntilSuccessful(duration,
		1*time.Second,
		func() bool {
			apiGateway := suite.GetAPIGateway(getAPIGatewayOptions)
			suite.Logger.InfoWith("Waiting for api gateway state",
				"currentAPIGatewayState", apiGateway.GetConfig().Status.State,
				"desiredAPIGatewayState", desiredAPIGatewayState)
			return apiGateway.GetConfig().Status.State == desiredAPIGatewayState
		})
	suite.Require().NoError(err, "Api gateway did not reach its desired state")
}

func (suite *KubeTestSuite) deployAPIGateway(createAPIGatewayOptions *platform.CreateAPIGatewayOptions,
	onAfterIngressCreated OnAfterIngressCreated) error {

	// deploy the api gateway
	err := suite.Platform.CreateAPIGateway(createAPIGatewayOptions)
	if err != nil {
		return err
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

	return nil
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
		if err != nil && !exist && kubeapierrors.IsNotFound(err) {
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

func (suite *KubeTestSuite) deleteAllResourcesByKind(kind string) error {
	_, err := suite.executeKubectl([]string{"delete", kind, "--all", "--force"},
		map[string]string{
			"grace-period": "0",
		})
	if err != nil {
		return errors.Wrapf(err, "Failed delete all resources for kind \"%s\"", kind)
	}
	suite.Logger.DebugWith("Successfully deleted all resources", "kind", kind)
	return nil
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

func (suite *KubeTestSuite) verifyCreatedTrigger(functionName string, trigger functionconfig.Trigger) bool {
	functionInstance := &nuclioio.NuclioFunction{}
	suite.GetResourceAndUnmarshal("nucliofunction",
		functionName,
		functionInstance)

	// TODO: verify other parts of the trigger spec
	suite.Require().Equal(trigger.Name, functionInstance.Spec.Triggers[trigger.Name].Name)
	suite.Require().Equal(trigger.Kind, functionInstance.Spec.Triggers[trigger.Name].Kind)
	suite.Require().Equal(trigger.MaxWorkers, functionInstance.Spec.Triggers[trigger.Name].MaxWorkers)
	return true
}

func (suite *KubeTestSuite) ensureTriggerAmount(functionName, triggerKind string, amount int) {
	functionInstance := &nuclioio.NuclioFunction{}
	suite.GetResourceAndUnmarshal("nucliofunction",
		functionName,
		functionInstance)

	functionHTTPTriggers := functionconfig.GetTriggersByKind(functionInstance.Spec.Triggers, triggerKind)
	suite.Require().Equal(amount, len(functionHTTPTriggers))
}

func (suite *KubeTestSuite) compileCreateAPIGatewayOptions(apiGatewayName string,
	functionName string) *platform.CreateAPIGatewayOptions {

	return &platform.CreateAPIGatewayOptions{
		APIGatewayConfig: &platform.APIGatewayConfig{
			Meta: platform.APIGatewayMeta{
				Name:      apiGatewayName,
				Namespace: suite.Namespace,
			},
			Spec: platform.APIGatewaySpec{
				Host:               "some-host",
				AuthenticationMode: ingress.AuthenticationModeNone,
				Upstreams: []platform.APIGatewayUpstreamSpec{
					{
						Kind: platform.APIGatewayUpstreamKindNuclioFunction,
						Nucliofunction: &platform.NuclioFunctionAPIGatewaySpec{
							Name: functionName,
						},
					},
				},
			},
		},
	}
}
