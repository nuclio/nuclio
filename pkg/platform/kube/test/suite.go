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
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errgroup"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
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
	"github.com/rs/xid"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	kubeapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type OnAfterIngressCreated func(*networkingv1.Ingress)

type KubeTestSuite struct {
	processorsuite.TestSuite
	CmdRunner         cmdrunner.CmdRunner
	RegistryURL       string
	Controller        *controller.Controller
	KubeClientSet     *kubernetes.Clientset
	FunctionClientSet *nuclioioclient.Clientset
	FunctionClient    functionres.Client

	DisableControllerStart bool
	Ctx                    context.Context
}

// SetupSuite To run this test suite you should:
// - Have Helm 3 Installed - click here for instructions https://helm.sh/docs/intro/install
// - Kubernetes for Mac: Ingress controller installed (you can install it by running "test/k8s/ci_assets/install_nginx_ingress_controller.sh")
// - have Nuclio CRDs installed (you can install them by running "test/k8s/ci_assets/install_nuclio_crds.sh")
// - have docker registry running (you can run docker registry by running "docker run --rm -d -p 5000:5000 registry:2")
// - use "(kube) - platform test" run configuration via GoLand to run your test
func (suite *KubeTestSuite) SetupSuite() {
	var err error

	suite.Ctx = context.Background()

	common.SetVersionFromEnv()
	suite.Namespace = common.GetEnvOrDefaultString("NUCLIO_TEST_NAMESPACE", "default")
	suite.PlatformType = "kube"

	if suite.PlatformConfiguration == nil {
		suite.PlatformConfiguration = &platformconfig.Config{}
	}

	suite.PlatformConfiguration.Kind = suite.PlatformType

	// use Kubernetes cron job to invoke nuclio functions with cron triggers
	suite.PlatformConfiguration.CronTriggerCreationMode = platformconfig.KubeCronTriggerCreationMode

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

	if !suite.DisableControllerStart {

		// start controller
		if err := suite.Controller.Start(suite.Ctx); err != nil {
			suite.Require().NoError(err, "Failed to start controller")
		}
	}
}

func (suite *KubeTestSuite) SetupTest() {
	suite.TestSuite.SetupTest()

	// default project gets deleted during testings, ensure it is being recreated
	err := suite.Platform.EnsureDefaultProjectExistence(suite.Ctx)
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
	errGroup, _ := errgroup.WithContext(suite.Ctx, suite.Logger)
	for _, resourceKind := range []string{
		"nucliofunctions",
		"nuclioprojects",
		"nucliofunctionevents",
		"nuclioapigateways",
	} {
		resourceKind := resourceKind
		errGroup.Go(fmt.Sprintf("Delete %s resources", resourceKind), func() error {
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
				Labels:    map[string]string{},
			},
			Spec: functionconfig.Spec{
				Build: functionconfig.Build{
					Registry: suite.RegistryURL,
				},
			},
		},
	}
	createFunctionOptions.FunctionConfig.Spec.Handler = "main:handler"
	createFunctionOptions.FunctionConfig.Spec.Runtime = "python:3.8"
	createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode = base64.StdEncoding.EncodeToString([]byte(`
def handler(context, event):
  return "hello world"
`))

	// expose function for testing purposes
	createFunctionOptions.FunctionConfig.Spec.ServiceType = v1.ServiceTypeNodePort

	// don't explicitly pull base images before building
	createFunctionOptions.FunctionConfig.Spec.Build.NoBaseImagesPull = true
	return createFunctionOptions
}

func (suite *KubeTestSuite) CompileCreateFunctionEventOptions(functionEventName, functionName string) *platform.CreateFunctionEventOptions {
	return &platform.CreateFunctionEventOptions{
		FunctionEventConfig: platform.FunctionEventConfig{
			Meta: platform.FunctionEventMeta{
				Name:      functionEventName,
				Namespace: suite.Namespace,
				Labels: map[string]string{
					"nuclio.io/function-name": functionName,
				},
			},
			Spec: platform.FunctionEventSpec{

				// random body
				Body: xid.New().String(),
			},
		},
	}
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

func (suite *KubeTestSuite) TryGetAndUnmarshalFunctionRecordedEvents(functionURL string,
	retryDuration time.Duration,
	events interface{}) {
	err := common.RetryUntilSuccessful(retryDuration,
		2*time.Second,
		func() bool {
			suite.Logger.DebugWith("Trying to get recorded events", "functionURL", functionURL)

			// invoke function
			httpResponse, err := http.Get(functionURL)
			if err != nil {
				suite.Logger.WarnWith("Failed to get function recorded events",
					"functionURL", functionURL,
					"err", err)
				return false
			}

			// read response body
			responseBody, err := ioutil.ReadAll(httpResponse.Body)
			if err != nil {
				suite.Logger.WarnWith("Failed to read response body", "err", err)
				return false
			}

			// unmarshal recorded events
			if err = json.Unmarshal(responseBody, &events); err != nil {
				suite.Logger.WarnWith("Failed to unmarshal response body",
					"responseBody", responseBody,
					"err", err)
				return false
			}

			// events has not been unmarshalled yet, responseBody might be empty
			if events == nil {
				return false
			}

			// a bit hacky, but:
			// this is how you can determine whether an `interface{}` is a slice
			// we do it because the invoked functions returns a list of "unknown" events.
			// here, we simply want to know the list has been initialized and its length is greater than zero.
			switch kind := reflect.TypeOf(events).Kind(); kind {
			case reflect.Slice, reflect.Ptr:
				return reflect.Indirect(reflect.ValueOf(events)).Len() > 0
			default:
				suite.Require().FailNow("Expected a list", "receivedKind", kind)
				return false
			}
		})

	suite.Require().NoError(err)
	suite.Logger.DebugWith("Got events from event recorder function",
		"events", events)
}

func (suite *KubeTestSuite) GetAPIGateway(getAPIGatewayOptions *platform.GetAPIGatewaysOptions) platform.APIGateway {

	// get the function
	apiGateways, err := suite.Platform.GetAPIGateways(suite.Ctx, getAPIGatewayOptions)
	suite.Require().NoError(err)
	return apiGateways[0]
}

func (suite *KubeTestSuite) GetProject(getProjectFunctions *platform.GetProjectsOptions) platform.Project {
	projects, err := suite.Platform.GetProjects(suite.Ctx, getProjectFunctions)
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

func (suite *KubeTestSuite) GetAPIGatewayIngress(apiGatewayName string, canary bool) *networkingv1.Ingress {
	ingressInstance := &networkingv1.Ingress{}
	suite.GetResourceAndUnmarshal("ingress",
		kube.IngressNameFromAPIGatewayName(apiGatewayName, canary),
		ingressInstance)
	return ingressInstance
}

func (suite *KubeTestSuite) GetFunctionIngress(functionName string) *networkingv1.Ingress {
	ingressInstance := &networkingv1.Ingress{}
	suite.GetResourceAndUnmarshal("ingress",
		kube.IngressNameFromFunctionName(functionName),
		ingressInstance)
	return ingressInstance
}

func (suite *KubeTestSuite) WithResourceQuota(rq *v1.ResourceQuota, handler func()) {
	// limit running pod on a node
	resourceQuota, err := suite.KubeClientSet.
		CoreV1().
		ResourceQuotas(suite.Namespace).
		Create(suite.Ctx, rq, metav1.CreateOptions{})
	suite.Require().NoError(err)

	// clean leftovers
	defer suite.KubeClientSet.
		CoreV1().
		ResourceQuotas(suite.Namespace).
		Delete(suite.Ctx, resourceQuota.Name, metav1.DeleteOptions{}) // nolint: errcheck

	handler()
}

func (suite *KubeTestSuite) GetFunctionPods(functionName string) []v1.Pod {
	pods, err := suite.KubeClientSet.CoreV1().Pods(suite.Namespace).List(suite.Ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("nuclio.io/function-name=%s", functionName),
	})

	suite.Require().NoError(err, "Failed to list function pods")
	return pods.Items
}

func (suite *KubeTestSuite) DrainNode(nodeName string, ignoreDaemonSet bool) error {
	positionalArgs := []string{"drain", nodeName}
	if ignoreDaemonSet {
		positionalArgs = append(positionalArgs, "--ignore-daemonsets")
	}
	_, err := suite.executeKubectl(positionalArgs, nil)
	return err
}

func (suite *KubeTestSuite) UnCordonNode(nodeName string) error {
	_, err := suite.executeKubectl([]string{"uncordon", nodeName}, nil)
	return err
}

func (suite *KubeTestSuite) GetNodes() []v1.Node {
	nodesList, err := suite.KubeClientSet.CoreV1().Nodes().List(suite.Ctx, metav1.ListOptions{})
	suite.Require().NoError(err)
	return nodesList.Items
}

func (suite *KubeTestSuite) DeleteFunctionPods(functionName string) {
	suite.Logger.InfoWith("Deleting function pods", "functionName", functionName)
	errGroup, _ := errgroup.WithContext(suite.Ctx, suite.Logger)
	for _, pod := range suite.GetFunctionPods(functionName) {
		pod := pod
		errGroup.Go("Delete function pods", func() error {
			suite.Logger.DebugWith("Deleting function pod",
				"functionName", functionName,
				"podName", pod.Name)
			return suite.KubeClientSet.
				CoreV1().
				Pods(suite.Namespace).
				Delete(suite.Ctx, pod.Name, *metav1.NewDeleteOptions(0))
		})
	}
	suite.Require().NoError(errGroup.Wait(), "Failed to delete function pods")
}

func (suite *KubeTestSuite) GetResourceAndUnmarshal(resourceKind, resourceName string, resource interface{}) {
	resourceContent := suite.getResource(resourceKind, resourceName)
	err := yaml.Unmarshal([]byte(resourceContent), resource)
	suite.Require().NoError(err)
}

func (suite *KubeTestSuite) CreateImportedFunction(functionName, projectName string) *functionconfig.Config {
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	createFunctionOptions.FunctionConfig.Meta.Annotations = map[string]string{
		functionconfig.FunctionAnnotationSkipBuild:  "true",
		functionconfig.FunctionAnnotationSkipDeploy: "true",
	}
	createFunctionOptions.FunctionConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName] = projectName
	suite.PopulateDeployOptions(createFunctionOptions)
	_, err := suite.Platform.CreateFunction(suite.Ctx, createFunctionOptions)
	suite.Require().NoError(err)
	suite.WaitForFunctionState(&platform.GetFunctionsOptions{
		Name:      createFunctionOptions.FunctionConfig.Meta.Name,
		Namespace: suite.Namespace,
	}, functionconfig.FunctionStateImported, time.Minute)
	return &createFunctionOptions.FunctionConfig
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

func (suite *KubeTestSuite) WaitForFunctionPods(functionName string,
	duration time.Duration,
	callback func(pods []v1.Pod) bool) {
	err := common.RetryUntilSuccessful(duration,
		time.Second,
		func() bool {
			return callback(suite.GetFunctionPods(functionName))
		})
	suite.Require().NoError(err, "Failed to wait on function pods callback")
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

func (suite *KubeTestSuite) KubectlInvokeFunctionViaCurl(functionName string, curlCommand string) string {
	curlPodName := fmt.Sprintf("curl-%s", functionName)

	// start curl pod, let it sleep
	runCurlPodCommand := fmt.Sprintf(""+
		"kubectl "+
		"run "+
		"%s "+
		"--image=curlimages/curl:7.77.0 "+
		"--restart=Never "+
		"--command -- "+
		"sleep 600",
		curlPodName)
	_, err := suite.CmdRunner.Run(nil, runCurlPodCommand)
	suite.Require().NoError(err)

	waitForCurlPodReadyCommand := fmt.Sprintf("kubectl wait --for=condition=ready pod/%s", curlPodName)
	_, err = suite.CmdRunner.Run(nil, waitForCurlPodReadyCommand)
	suite.Require().NoError(err)

	execCurlCommand := fmt.Sprintf(
		"kubectl "+
			"exec "+
			"%s -- "+
			"curl %s",
		curlPodName,
		curlCommand)

	curlResults, err := suite.CmdRunner.Run(nil, execCurlCommand)
	suite.Require().NoError(err)

	return curlResults.Output
}

func (suite *KubeTestSuite) DeployAPIGateway(createAPIGatewayOptions *platform.CreateAPIGatewayOptions,
	onAfterIngressCreated OnAfterIngressCreated) error {

	// deploy the api gateway
	if err := suite.Platform.CreateAPIGateway(suite.Ctx, createAPIGatewayOptions); err != nil {
		return err
	}

	// delete the api gateway when done
	defer func() {
		suite.Logger.Debug("Deleting deployed api gateway")
		err := suite.Platform.DeleteAPIGateway(suite.Ctx, &platform.DeleteAPIGatewayOptions{
			Meta: createAPIGatewayOptions.APIGatewayConfig.Meta,
		})
		suite.Require().NoError(err)
		suite.verifyAPIGatewayIngress(createAPIGatewayOptions, false)

	}()

	// verify ingress created
	ingressObject := suite.verifyAPIGatewayIngress(createAPIGatewayOptions, true)

	onAfterIngressCreated(ingressObject)

	return nil
}

func (suite *KubeTestSuite) verifyAPIGatewayIngress(createAPIGatewayOptions *platform.CreateAPIGatewayOptions, exist bool) *networkingv1.Ingress {
	deadline := time.Now().Add(10 * time.Second)

	var ingressObject *networkingv1.Ingress
	var err error

	for {

		// stop after 10 seconds
		if time.Now().After(deadline) {
			suite.FailNow("API gateway ingress didn't create in time")
		}

		ingressObject, err = suite.KubeClientSet.
			NetworkingV1().
			Ingresses(suite.Namespace).
			Get(suite.Ctx,

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

	suite.FunctionClientSet, err = nuclioioclient.NewForConfig(restConfig)
	suite.Require().NoError(err)

	// create a client for function deployments
	suite.FunctionClient, err = functionres.NewLazyClient(suite.Logger, suite.KubeClientSet, suite.FunctionClientSet)
	suite.Require().NoError(err)

	// create ingress manager
	ingressManager, err := ingress.NewManager(suite.Logger, suite.KubeClientSet, suite.PlatformConfiguration)
	suite.Require().NoError(err)

	// create api-gateway provisioner
	apigatewayresClient, err := apigatewayres.NewLazyClient(suite.Logger,
		suite.KubeClientSet,
		suite.FunctionClientSet,
		ingressManager)
	suite.Require().NoError(err)

	controllerInstance, err := controller.NewController(suite.Logger,
		suite.Namespace,
		"",
		suite.KubeClientSet,
		suite.FunctionClientSet,
		suite.FunctionClient,
		apigatewayresClient,
		0,              // disable resync interval
		time.Second*5,  // monitor interval
		time.Second*30, // cronjob stale duration
		suite.PlatformConfiguration,
		"nuclio-platform-config",
		1,
		1,
		1,
		1)
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

func (suite *KubeTestSuite) CompileCreateAPIGatewayOptions(apiGatewayName string,
	functionNames ...string) *platform.CreateAPIGatewayOptions {

	var upstreams []platform.APIGatewayUpstreamSpec
	for idx, functionName := range functionNames {
		upstreams = append(upstreams, platform.APIGatewayUpstreamSpec{
			Kind: platform.APIGatewayUpstreamKindNuclioFunction,
			NuclioFunction: &platform.NuclioFunctionAPIGatewaySpec{
				Name: functionName,
			},
			Percentage: int((float64(idx) / float64(len(functionNames))) * 100),
		})
	}

	return &platform.CreateAPIGatewayOptions{
		APIGatewayConfig: &platform.APIGatewayConfig{
			Meta: platform.APIGatewayMeta{
				Name:      apiGatewayName,
				Namespace: suite.Namespace,
				Labels:    map[string]string{},
			},
			Spec: platform.APIGatewaySpec{
				Host:               "some-host",
				AuthenticationMode: ingress.AuthenticationModeNone,
				Upstreams:          upstreams,
			},
		},
	}
}
