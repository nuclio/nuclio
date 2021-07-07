// +build test_unit

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

package functionres

import (
	"context"
	"testing"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform/abstract"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/google/go-cmp/cmp"
	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
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
	client lazyClient
}

func (suite *lazyTestSuite) SetupTest() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	suite.client.logger = suite.logger

	// use a fake kube client
	suite.client.kubeClientSet = fake.NewSimpleClientset()

	// use the default platform configuration
	defaultPlatformConfiguration, err := platformconfig.NewPlatformConfig("")
	suite.Require().NoError(err)
	suite.client.SetPlatformConfigurationProvider(&mockedPlatformConfigurationProvider{
		platformConfiguration: defaultPlatformConfiguration,
	})
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
	resources, err := suite.client.CreateOrUpdate(context.TODO(), functionInstance, "")
	suite.Require().NoError(err)
	suite.Require().NotEmpty(resources)
	deployment, err := resources.Deployment()
	suite.Require().NoError(err)

	// ensure fields were passed
	deployment.Spec.Template.Spec.NodeName = functionInstance.Spec.NodeName
	deployment.Spec.Template.Spec.NodeSelector = functionInstance.Spec.NodeSelector
	deployment.Spec.Template.Spec.Affinity = functionInstance.Spec.Affinity
}

func (suite *lazyTestSuite) TestNoChanges() {
	one := 1
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

				// we want the created ingress host to be exceed the length limitation
				"nuclio.io/project-name": common.GenerateRandomString(60, common.SmallLettersAndNumbers),
			},
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

	// logs are spammy, let them
	prevLevel := suite.client.logger.(*nucliozap.NuclioZap).GetLevel()
	suite.client.logger.(*nucliozap.NuclioZap).SetLevel(nucliozap.InfoLevel)
	defer suite.client.logger.(*nucliozap.NuclioZap).SetLevel(prevLevel)

	suite.client.SetPlatformConfigurationProvider(&mockedPlatformConfigurationProvider{
		platformConfiguration: &platformconfig.Config{
			Kube: platformconfig.PlatformKubeConfig{
				DefaultHTTPIngressHostTemplate: "{{ .ResourceName }}-{{ .ProjectName }}.test-nuclio.com",
			},
		},
	})

	suite.client.nginxIngressUpdateGracePeriod = 0

	// "create the ingress
	ingressInstance, err := suite.client.createOrUpdateIngress(functionLabels, &function)
	suite.Require().NoError(err)
	suite.Require().NotNil(ingressInstance)

	// "create" the deployment
	deploymentInstance, err := suite.client.createOrUpdateDeployment(functionLabels,
		"image-pull-secret-str",
		&function)
	suite.Require().NoError(err)
	suite.Require().NotNil(deploymentInstance)

	// make sure no changes were applied for 1000 times of re-apply deployment.
	for i := 0; i < 1000; i++ {

		// "update" the ingress
		updatedIngressInstance, err := suite.client.createOrUpdateIngress(functionLabels, &function)
		suite.Require().NoError(err)
		suite.Require().NotNil(updatedIngressInstance)

		// ensure no changes
		suite.Require().Empty(cmp.Diff(ingressInstance, updatedIngressInstance))

		// "update" the deployment
		updatedDeploymentInstance, err := suite.client.createOrUpdateDeployment(functionLabels,
			"image-pull-secret-str",
			&function)
		suite.Require().NoError(err)
		suite.Require().NotNil(updatedDeploymentInstance)

		// ensure no changes
		suite.Require().Empty(cmp.Diff(deploymentInstance, updatedDeploymentInstance))
	}
}

func (suite *lazyTestSuite) TestNoTriggers() {
	ingressMeta := metav1.ObjectMeta{}
	ingressSpec := extv1beta1.IngressSpec{}

	// function instance has no triggers
	functionInstance := nuclioio.NuclioFunction{}
	functionInstance.Name = "func-name"
	functionInstance.Spec.Triggers = map[string]functionconfig.Trigger{}

	// get labels
	labels := map[string]string{
		"nuclio.io/function-version": "latest",
	}

	err := suite.client.populateIngressConfig(labels,
		&functionInstance,
		&ingressMeta,
		&ingressSpec)

	suite.Require().NoError(err)
	suite.Require().Len(ingressSpec.Rules, 0)
}

func (suite *lazyTestSuite) TestTriggerDefinedNoIngresses() {
	ingressMeta := metav1.ObjectMeta{}
	ingressSpec := extv1beta1.IngressSpec{}

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

	err := suite.client.populateIngressConfig(labels,
		&functionInstance,
		&ingressMeta,
		&ingressSpec)

	suite.Require().NoError(err)
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
	err := suite.client.populateIngressConfig(functionLabels,
		functionInstance,
		&ingressMeta,
		&extv1beta1.IngressSpec{})
	suite.Require().NoError(err)
	suite.Require().Equal("added", ingressMeta.Annotations["something"])
}

func (suite *lazyTestSuite) TestTriggerDefinedMultipleIngresses() {
	ingressMeta := metav1.ObjectMeta{}
	ingressSpec := extv1beta1.IngressSpec{}

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

	err := suite.client.populateIngressConfig(labels,
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
							"nuclio.io/class": "apply-me",
						},
					},
					FunctionConfig: functionconfig.Config{},
					Kubernetes:     platformconfig.Kubernetes{},
				},
				{
					LabelSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"nuclio.io/class": "apply-me",
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
							"nuclio.io/class": "dont-apply-me",
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
		"nuclio.io/class": "apply-me",
	}

	deployment := appsv1.Deployment{}
	err := suite.client.enrichDeploymentFromPlatformConfiguration(&functionInstance,
		&deployment,
		updateDeploymentResourceMethod)
	suite.Equal(deployment.Spec.Strategy.Type, appsv1.RecreateDeploymentStrategyType)
	suite.True(deployment.Spec.Paused)
	suite.Equal(deployment.Spec.Template.Spec.ServiceAccountName, "")
	suite.Require().NoError(err)
}

func (suite *lazyTestSuite) getIngressRuleByHost(rules []extv1beta1.IngressRule, host string) *extv1beta1.IngressRule {
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
