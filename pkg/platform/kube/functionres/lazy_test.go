//go:build test_unit

/*
Copyright 2023 The Nuclio Authors.

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

package functionres

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform/abstract"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	nuclioiofake "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned/fake"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"dario.cat/mergo"
	"github.com/google/go-cmp/cmp"
	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	autosv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type mockedPlatformConfigurationProvider struct {
	platformConfiguration *platformconfig.Config
}

func (c *mockedPlatformConfigurationProvider) GetPlatformConfigurationName() string {
	return "mocked-platform-configuration"
}

func (c *mockedPlatformConfigurationProvider) GetPlatformConfiguration() *platformconfig.Config {
	return c.platformConfiguration
}

type lazyTestSuite struct {
	suite.Suite
	logger logger.Logger
	client *lazyClient
	ctx    context.Context
}

func (suite *lazyTestSuite) SetupTest() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	// create client
	lazyClientInstance, err := NewLazyClient(suite.logger,
		fake.NewSimpleClientset(),
		nuclioiofake.NewSimpleClientset())
	suite.Require().NoError(err)
	suite.client = lazyClientInstance.(*lazyClient)
	suite.ctx = context.Background()

	// use the default platform configuration
	defaultPlatformConfiguration, err := platformconfig.NewPlatformConfig("")
	suite.Require().NoError(err)
	suite.client.SetPlatformConfigurationProvider(&mockedPlatformConfigurationProvider{
		platformConfiguration: defaultPlatformConfiguration,
	})

	// don't wait for too long
	suite.client.nodeScaleUpSleepTimeout = 100 * time.Millisecond
}

func (suite *lazyTestSuite) TestNodeConstrains() {
	functionInstance := &nuclioio.NuclioFunction{}
	functionInstance.Name = "func-name"
	functionInstance.Spec.NodeName = "some-node-name"
	functionInstance.Spec.NodeSelector = map[string]string{
		"some-key": "some-value",
	}
	functionInstance.Spec.Affinity = &v1.Affinity{
		NodeAffinity: &v1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{
					{
						MatchExpressions: []v1.NodeSelectorRequirement{
							{
								Key: "req-key",
								Values: []string{
									"a",
									"b",
								},
							},
						},
					},
				},
			},
		},
	}
	resources, err := suite.client.CreateOrUpdate(suite.ctx, functionInstance, "")
	suite.Require().NoError(err)
	suite.Require().NotEmpty(resources)
	deployment, err := resources.Deployment()
	suite.Require().NoError(err)

	// ensure fields were passed
	deployment.Spec.Template.Spec.NodeName = functionInstance.Spec.NodeName
	deployment.Spec.Template.Spec.NodeSelector = functionInstance.Spec.NodeSelector
	deployment.Spec.Template.Spec.Affinity = functionInstance.Spec.Affinity
}

func (suite *lazyTestSuite) TestEnrichIngressWithDefaultAnnotations() {
	defaultIngressAnnotations := map[string]string{
		"a": "b",
	}
	suite.client.SetPlatformConfigurationProvider(&mockedPlatformConfigurationProvider{
		platformConfiguration: &platformconfig.Config{
			Kube: platformconfig.PlatformKubeConfig{
				DefaultHTTPIngressAnnotations: defaultIngressAnnotations,
			},
		},
	})
	for _, testCase := range []struct {
		name                               string
		functionIngressAnnotations         map[string]string
		expectedFunctionIngressAnnotations map[string]string
	}{
		{
			name: "sanity-no-override-with-value",
			functionIngressAnnotations: map[string]string{
				"a": "c",
			},
			expectedFunctionIngressAnnotations: map[string]string{
				"a": "c",
			},
		},
		{
			name: "sanity-no-override-empty-value",
			functionIngressAnnotations: map[string]string{
				"a": "",
			},
			expectedFunctionIngressAnnotations: map[string]string{
				"a": "",
			},
		},
		{
			name: "override",
			functionIngressAnnotations: map[string]string{
				"x": "y",
			},
			expectedFunctionIngressAnnotations: func() map[string]string {
				ingressAnnotations := map[string]string{
					"x": "y",
				}
				err := mergo.Merge(&ingressAnnotations, &defaultIngressAnnotations)
				suite.Require().NoError(err)
				return ingressAnnotations
			}(),
		},
	} {
		suite.Run(testCase.name, func() {
			one := 1
			defaultHTTPTrigger := functionconfig.GetDefaultHTTPTrigger()
			defaultHTTPTrigger.Annotations = testCase.functionIngressAnnotations
			defaultHTTPTrigger.Attributes = map[string]interface{}{
				"ingresses": map[string]interface{}{
					"0": map[string]interface{}{
						"host":  "something.com",
						"paths": []string{"/"},
					},
				},
			}
			function := nuclioio.NuclioFunction{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-function" + testCase.name,
				},
				Spec: functionconfig.Spec{
					Replicas: &one,
					Triggers: map[string]functionconfig.Trigger{
						defaultHTTPTrigger.Name: defaultHTTPTrigger,
					},
				},
			}
			functionLabels := suite.client.getFunctionLabels(&function)
			functionLabels["nuclio.io/function-name"] = function.Name

			// "create the ingress
			ingressInstance, err := suite.client.createOrUpdateIngress(suite.ctx, functionLabels, &function)
			suite.Require().NoError(err)
			suite.Require().NotNil(ingressInstance)
			suite.Require().NotEmpty(ingressInstance.Annotations)

			// make sure user function annotations exists
			delete(ingressInstance.Annotations, "nginx.ingress.kubernetes.io/configuration-snippet")
			suite.Require().Equal(testCase.expectedFunctionIngressAnnotations,
				ingressInstance.Annotations)

		})
	}
}

func (suite *lazyTestSuite) TestEnrichIngressTLS() {
	sslRedirectAnnotation := "nginx.ingress.kubernetes.io/ssl-redirect"

	for _, testCase := range []struct {
		name              string
		enableSSLRedirect bool
		tlsSecret         string
	}{
		{
			name:              "no-tls-secret-no-ssl-redirect",
			enableSSLRedirect: false,
			tlsSecret:         "",
		},
		{
			name:              "no-tls-secret-ssl-redirect",
			enableSSLRedirect: true,
			tlsSecret:         "",
		},
		{
			name:              "tls-secret-no-ssl-redirect",
			enableSSLRedirect: false,
			tlsSecret:         "my-tls-secret",
		},
	} {
		suite.Run(testCase.name, func() {
			suite.client.SetPlatformConfigurationProvider(&mockedPlatformConfigurationProvider{
				platformConfiguration: &platformconfig.Config{
					IngressConfig: platformconfig.IngressConfig{
						TLSSecret:         testCase.tlsSecret,
						EnableSSLRedirect: testCase.enableSSLRedirect,
					},
				},
			})
			host := "something.com"
			one := 1
			defaultHTTPTrigger := functionconfig.GetDefaultHTTPTrigger()
			defaultHTTPTrigger.Attributes = map[string]interface{}{
				"ingresses": map[string]interface{}{
					"0": map[string]interface{}{
						"host":  host,
						"paths": []string{"/"},
					},
				},
			}
			function := nuclioio.NuclioFunction{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-function" + testCase.name,
				},
				Spec: functionconfig.Spec{
					Replicas: &one,
					Triggers: map[string]functionconfig.Trigger{
						defaultHTTPTrigger.Name: defaultHTTPTrigger,
					},
				},
			}
			functionLabels := suite.client.getFunctionLabels(&function)

			ingressInstance, err := suite.client.createOrUpdateIngress(suite.ctx, functionLabels, &function)
			suite.Require().NoError(err)
			suite.Require().NotNil(ingressInstance)

			if testCase.enableSSLRedirect {
				suite.Require().Equal("true", ingressInstance.Annotations[sslRedirectAnnotation])
			} else {
				suite.Require().NotContains(ingressInstance.Annotations, sslRedirectAnnotation)
			}
			if testCase.tlsSecret != "" {
				suite.Require().Equal(testCase.tlsSecret, ingressInstance.Spec.TLS[0].SecretName)
				suite.Require().Equal(host, ingressInstance.Spec.TLS[0].Hosts[0])
			} else {
				suite.Require().Empty(ingressInstance.Spec.TLS)
			}
		})
	}
}

func (suite *lazyTestSuite) TestEnrichIngressWithDefaultTLSSecret() {
	tlsSecretName := "my-secret"
	suite.client.SetPlatformConfigurationProvider(&mockedPlatformConfigurationProvider{
		platformConfiguration: &platformconfig.Config{
			IngressConfig: platformconfig.IngressConfig{
				TLSSecret:         tlsSecretName,
				EnableSSLRedirect: true,
			},
		},
	})
	one := 1
	defaultHTTPTrigger := functionconfig.GetDefaultHTTPTrigger()
	defaultHTTPTrigger.Attributes = map[string]interface{}{
		"ingresses": map[string]interface{}{
			"0": map[string]interface{}{
				"host":  "something.com",
				"paths": []string{"/"},
			},
		},
	}
	function := nuclioio.NuclioFunction{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-function",
		},
		Spec: functionconfig.Spec{
			Replicas: &one,
			Triggers: map[string]functionconfig.Trigger{
				defaultHTTPTrigger.Name: defaultHTTPTrigger,
			},
		},
	}
	// "create the ingress
	ingressInstance, err := suite.client.createOrUpdateIngress(suite.ctx, map[string]string{}, &function)
	suite.Require().NoError(err)
	suite.Require().NotNil(ingressInstance)

	// make sure default TLS secret exists
	sslRedirectAnnotation := "nginx.ingress.kubernetes.io/ssl-redirect"
	suite.Require().Equal(ingressInstance.Spec.TLS[0].SecretName, tlsSecretName)
	suite.Require().Contains(ingressInstance.Annotations, sslRedirectAnnotation)
	suite.Require().Equal("true", ingressInstance.Annotations[sslRedirectAnnotation])
}

func (suite *lazyTestSuite) TestNoChanges() {
	one := 1
	volumeName := "my-volume"
	defaultHTTPTrigger := functionconfig.GetDefaultHTTPTrigger()
	defaultHTTPTrigger.Attributes = map[string]interface{}{
		"ingresses": map[string]interface{}{
			"0": map[string]interface{}{
				"hostTemplate": "@nuclio.fromDefault",
				"paths":        []string{"/"},
			},
		},
	}
	function := nuclioio.NuclioFunction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-function",
			Namespace: "test-namespace",
			Labels: map[string]string{

				// we want the created ingress host to exceed the length limitation
				"nuclio.io/project-name": common.GenerateRandomString(60, common.SmallLettersAndNumbers),
			},
		},
		Spec: functionconfig.Spec{
			Replicas: &one,
			Triggers: map[string]functionconfig.Trigger{
				defaultHTTPTrigger.Name: defaultHTTPTrigger,
			},
			Volumes: []functionconfig.Volume{
				{
					Volume: v1.Volume{
						Name: volumeName,
						VolumeSource: v1.VolumeSource{
							FlexVolume: &v1.FlexVolumeSource{
								Driver: "v3io/fuse",
								Options: map[string]string{
									"container": "users",
									"subPath":   "/",
									"accessKey": "$ref:/spec/volumes/bla/bla",
								},
							},
						},
					},
					VolumeMount: v1.VolumeMount{
						Name:      volumeName,
						MountPath: "/tmp/vol-1",
					},
				},
			},
		},
	}
	functionLabels := suite.client.getFunctionLabels(&function)
	functionLabels["nuclio.io/function-name"] = function.Name

	// mock volume secret creation
	_, err := suite.client.kubeClientSet.CoreV1().Secrets("test-namespace").Create(suite.ctx, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-volume-secret",
			Labels: map[string]string{
				common.NuclioResourceLabelKeyFunctionName: function.Name,
				common.NuclioResourceLabelKeyProjectName:  function.Labels["nuclio.io/project-name"],
				common.NuclioResourceLabelKeyVolumeName:   volumeName,
			},
			CreationTimestamp: metav1.Time{
				Time: time.Now(),
			},
		},
	}, metav1.CreateOptions{})
	suite.Require().NoError(err)

	// logs are spammy, let them
	prevLevel := suite.logger.(*nucliozap.NuclioZap).GetLevel()
	suite.logger.(*nucliozap.NuclioZap).SetLevel(nucliozap.InfoLevel)
	defer suite.logger.(*nucliozap.NuclioZap).SetLevel(prevLevel)

	suite.client.SetPlatformConfigurationProvider(&mockedPlatformConfigurationProvider{
		platformConfiguration: &platformconfig.Config{
			Kube: platformconfig.PlatformKubeConfig{
				DefaultHTTPIngressHostTemplate: "{{ .ResourceName }}-{{ .ProjectName }}.test-nuclio.com",
			},
		},
	})

	// "create" the ingress
	ingressInstance, err := suite.client.createOrUpdateIngress(suite.ctx, functionLabels, &function)
	suite.Require().NoError(err)
	suite.Require().NotNil(ingressInstance)

	// "create" the deployment
	deploymentInstance, err := suite.client.createOrUpdateDeployment(suite.ctx,
		functionLabels,
		"image-pull-secret-str",
		&function)
	suite.Require().NoError(err)
	suite.Require().NotNil(deploymentInstance)

	// make sure no changes were applied for 1000 times of re-apply deployment.
	for i := 0; i < 1000; i++ {

		// "update" the ingress
		updatedIngressInstance, err := suite.client.createOrUpdateIngress(suite.ctx, functionLabels, &function)
		suite.Require().NoError(err)
		suite.Require().NotNil(updatedIngressInstance)

		// ensure no changes
		suite.Require().Empty(cmp.Diff(ingressInstance, updatedIngressInstance))

		// "update" the deployment
		updatedDeploymentInstance, err := suite.client.createOrUpdateDeployment(suite.ctx,
			functionLabels,
			"image-pull-secret-str",
			&function)
		suite.Require().NoError(err)
		suite.Require().NotNil(updatedDeploymentInstance)

		// make sure access key is still present in the function spec volume options
		suite.Require().Contains(function.Spec.Volumes[0].Volume.VolumeSource.FlexVolume.Options, "accessKey")

		// make sure flex volume doesn't contain access key
		for _, volume := range updatedDeploymentInstance.Spec.Template.Spec.Volumes {
			if volume.Name == volumeName {
				suite.Require().NotContains(volume.FlexVolume.Options, "accessKey")
				break
			}
		}

		// ensure no changes
		suite.Require().Empty(cmp.Diff(deploymentInstance, updatedDeploymentInstance))
	}
}

func (suite *lazyTestSuite) TestNoTriggers() {
	ingressMeta := metav1.ObjectMeta{}
	ingressSpec := networkingv1.IngressSpec{}

	// function instance has no triggers
	functionInstance := nuclioio.NuclioFunction{}
	functionInstance.Name = "func-name"
	functionInstance.Spec.Triggers = map[string]functionconfig.Trigger{}

	// get labels
	labels := map[string]string{
		"nuclio.io/function-version": "latest",
	}

	err := suite.client.populateIngressConfig(suite.ctx,
		labels,
		&functionInstance,
		&ingressMeta,
		&ingressSpec)

	suite.Require().NoError(err)
	suite.Require().Len(ingressSpec.Rules, 0)
}

func (suite *lazyTestSuite) TestTriggerDefinedNoIngresses() {
	ingressMeta := metav1.ObjectMeta{}
	ingressSpec := networkingv1.IngressSpec{}

	// function instance has no triggers
	functionInstance := nuclioio.NuclioFunction{}
	functionInstance.Name = "func-name"
	functionInstance.Spec.Triggers = map[string]functionconfig.Trigger{
		"mh": {
			Kind: "http",
		},
	}

	// get labels
	labels := map[string]string{
		"nuclio.io/function-version": "latest",
	}

	// ensure no ingress rules are populated
	err := suite.client.populateIngressConfig(suite.ctx,
		labels,
		&functionInstance,
		&ingressMeta,
		&ingressSpec)
	suite.Require().NoError(err)
	suite.Require().Len(ingressSpec.Rules, 0)
}

func (suite *lazyTestSuite) TestScaleToZeroSpecificAnnotations() {
	suite.client.SetPlatformConfigurationProvider(&mockedPlatformConfigurationProvider{
		platformConfiguration: &platformconfig.Config{
			ScaleToZero: platformconfig.ScaleToZero{
				HTTPTriggerIngressAnnotations: map[string]string{
					"something": "added",
				},
			},
		},
	})

	zero := 0
	one := 1
	ingressMeta := metav1.ObjectMeta{}
	functionInstance := &nuclioio.NuclioFunction{}
	functionInstance.Spec.MinReplicas = &zero
	functionInstance.Spec.MaxReplicas = &one
	functionInstance.Name = "func-name"
	functionInstance.Spec.Triggers = map[string]functionconfig.Trigger{
		"http": functionconfig.GetDefaultHTTPTrigger(),
	}

	functionLabels := suite.client.getFunctionLabels(functionInstance)
	err := suite.client.populateIngressConfig(suite.ctx,
		functionLabels,
		functionInstance,
		&ingressMeta,
		&networkingv1.IngressSpec{})
	suite.Require().NoError(err)
	suite.Require().Equal("added", ingressMeta.Annotations["something"])
}

func (suite *lazyTestSuite) TestTriggerDefinedMultipleIngresses() {
	ingressMeta := metav1.ObjectMeta{}
	ingressSpec := networkingv1.IngressSpec{}

	annotations := map[string]string{
		"a1": "v1",
		"a2": "v2",
	}

	// function instance has no triggers
	functionInstance := nuclioio.NuclioFunction{}
	functionInstance.Name = "func-name"
	functionInstance.Namespace = "func-namespace"
	functionInstance.Spec.Triggers = map[string]functionconfig.Trigger{
		"mh": {
			Kind:        "http",
			Annotations: annotations,
			Attributes: map[string]interface{}{
				"ingresses": map[string]interface{}{
					"1": map[string]interface{}{
						"host": "host1",
						"paths": []string{
							"constant-value-1",
						},
					},
					"2": map[string]interface{}{
						"host": "host2",
						"paths": []string{
							"constant-value-2",
							"/{{.Namespace}}/{{.Name}}/{{.Version}}/wat",
						},
					},
					"3": map[string]interface{}{
						"host": "host3",
						"paths": []string{
							"{{.Name}}/{{.Version}}",
						},
					},
					"4": map[string]interface{}{
						"host": "host4",
						"paths": []string{
							"constant-value-3",
							"/constant-value-4",
						},
					},
				},
			},
		},
	}

	// get labels
	labels := map[string]string{
		"nuclio.io/function-version": "latest",
	}

	err := suite.client.populateIngressConfig(suite.ctx,
		labels,
		&functionInstance,
		&ingressMeta,
		&ingressSpec)

	// verify annotations
	suite.Require().Equal(annotations, ingressMeta.Annotations)

	suite.Require().NoError(err)
	suite.Require().Len(ingressSpec.Rules, 4)

	// get first host - expect single path
	rule := suite.getIngressRuleByHost(ingressSpec.Rules, "host1")
	suite.Require().Equal("/constant-value-1", rule.HTTP.Paths[0].Path)

	// get second host - expect constant and formatted path
	rule = suite.getIngressRuleByHost(ingressSpec.Rules, "host2")
	suite.Require().Equal("/constant-value-2", rule.HTTP.Paths[0].Path)
	suite.Require().Equal("/func-namespace/func-name/latest/wat", rule.HTTP.Paths[1].Path)

	// get third host - expect single formatted path
	rule = suite.getIngressRuleByHost(ingressSpec.Rules, "host3")
	suite.Require().Equal("/func-name/latest", rule.HTTP.Paths[0].Path)

	// get fourth host - expect two constants
	rule = suite.getIngressRuleByHost(ingressSpec.Rules, "host4")
	suite.Require().Equal("/constant-value-3", rule.HTTP.Paths[0].Path)
	suite.Require().Equal("/constant-value-4", rule.HTTP.Paths[1].Path)
}

func (suite *lazyTestSuite) TestPlatformServicePorts() {

	// configuration with no ports
	servicePorts := suite.client.getServicePortsFromPlatform(&platformconfig.Config{})
	suite.Require().Len(servicePorts, 0)

	// configuration with prometheus pull
	servicePorts = suite.client.getServicePortsFromPlatform(&platformconfig.Config{
		Metrics: platformconfig.Metrics{
			Sinks: map[string]platformconfig.MetricSink{
				"pp": {
					Kind: "prometheusPull",
				},
			},
			Functions: []string{"pp"},
		},
	})
	suite.Require().Len(servicePorts, 1)
	suite.Require().Equal(servicePorts[0].Name, containerMetricPortName)
	suite.Require().Equal(servicePorts[0].Port, int32(containerMetricPort))

	// ensure metric port
	toServicePorts := suite.client.ensureServicePortsExist([]v1.ServicePort{
		{
			Name:     ContainerHTTPPortName,
			Port:     int32(abstract.FunctionContainerHTTPPort),
			NodePort: 12345,
		},
	}, []v1.ServicePort{
		{
			Name: containerMetricPortName,
			Port: int32(containerMetricPort),
		},
	})

	// should be added
	suite.Require().Len(toServicePorts, 2)

	toServicePorts = suite.client.ensureServicePortsExist([]v1.ServicePort{
		{
			Name:     ContainerHTTPPortName,
			Port:     int32(abstract.FunctionContainerHTTPPort),
			NodePort: 12345,
		},
	}, []v1.ServicePort{
		{
			Name: containerMetricPortName,
			Port: int32(containerMetricPort),
		},
	})

	// should not be added
	suite.Require().Len(toServicePorts, 2)
}

func (suite *lazyTestSuite) TestEnrichDeploymentFromPlatformConfiguration() {
	suite.client.SetPlatformConfigurationProvider(&mockedPlatformConfigurationProvider{
		platformConfiguration: &platformconfig.Config{
			FunctionAugmentedConfigs: []platformconfig.LabelSelectorAndConfig{
				{
					LabelSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"nuclio.io/class": "function",
						},
					},
					FunctionConfig: functionconfig.Config{},
					Kubernetes:     platformconfig.Kubernetes{},
				},
				{
					LabelSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"nuclio.io/class": "function",
						},
					},
					FunctionConfig: functionconfig.Config{},
					Kubernetes: platformconfig.Kubernetes{
						Deployment: &appsv1.Deployment{
							Spec: appsv1.DeploymentSpec{
								Paused: true,
							},
						},
					},
				},
				{
					LabelSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"nuclio.io/class": "notfunction",
						},
					},
					FunctionConfig: functionconfig.Config{},
					Kubernetes: platformconfig.Kubernetes{
						Deployment: &appsv1.Deployment{
							Spec: appsv1.DeploymentSpec{
								Template: v1.PodTemplateSpec{
									Spec: v1.PodSpec{
										ServiceAccountName: "pleasedont",
									},
								},
							},
						},
					},
				},
				{
					LabelSelector:  metav1.LabelSelector{},
					FunctionConfig: functionconfig.Config{},
					Kubernetes: platformconfig.Kubernetes{
						Deployment: &appsv1.Deployment{
							Spec: appsv1.DeploymentSpec{
								Strategy: appsv1.DeploymentStrategy{
									Type:          appsv1.RecreateDeploymentStrategyType,
									RollingUpdate: nil,
								},
							},
						},
					},
				},
			},
		},
	})

	functionInstance := nuclioio.NuclioFunction{}
	functionInstance.Name = "func-name"
	functionInstance.Namespace = "func-namespace"
	functionInstance.Labels = map[string]string{
		"nuclio.io/class": "function",
	}

	deployment := appsv1.Deployment{}
	err := suite.client.enrichDeploymentFromPlatformConfiguration(&functionInstance,
		&deployment,
		updateDeploymentResourceMethod)
	suite.Require().NoError(err)
	suite.Require().Equal(deployment.Spec.Strategy.Type, appsv1.RecreateDeploymentStrategyType)
	suite.Require().Equal(deployment.Spec.Template.Spec.ServiceAccountName, "")
	suite.Require().True(deployment.Spec.Paused)
}

func (suite *lazyTestSuite) TestFastFailOnAutoScalerEvents() {
	namespace := "some-namespace"
	podName := "my-pod"

	for _, testCase := range []struct {
		name          string
		event         v1.Event
		expectedError bool
	}{
		{
			name: "PodScalingUp",
			event: v1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name: "PodScalingUpEvent",
				},
				InvolvedObject: v1.ObjectReference{
					Name: podName,
				},
				Source: v1.EventSource{
					Component: "cluster-autoscaler",
				},
				Reason: "TriggeredScaleUp",
			},
			expectedError: false,
		},
		{
			name: "PodScalingDown",
			event: v1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name: "PodScalingDownEvent",
				},
				InvolvedObject: v1.ObjectReference{
					Name: podName,
				},
				Source: v1.EventSource{
					Component: "cluster-autoscaler",
				},
				Reason: "ScaleDown",
			},
			expectedError: true,
		},
	} {
		suite.Run(testCase.name, func() {

			pod := v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              podName,
					Namespace:         namespace,
					CreationTimestamp: metav1.Now(),
				},
				Status: v1.PodStatus{
					Phase: v1.PodPending,
					Conditions: []v1.PodCondition{
						{
							Reason: "Unschedulable",
						},
					},
				},
			}
			podsList := v1.PodList{
				Items: []v1.Pod{pod},
			}

			_, err := suite.client.kubeClientSet.CoreV1().Events(namespace).Create(suite.ctx, &testCase.event, metav1.CreateOptions{})
			suite.Require().NoError(err)

			// call resolveFailFast
			err = suite.client.resolveFailFast(suite.ctx, &podsList, time.Now())
			if testCase.expectedError {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
			}

			err = suite.client.kubeClientSet.CoreV1().Events(namespace).Delete(suite.ctx, testCase.event.Name, metav1.DeleteOptions{})
			suite.Require().NoError(err)
		})
	}
}

func (suite *lazyTestSuite) TestResolveAutoScaleMetricSpec() {

	resourceTargetValue := 60
	externalTargetValue := 100
	podTargetValue := *apiresource.NewQuantity(
		200,
		apiresource.DecimalSI,
	)

	functionInstance := &nuclioio.NuclioFunction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "func-name",
			Namespace: "func-namespace",
		},
		Spec: functionconfig.Spec{
			AutoScaleMetrics: []functionconfig.AutoScaleMetric{
				{
					ScaleResource: functionconfig.ScaleResource{
						MetricName: string(v1.ResourceMemory),
						Threshold:  resourceTargetValue,
					},
					SourceType:  autosv2.ResourceMetricSourceType,
					DisplayType: functionconfig.AutoScaleMetricTypePercentage,
				},
				{
					ScaleResource: functionconfig.ScaleResource{
						MetricName: "custom-metric",
						Threshold:  externalTargetValue,
					},
					SourceType:  autosv2.ExternalMetricSourceType,
					DisplayType: functionconfig.AutoScaleMetricTypeInt,
				},
			},
			CustomScalingMetricSpecs: []autosv2.MetricSpec{
				{
					Pods: &autosv2.PodsMetricSource{
						Metric: autosv2.MetricIdentifier{
							Name: "another-custom-metric",
						},
						Target: autosv2.MetricTarget{
							Type:         autosv2.AverageValueMetricType,
							AverageValue: &podTargetValue,
						},
					},
				},
			},
		},
	}
	resolvedMetricSpec, err := suite.client.resolveMetricSpecs(functionInstance)
	suite.Require().NoError(err)
	suite.Require().Equal(len(resolvedMetricSpec), 3)

	externalQuantity, err := apiresource.ParseQuantity(strconv.Itoa(externalTargetValue))
	suite.Require().NoError(err)

	for _, metricSpec := range resolvedMetricSpec {
		switch metricSpec.Type {
		case autosv2.ResourceMetricSourceType:

			// TargetAverageUtilization
			suite.Require().Equal(*metricSpec.Resource.Target.AverageUtilization, int32(resourceTargetValue))

		case autosv2.ExternalMetricSourceType:
			suite.Require().True(metricSpec.External.Target.Value.Equal(externalQuantity))

		case autosv2.PodsMetricSourceType:
			suite.Require().True(metricSpec.Pods.Target.AverageValue.Equal(podTargetValue))
		}
	}
}

func (suite *lazyTestSuite) getIngressRuleByHost(rules []networkingv1.IngressRule, host string) *networkingv1.IngressRule {
	for _, rule := range rules {
		if rule.Host == host {
			return &rule
		}
	}

	suite.Failf("Could not find host in rules: %s", host)
	return nil
}

func TestLazyTestSuite(t *testing.T) {
	suite.Run(t, new(lazyTestSuite))
}
