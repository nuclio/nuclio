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
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/annotations"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform/abstract"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/clients/kube"
	nuclioiofake "github.com/nuclio/nuclio/pkg/platform/kube/clients/nuclio/clientset/versioned/fake"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	processorconfig "github.com/nuclio/nuclio/pkg/processor/config"

	"dario.cat/mergo"
	"github.com/google/go-cmp/cmp"
	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	autosv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
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
	logger        logger.Logger
	client        *lazyClient
	kubeClientSet *fake.Clientset
	ctx           context.Context
}

func (suite *lazyTestSuite) SetupTest() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	suite.kubeClientSet = fake.NewSimpleClientset()
	// create client
	lazyClientInstance, err := NewLazyClient(suite.logger,
		kube.NewClientWithRetryFromClient(suite.kubeClientSet),
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
	functionInstance := suite.getFunctionInstanceWithDefaultProbes("func-name")
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

func (suite *lazyTestSuite) TestRuntimeClassName() {
	runtimeClassName := "nvidia"
	functionInstance := suite.getFunctionInstanceWithDefaultProbes("func-name")
	functionInstance.Spec.RuntimeClassName = &runtimeClassName

	resources, err := suite.client.CreateOrUpdate(suite.ctx, functionInstance, "")
	suite.Require().NoError(err)
	suite.Require().NotEmpty(resources)
	deployment, err := resources.Deployment()
	suite.Require().NoError(err)

	suite.Require().Equal(&runtimeClassName, deployment.Spec.Template.Spec.RuntimeClassName)
}

func (suite *lazyTestSuite) TestRuntimeClassNameNil() {
	functionInstance := suite.getFunctionInstanceWithDefaultProbes("func-name")

	resources, err := suite.client.CreateOrUpdate(suite.ctx, functionInstance, "")
	suite.Require().NoError(err)
	suite.Require().NotEmpty(resources)
	deployment, err := resources.Deployment()
	suite.Require().NoError(err)

	suite.Require().Nil(deployment.Spec.Template.Spec.RuntimeClassName)
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
				"a":                               "c",
				common.NuclioAnnotationKeyVersion: common.GetNuclioVersion(),
			},
		},
		{
			name: "sanity-no-override-empty-value",
			functionIngressAnnotations: map[string]string{
				"a": "",
			},
			expectedFunctionIngressAnnotations: map[string]string{
				"a":                               "",
				common.NuclioAnnotationKeyVersion: common.GetNuclioVersion(),
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
				ingressAnnotations[common.NuclioAnnotationKeyVersion] = common.GetNuclioVersion()
				return ingressAnnotations
			}(),
		},
	} {
		suite.Run(testCase.name, func() {
			function := suite.generateFunctionWithIngress(testCase.name, "", testCase.functionIngressAnnotations)
			functionLabels := suite.client.getFunctionLabels(&function)
			functionLabels[common.NuclioResourceLabelKeyFunctionName] = function.Name

			// create the ingress
			ingressInstance, err := suite.client.createOrUpdateIngress(suite.ctx, functionLabels, &function)
			suite.Require().NoError(err)
			suite.Require().NotNil(ingressInstance)
			suite.Require().NotEmpty(ingressInstance.Annotations)

			// make sure user function annotations exists
			suite.Require().Equal(testCase.expectedFunctionIngressAnnotations,
				ingressInstance.Annotations)
		})
	}
}

func (suite *lazyTestSuite) TestEnrichIngressWithDefaultIngressClassName() {
	defaultIngressClassName := "my-ingress-class"
	suite.client.SetPlatformConfigurationProvider(&mockedPlatformConfigurationProvider{
		platformConfiguration: &platformconfig.Config{
			Kube: platformconfig.PlatformKubeConfig{
				DefaultHTTPIngressClassName: defaultIngressClassName,
			},
		},
	})

	function := suite.generateFunctionWithIngress("function-name", "", nil)
	functionLabels := suite.client.getFunctionLabels(&function)
	functionLabels[common.NuclioResourceLabelKeyFunctionName] = function.Name

	// "create the ingress
	ingressInstance, err := suite.client.createOrUpdateIngress(suite.ctx, functionLabels, &function)
	suite.Require().NoError(err)
	suite.Require().NotNil(ingressInstance)
	suite.Require().Equal(defaultIngressClassName, *ingressInstance.Spec.IngressClassName)
}

func (suite *lazyTestSuite) TestEnrichIngressTLS() {
	sslRedirectAnnotation := annotations.NginxSSLRedirect

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
			function := suite.generateFunctionWithIngress(testCase.name, host, nil)
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
	sslRedirectAnnotation := annotations.NginxSSLRedirect
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
				"hostTemplate": common.DefaultIngressHostTemplate,
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
				common.NuclioResourceLabelKeyProjectName: common.GenerateRandomString(60, common.SmallLettersAndNumbers),
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
			LivenessProbe:  platformconfig.DefaultLivenessProbeConfiguration,
			ReadinessProbe: platformconfig.DefaultReadinessProbeConfiguration,
		},
	}
	functionLabels := suite.client.getFunctionLabels(&function)
	functionLabels[common.NuclioResourceLabelKeyFunctionName] = function.Name

	// mock volume secret creation
	_, err := suite.kubeClientSet.CoreV1().Secrets("test-namespace").Create(suite.ctx, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-volume-secret",
			Labels: map[string]string{
				common.NuclioResourceLabelKeyFunctionName: function.Name,
				common.NuclioResourceLabelKeyProjectName:  function.Labels[common.NuclioResourceLabelKeyProjectName],
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
		suite.Require().Contains(function.Spec.Volumes[0].Volume.FlexVolume.Options, "accessKey")

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
		common.NuclioLabelKeyFunctionVersion: "latest",
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
		common.NuclioLabelKeyFunctionVersion: "latest",
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

// TestFunctionAuthenticationEnabled verifies the auth-proxy gate: the platform-wide feature flag AND a
// function-level authenticationMode (other than none/unset) on the HTTP trigger must both hold.
func (suite *lazyTestSuite) TestFunctionAuthenticationEnabled() {
	for _, testCase := range []struct {
		name                          string
		functionAuthenticationEnabled bool
		authenticationMode            string
		noHTTPTrigger                 bool
		expected                      bool
	}{
		{
			name:                          "disabled when feature flag is off",
			functionAuthenticationEnabled: false,
			authenticationMode:            string(auth.AuthenticationModeBasicAuth),
		},
		{
			name:                          "disabled when authenticationMode is none",
			functionAuthenticationEnabled: true,
			authenticationMode:            string(auth.AuthenticationModeNone),
		},
		{
			name:                          "disabled when authenticationMode is unset",
			functionAuthenticationEnabled: true,
		},
		{
			name:                          "disabled when there is no HTTP trigger",
			functionAuthenticationEnabled: true,
			authenticationMode:            string(auth.AuthenticationModeBasicAuth),
			noHTTPTrigger:                 true,
		},
		{
			name:                          "enabled for basicAuth",
			functionAuthenticationEnabled: true,
			authenticationMode:            string(auth.AuthenticationModeBasicAuth),
			expected:                      true,
		},
		{
			name:                          "enabled for api",
			functionAuthenticationEnabled: true,
			authenticationMode:            string(auth.AuthenticationModeAPI),
			expected:                      true,
		},
		{
			name:                          "enabled for browser",
			functionAuthenticationEnabled: true,
			authenticationMode:            string(auth.AuthenticationModeBrowser),
			expected:                      true,
		},
	} {
		suite.Run(testCase.name, func() {
			suite.client.SetPlatformConfigurationProvider(&mockedPlatformConfigurationProvider{
				platformConfiguration: &platformconfig.Config{
					Authentication: &platformconfig.Authentication{
						FunctionAuthenticationEnabled: testCase.functionAuthenticationEnabled,
					},
				},
			})

			functionInstance := &nuclioio.NuclioFunction{}
			if !testCase.noHTTPTrigger {
				httpTrigger := functionconfig.GetDefaultHTTPTrigger()
				httpTrigger.Attributes = map[string]interface{}{
					auth.AttributeAuthenticationMode: testCase.authenticationMode,
				}
				functionInstance.Spec.Triggers = map[string]functionconfig.Trigger{
					"http": httpTrigger,
				}
			}

			suite.Require().Equal(testCase.expected, suite.client.functionAuthenticationEnabled(functionInstance))
		})
	}
}

// TestInjectAuthProxySidecar verifies the auth-proxy sidecar is appended with the expected image/args/ports,
// and that the config-restore env var is only set for basicAuth mode (api/browser have no credentials to
// restore).
func (suite *lazyTestSuite) TestInjectAuthProxySidecar() {
	for _, testCase := range []struct {
		name                string
		authenticationMode  auth.AuthenticationMode
		expectedRestoreEnvs []v1.EnvVar
	}{
		{
			name:               "basicAuth restores config from the mounted secret",
			authenticationMode: auth.AuthenticationModeBasicAuth,
			expectedRestoreEnvs: []v1.EnvVar{
				{Name: common.RestoreConfigFromSecretEnvVar, Value: "true"},
			},
		},
		{
			name:               "api mode does not need to restore anything",
			authenticationMode: auth.AuthenticationModeAPI,
		},
	} {
		suite.Run(testCase.name, func() {
			suite.client.SetPlatformConfigurationProvider(&mockedPlatformConfigurationProvider{
				platformConfiguration: &platformconfig.Config{
					Authentication: &platformconfig.Authentication{
						FunctionAuthenticationEnabled: true,
						SidecarImage:                  "nuclio/auth-proxy:latest",
						AuthURL:                       "http://auth.example.com",
						SignInURL:                     "http://signin.example.com",
						AuthKind:                      auth.KindIguazio,
					},
				},
			})

			httpTrigger := functionconfig.GetDefaultHTTPTrigger()
			httpTrigger.Attributes = map[string]interface{}{
				auth.AttributeAuthenticationMode: string(testCase.authenticationMode),
			}
			functionInstance := &nuclioio.NuclioFunction{}
			functionInstance.Name = "func-name"
			functionInstance.Spec.Triggers = map[string]functionconfig.Trigger{
				"http": httpTrigger,
			}

			deploymentSpec := &appsv1.DeploymentSpec{
				Template: v1.PodTemplateSpec{
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{Name: common.FunctionContainerName},
						},
					},
				},
			}

			suite.client.injectAuthProxySidecar(suite.ctx, functionInstance, deploymentSpec, nil)

			suite.Require().Len(deploymentSpec.Template.Spec.Containers, 2)
			sidecar := deploymentSpec.Template.Spec.Containers[1]
			suite.Require().Equal(abstract.AuthProxySidecarContainerName, sidecar.Name)
			suite.Require().Equal("nuclio/auth-proxy:latest", sidecar.Image)
			suite.Require().Contains(sidecar.Args, "--mode=reverseProxy")
			suite.Require().Contains(sidecar.Args, fmt.Sprintf("--listen-ports=%d", abstract.FunctionContainerHTTPPort))
			suite.Require().Contains(sidecar.Args,
				fmt.Sprintf("--upstream-url=http://127.0.0.1:%d", abstract.FunctionContainerHTTPLoopbackPort))
			suite.Require().Contains(sidecar.Args, fmt.Sprintf("--auth-mode=%s", testCase.authenticationMode))
			suite.Require().Contains(sidecar.Args, "--auth-url=http://auth.example.com")
			suite.Require().Contains(sidecar.Args, "--signin-url=http://signin.example.com")
			suite.Require().Contains(sidecar.Args, fmt.Sprintf("--auth-kind=%s", auth.KindIguazio))
			suite.Require().Equal([]v1.ContainerPort{
				{
					Name:          abstract.FunctionContainerHTTPPortName,
					ContainerPort: abstract.FunctionContainerHTTPPort,
					Protocol:      v1.ProtocolTCP,
				},
			}, sidecar.Ports)
			suite.Require().Equal(testCase.expectedRestoreEnvs, sidecar.Env)
		})
	}
}

// TestPopulateSupplementaryContainersInjectsAuthProxyWhenGated verifies the auth-proxy sidecar is only
// appended to the pod when function-level authentication is gated on.
func (suite *lazyTestSuite) TestPopulateSupplementaryContainersInjectsAuthProxyWhenGated() {
	for _, testCase := range []struct {
		name                   string
		functionAuthEnabled    bool
		expectAuthProxySidecar bool
	}{
		{name: "not injected when gate is off", functionAuthEnabled: false, expectAuthProxySidecar: false},
		{name: "injected when gate is on", functionAuthEnabled: true, expectAuthProxySidecar: true},
	} {
		suite.Run(testCase.name, func() {
			suite.client.SetPlatformConfigurationProvider(&mockedPlatformConfigurationProvider{
				platformConfiguration: &platformconfig.Config{
					Authentication: &platformconfig.Authentication{
						FunctionAuthenticationEnabled: testCase.functionAuthEnabled,
					},
				},
			})

			httpTrigger := functionconfig.GetDefaultHTTPTrigger()
			httpTrigger.Attributes = map[string]interface{}{
				auth.AttributeAuthenticationMode: string(auth.AuthenticationModeBasicAuth),
			}
			functionInstance := &nuclioio.NuclioFunction{}
			functionInstance.Spec.Triggers = map[string]functionconfig.Trigger{
				"http": httpTrigger,
			}

			deploymentSpec := &appsv1.DeploymentSpec{
				Template: v1.PodTemplateSpec{
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{Name: common.FunctionContainerName},
						},
					},
				},
			}

			suite.client.populateSupplementaryContainers(suite.ctx, functionInstance, deploymentSpec, nil)

			hasAuthProxy := false
			for _, container := range deploymentSpec.Template.Spec.Containers {
				if container.Name == abstract.AuthProxySidecarContainerName {
					hasAuthProxy = true
				}
			}
			suite.Require().Equal(testCase.expectAuthProxySidecar, hasAuthProxy)
		})
	}
}

// TestPopulateDeploymentContainerRemovesHTTPPortWhenAuthProxyFronts verifies the processor container no
// longer declares the main HTTP port once the auth-proxy sidecar takes over listening on it.
func (suite *lazyTestSuite) TestPopulateDeploymentContainerRemovesHTTPPortWhenAuthProxyFronts() {
	for _, testCase := range []struct {
		name                string
		functionAuthEnabled bool
		expectHTTPPort      bool
	}{
		{name: "keeps HTTP port when gate is off", functionAuthEnabled: false, expectHTTPPort: true},
		{name: "removes HTTP port when gate is on", functionAuthEnabled: true, expectHTTPPort: false},
	} {
		suite.Run(testCase.name, func() {
			suite.client.SetPlatformConfigurationProvider(&mockedPlatformConfigurationProvider{
				platformConfiguration: &platformconfig.Config{
					Authentication: &platformconfig.Authentication{
						FunctionAuthenticationEnabled: testCase.functionAuthEnabled,
					},
				},
			})

			httpTrigger := functionconfig.GetDefaultHTTPTrigger()
			httpTrigger.Attributes = map[string]interface{}{
				auth.AttributeAuthenticationMode: string(auth.AuthenticationModeBasicAuth),
			}
			functionInstance := suite.getFunctionInstanceWithDefaultProbes("func-name")
			functionInstance.Spec.Triggers = map[string]functionconfig.Trigger{
				"http": httpTrigger,
			}

			functionLabels := suite.client.getFunctionLabels(functionInstance)
			container := &v1.Container{}
			suite.client.populateDeploymentContainer(suite.ctx, functionLabels, functionInstance, container)

			hasHTTPPort := false
			for _, port := range container.Ports {
				if port.Name == abstract.FunctionContainerHTTPPortName {
					hasHTTPPort = true
				}
			}
			suite.Require().Equal(testCase.expectHTTPPort, hasHTTPPort)
		})
	}
}

// TestPopulateConfigMapRewritesHTTPTriggerURLToLoopback verifies the processor's HTTP trigger is
// configured to listen on the pod-local loopback (reachable only via the auth-proxy sidecar) once the
// gate is on, and that the function's own Spec is never mutated in the process.
func (suite *lazyTestSuite) TestPopulateConfigMapRewritesHTTPTriggerURLToLoopback() {
	suite.client.SetPlatformConfigurationProvider(&mockedPlatformConfigurationProvider{
		platformConfiguration: &platformconfig.Config{
			Authentication: &platformconfig.Authentication{
				FunctionAuthenticationEnabled: true,
			},
		},
	})

	httpTrigger := functionconfig.GetDefaultHTTPTrigger()
	httpTrigger.Attributes = map[string]interface{}{
		auth.AttributeAuthenticationMode: string(auth.AuthenticationModeBasicAuth),
	}
	functionInstance := &nuclioio.NuclioFunction{}
	functionInstance.Name = "func-name"
	functionInstance.Spec.Triggers = map[string]functionconfig.Trigger{
		"http": httpTrigger,
	}

	functionLabels := suite.client.getFunctionLabels(functionInstance)
	configMap := &v1.ConfigMap{}
	suite.Require().NoError(suite.client.populateConfigMap(functionLabels, functionInstance, configMap))

	reader, err := processorconfig.NewReader()
	suite.Require().NoError(err)

	var writtenConfiguration processor.Configuration
	suite.Require().NoError(reader.Read(strings.NewReader(configMap.Data["processor.yaml"]), &writtenConfiguration))
	suite.Require().Equal(fmt.Sprintf("127.0.0.1:%d", abstract.FunctionContainerHTTPLoopbackPort),
		writtenConfiguration.Config.Spec.Triggers["http"].URL)

	// the original function spec must remain untouched
	suite.Require().Empty(functionInstance.Spec.Triggers["http"].URL)
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
		common.NuclioLabelKeyFunctionVersion: "latest",
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
	suite.Require().Equal(servicePorts[0].Name, abstract.FunctionContainerMetricPortName)
	suite.Require().Equal(servicePorts[0].Port, int32(abstract.FunctionContainerMetricPort))

	// ensure metric port
	toServicePorts := suite.client.ensureServicePortsExist([]v1.ServicePort{
		{
			Name:     abstract.FunctionContainerHTTPPortName,
			Port:     int32(abstract.FunctionContainerHTTPPort),
			NodePort: 12345,
		},
	}, []v1.ServicePort{
		{
			Name: abstract.FunctionContainerMetricPortName,
			Port: int32(abstract.FunctionContainerMetricPort),
		},
	})

	// should be added
	suite.Require().Len(toServicePorts, 2)

	toServicePorts = suite.client.ensureServicePortsExist([]v1.ServicePort{
		{
			Name:     abstract.FunctionContainerHTTPPortName,
			Port:     int32(abstract.FunctionContainerHTTPPort),
			NodePort: 12345,
		},
	}, []v1.ServicePort{
		{
			Name: abstract.FunctionContainerMetricPortName,
			Port: int32(abstract.FunctionContainerMetricPort),
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
							common.NuclioLabelKeyClass: "function",
						},
					},
					FunctionConfig: functionconfig.Config{},
					Kubernetes:     platformconfig.Kubernetes{},
				},
				{
					LabelSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							common.NuclioLabelKeyClass: "function",
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
							common.NuclioLabelKeyClass: "notfunction",
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
		common.NuclioLabelKeyClass: "function",
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

func (suite *lazyTestSuite) TestEnrichProbesInPopulateDeploymentContainer() {
	// Prepare
	for _, testCase := range []struct {
		name             string
		functionInstance *nuclioio.NuclioFunction
	}{
		{
			name:             "test-enrich-probes-function",
			functionInstance: suite.getFunctionInstanceWithDefaultProbes("test-enrich-probes-function"),
		}, {
			// Test backward compatibility for functions created with controller versions < v1.14.5, where probes are nil.
			// After an upgrade, the new controller will access these fields.
			name:             "test-empty-probes-function",
			functionInstance: &nuclioio.NuclioFunction{},
		},
	} {
		suite.Run(testCase.name, func() {
			testFunctionLabels := suite.client.getFunctionLabels(testCase.functionInstance)
			testContainer := &v1.Container{}

			// Populate the container with configuration from the function instance
			suite.client.populateDeploymentContainer(suite.ctx, testFunctionLabels, testCase.functionInstance, testContainer)

			// Verify readinessProbe is populated with default values
			suite.Require().Equal(testContainer.ReadinessProbe.InitialDelaySeconds, platformconfig.DefaultReadinessProbeConfiguration.InitialDelaySeconds)
			suite.Require().Equal(testContainer.ReadinessProbe.TimeoutSeconds, platformconfig.DefaultReadinessProbeConfiguration.TimeoutSeconds)
			suite.Require().Equal(testContainer.ReadinessProbe.PeriodSeconds, platformconfig.DefaultReadinessProbeConfiguration.PeriodSeconds)
			suite.Require().Equal(testContainer.ReadinessProbe.FailureThreshold, platformconfig.DefaultReadinessProbeConfiguration.FailureThreshold)

			// Verify LivenessProbe is populated with default values
			suite.Require().Equal(testContainer.LivenessProbe.InitialDelaySeconds, platformconfig.DefaultLivenessProbeConfiguration.InitialDelaySeconds)
			suite.Require().Equal(testContainer.LivenessProbe.TimeoutSeconds, platformconfig.DefaultLivenessProbeConfiguration.TimeoutSeconds)
			suite.Require().Equal(testContainer.LivenessProbe.PeriodSeconds, platformconfig.DefaultLivenessProbeConfiguration.PeriodSeconds)
			suite.Require().Equal(testContainer.LivenessProbe.FailureThreshold, platformconfig.DefaultLivenessProbeConfiguration.FailureThreshold)
		})
	}
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

			_, err := suite.kubeClientSet.CoreV1().Events(namespace).Create(suite.ctx, &testCase.event, metav1.CreateOptions{})
			suite.Require().NoError(err)

			// call resolveFailFast
			_, err = suite.client.resolveFailFast(suite.ctx, &podsList, time.Now())
			if testCase.expectedError {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
			}

			err = suite.kubeClientSet.CoreV1().Events(namespace).Delete(suite.ctx, testCase.event.Name, metav1.DeleteOptions{})
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

// TestCronTriggerExecFormNoShellInjection is the regression test for GHSA-v5px-423j-pf7p.
// It exercises the cron-trigger CronJob spec generation and asserts that the resulting
// container runs `curl` directly (exec form) with user-supplied header keys/values and
// event body passed as discrete argv entries — never as parts of a shell command.
func (suite *lazyTestSuite) TestCronTriggerExecFormNoShellInjection() {
	for _, testCase := range []struct {
		name             string
		headers          map[string]interface{}
		body             string
		assertions       func(args []string)
		assertNoDataFlag bool
	}{
		{
			name: "header_key_with_quote_does_not_break_shell",

			// the advisory's Path-A payload: a header key containing `"` and shell
			// metacharacters. In exec form, this entire string is one argv entry of
			// curl after `--header`; the shell never sees it.
			headers: map[string]interface{}{
				`X-Inject"; echo PWNED; echo "`: "marker",
			},
			assertions: func(args []string) {
				suite.Require().Contains(args,
					`X-Inject"; echo PWNED; echo "`+`: marker`,
					"header key+value should appear as a single literal argv entry")
			},
		},
		{
			name: "body_with_command_substitution_is_literal",

			// the advisory's Path-B payload: `$()` survived strconv.Quote and was
			// expanded by /bin/sh. In exec form, no shell sees it.
			body: "$(id)",
			assertions: func(args []string) {
				suite.Require().Contains(args, "$(id)",
					"body should appear as a literal argv entry")
				suite.Require().Contains(args, "--data-raw",
					"body should be passed via --data-raw, not --data")
				suite.Require().NotContains(args, "--data",
					"--data interprets leading '@' as file-load; must use --data-raw")
			},
		},
		{
			name: "body_starting_with_at_is_not_file_load",

			// curl `--data` treats a body starting with `@` as "load from file" —
			// using `--data-raw` removes that primitive.
			body: "@/etc/passwd",
			assertions: func(args []string) {
				suite.Require().Contains(args, "@/etc/passwd",
					"body should appear literally; --data-raw must prevent file load")
				suite.Require().Contains(args, "--data-raw")
				suite.Require().NotContains(args, "--data")
			},
		},
		{
			name:             "empty_body_omits_data_flag",
			body:             "",
			assertNoDataFlag: true,
			assertions: func(args []string) {
				suite.Require().NotContains(args, "--data-raw")
				suite.Require().NotContains(args, "--data")
			},
		},
		{
			name: "json_body_is_compacted",
			body: "{\n  \"a\": 1,\n  \"b\": 2\n}",
			assertions: func(args []string) {
				suite.Require().Contains(args, `{"a":1,"b":2}`,
					"valid JSON body should be compacted before being passed to curl")
			},
		},
		{
			name: "non_json_body_passed_through_unchanged",
			body: "not json",
			assertions: func(args []string) {
				suite.Require().Contains(args, "not json",
					"non-JSON body should be passed through unchanged")
			},
		},
		{
			name: "headers_sorted_for_deterministic_order",

			// use a custom prefix so the default X-Nuclio-* headers don't sneak into
			// the order assertion
			headers: map[string]interface{}{
				"Q-Zeta":  "z",
				"Q-Alpha": "a",
				"Q-Mu":    "m",
			},
			assertions: func(args []string) {
				userHeaderArgs := suite.collectHeaderArgs(args, "Q-")
				suite.Require().Equal([]string{
					"Q-Alpha: a",
					"Q-Mu: m",
					"Q-Zeta: z",
				}, userHeaderArgs, "user-supplied headers must be sorted by key")
			},
		},
	} {
		suite.Run(testCase.name, func() {
			suite.setKubeCronTriggerMode()

			triggerAttributes := map[string]interface{}{
				"schedule": "*/1 * * * *",
				"event": map[string]interface{}{
					"headers": testCase.headers,
					"body":    testCase.body,
				},
			}
			cronJob := suite.deployFunctionWithCronTrigger("test-func", triggerAttributes)

			container := cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0]

			// the exec-form invariant: no shell anywhere.
			suite.Require().Equal([]string{"curl"}, container.Command,
				"container must invoke curl directly, never /bin/sh")
			suite.Require().NotContains(container.Args, "/bin/sh",
				"args must not contain /bin/sh — that's the vulnerability")
			suite.Require().NotContains(container.Args, "-c",
				"args must not contain -c — that's the shell-evaluation flag")

			// default headers always emitted
			suite.Require().Contains(container.Args, "X-Nuclio-Invoke-Trigger: cron")
			suite.Require().Contains(container.Args, "X-Nuclio-Target: test-func")

			testCase.assertions(container.Args)
		})
	}
}

// setKubeCronTriggerMode swaps the suite's platform configuration to one that creates
// cron triggers as k8s CronJobs (the path the security fix lives in).
func (suite *lazyTestSuite) setKubeCronTriggerMode() {
	platformConfiguration, err := platformconfig.NewPlatformConfig("")
	suite.Require().NoError(err)
	platformConfiguration.CronTriggerCreationMode = platformconfig.KubeCronTriggerCreationMode
	suite.client.SetPlatformConfigurationProvider(&mockedPlatformConfigurationProvider{
		platformConfiguration: platformConfiguration,
	})
}

// deployFunctionWithCronTrigger creates a NuclioFunction with a single cron trigger,
// runs the reconciliation path, and returns the resulting k8s CronJob.
func (suite *lazyTestSuite) deployFunctionWithCronTrigger(
	functionName string,
	triggerAttributes map[string]interface{}) *batchv1.CronJob {

	functionInstance := suite.getFunctionInstanceWithDefaultProbes(functionName)

	functionInstance.Spec.Triggers = map[string]functionconfig.Trigger{
		"cron-trigger": {
			Kind:       "cron",
			Name:       "cron-trigger",
			Attributes: triggerAttributes,
		},
	}

	resources, err := suite.client.CreateOrUpdate(suite.ctx, functionInstance, "")
	suite.Require().NoError(err)

	cronJobs, err := resources.CronJobs()
	suite.Require().NoError(err)
	suite.Require().Len(cronJobs, 1,
		"expected exactly one CronJob from a single cron trigger")

	return cronJobs[0]
}

// collectHeaderArgs returns the values of all `--header <K: V>` pairs in args whose
// key starts with the given prefix, in encounter order.
func (suite *lazyTestSuite) collectHeaderArgs(args []string, keyPrefix string) []string {
	var headerValues []string
	for i := 0; i < len(args)-1; i++ {
		if args[i] != "--header" {
			continue
		}
		if strings.HasPrefix(args[i+1], keyPrefix) {
			headerValues = append(headerValues, args[i+1])
		}
	}
	return headerValues
}

func (suite *lazyTestSuite) generateFunctionWithIngress(functionName, host string, annotations map[string]string) nuclioio.NuclioFunction {
	one := 1
	if host == "" {
		host = "something.com"
	}
	defaultHTTPTrigger := functionconfig.GetDefaultHTTPTrigger()
	defaultHTTPTrigger.Attributes = map[string]interface{}{
		"ingresses": map[string]interface{}{
			"0": map[string]interface{}{
				"host":  host,
				"paths": []string{"/"},
			},
		},
	}
	if annotations != nil {
		defaultHTTPTrigger.Annotations = annotations
	}

	function := nuclioio.NuclioFunction{
		ObjectMeta: metav1.ObjectMeta{
			Name: functionName,
		},
		Spec: functionconfig.Spec{
			Replicas: &one,
			Triggers: map[string]functionconfig.Trigger{
				defaultHTTPTrigger.Name: defaultHTTPTrigger,
			},
		},
	}
	return function
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

func (suite *lazyTestSuite) getFunctionInstanceWithDefaultProbes(funcName string) *nuclioio.NuclioFunction {
	functionInstance := &nuclioio.NuclioFunction{}
	functionInstance.Name = funcName
	functionInstance.Spec.LivenessProbe = platformconfig.DefaultLivenessProbeConfiguration
	functionInstance.Spec.ReadinessProbe = platformconfig.DefaultReadinessProbeConfiguration

	return functionInstance
}

func TestLazyTestSuite(t *testing.T) {
	suite.Run(t, new(lazyTestSuite))
}
