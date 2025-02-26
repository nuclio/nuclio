//go:build test_integration && test_kube

/*
Copyright 2024 The Nuclio Authors.

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

package apigateway

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube/ingress"
	kubesuite "github.com/nuclio/nuclio/pkg/platform/kube/test/suite"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
	"k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

type DeployAPIGatewayTestSuite struct {
	kubesuite.KubeTestSuite
}

func (suite *DeployAPIGatewayTestSuite) TestDexAuthMode() {
	functionName := "some-function-name"
	apiGatewayName := "some-api-gateway-name"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	configOauth2ProxyURL := "config-oauth2-url"
	suite.PlatformConfiguration.IngressConfig = platformconfig.IngressConfig{
		Oauth2ProxyURL: configOauth2ProxyURL,
	}
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		createAPIGatewayOptions := suite.CompileCreateAPIGatewayOptions(apiGatewayName, functionName)
		createAPIGatewayOptions.APIGatewayConfig.Spec.AuthenticationMode = ingress.AuthenticationModeOauth2
		err := suite.DeployAPIGateway(createAPIGatewayOptions, func(ingress *networkingv1.Ingress) {
			suite.Require().NotContains(ingress.Annotations, "nginx.ingress.kubernetes.io/auth-signin")
			suite.Require().Contains(ingress.Annotations["nginx.ingress.kubernetes.io/auth-url"], configOauth2ProxyURL)
			suite.Require().Contains(ingress.Annotations["nginx.ingress.kubernetes.io/configuration-snippet"],
				fmt.Sprintf(`proxy_set_header X-Nuclio-Target "%s";`, functionName))
		})
		suite.Require().NoError(err)

		overrideOauth2ProxyURL := "override-oauth2-url"
		createAPIGatewayOptions = suite.CompileCreateAPIGatewayOptions(apiGatewayName, functionName)
		createAPIGatewayOptions.APIGatewayConfig.Spec.AuthenticationMode = ingress.AuthenticationModeOauth2
		createAPIGatewayOptions.APIGatewayConfig.Spec.Authentication = &platform.APIGatewayAuthenticationSpec{
			DexAuth: &ingress.DexAuth{
				Oauth2ProxyURL:               overrideOauth2ProxyURL,
				RedirectUnauthorizedToSignIn: true,
			},
		}
		err = suite.DeployAPIGateway(createAPIGatewayOptions, func(ingress *networkingv1.Ingress) {
			suite.Assert().Contains(ingress.Annotations, "nginx.ingress.kubernetes.io/auth-signin")
			suite.Assert().Contains(ingress.Annotations["nginx.ingress.kubernetes.io/auth-signin"], overrideOauth2ProxyURL)
			suite.Assert().Contains(ingress.Annotations["nginx.ingress.kubernetes.io/auth-url"], overrideOauth2ProxyURL)
		})
		suite.Require().NoError(err)

		return true
	})
}

func (suite *DeployAPIGatewayTestSuite) TestFunctionWithTwoGateways() {
	functionName := "some-function-name"
	apiGatewayName1 := "api-gateway-1"
	apiGatewayName2 := "api-gateway-2"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		// create first api gateway on top of given function
		createAPIGatewayOptions1 := suite.CompileCreateAPIGatewayOptions(apiGatewayName1, functionName)
		createAPIGatewayOptions1.APIGatewayConfig.Spec.AuthenticationMode = ingress.AuthenticationModeNone
		createAPIGatewayOptions1.APIGatewayConfig.Spec.Host = "host1.com"

		err := suite.DeployAPIGateway(createAPIGatewayOptions1, func(ingressObj *networkingv1.Ingress) {
			// create second api gateway on top of the same function
			createAPIGatewayOptions2 := suite.CompileCreateAPIGatewayOptions(apiGatewayName2, functionName)
			createAPIGatewayOptions2.APIGatewayConfig.Spec.AuthenticationMode = ingress.AuthenticationModeNone
			createAPIGatewayOptions2.APIGatewayConfig.Spec.Host = "host2.com"

			err := suite.DeployAPIGateway(createAPIGatewayOptions2, func(ingress *networkingv1.Ingress) {
				// check that both gateways are invokable
				_, err := http.Get(fmt.Sprintf("http://%s", createAPIGatewayOptions1.APIGatewayConfig.Spec.Host))
				suite.Require().NoError(err)

				_, err = http.Get(fmt.Sprintf("http://%s", createAPIGatewayOptions2.APIGatewayConfig.Spec.Host))
				suite.Require().NoError(err)
			})
			suite.Require().NoError(err)
		})
		suite.Require().NoError(err)

		return true
	})
}

func (suite *DeployAPIGatewayTestSuite) TestUpdate() {
	projectName := "some-project-" + xid.New().String()
	functionName := "function-name-" + xid.New().String()
	apiGatewayName := "apigw-name-" + xid.New().String()
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)

	// create project
	err := suite.Platform.CreateProject(suite.Ctx, &platform.CreateProjectOptions{
		ProjectConfig: &platform.ProjectConfig{
			Meta: platform.ProjectMeta{
				Name:      projectName,
				Namespace: suite.Namespace,
			},
		},
	})
	suite.Require().NoError(err, "Failed to create project")
	createFunctionOptions.FunctionConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName] = projectName

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		createAPIGatewayOptions := suite.CompileCreateAPIGatewayOptions(apiGatewayName, functionName)
		beforeUpdateHostValue := "before-update-host.com"
		createAPIGatewayOptions.APIGatewayConfig.Spec.Host = beforeUpdateHostValue
		createAPIGatewayOptions.APIGatewayConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName] = projectName
		createAPIGatewayOptions.APIGatewayConfig.Meta.Annotations = map[string]string{
			"some/annotation": "some-value",
		}

		// create
		err := suite.Platform.CreateAPIGateway(suite.Ctx, createAPIGatewayOptions)
		suite.Require().NoError(err)

		// delete leftovers
		defer suite.Platform.DeleteAPIGateway(suite.Ctx, &platform.DeleteAPIGatewayOptions{ // nolint: errcheck
			Meta: createAPIGatewayOptions.APIGatewayConfig.Meta,
		})

		suite.WaitForAPIGatewayState(&platform.GetAPIGatewaysOptions{
			Name:      createAPIGatewayOptions.APIGatewayConfig.Meta.Name,
			Namespace: createAPIGatewayOptions.APIGatewayConfig.Meta.Namespace,
		}, platform.APIGatewayStateReady, 10*time.Second)

		ingressInstance := suite.GetAPIGatewayIngress(createAPIGatewayOptions.APIGatewayConfig.Meta.Name, false)
		suite.Require().Equal(beforeUpdateHostValue, ingressInstance.Spec.Rules[0].Host)

		// ensure ingress labels were created correctly
		suite.Require().Equal("apigateway", ingressInstance.Labels[common.NuclioLabelKeyClass])
		suite.Require().Equal("ingress-manager", ingressInstance.Labels[common.NuclioLabelKeyApp])
		suite.Require().Equal(apiGatewayName, ingressInstance.Labels[common.NuclioResourceLabelKeyApiGatewayName])
		suite.Require().Equal(projectName, ingressInstance.Labels[common.NuclioResourceLabelKeyProjectName])
		suite.Require().Equal("some-value", ingressInstance.Annotations["some/annotation"])

		// change host, update
		afterUpdateHostValue := "after-update-host.com"
		annotations := map[string]string{
			"annotation-key": "annotation-value",
		}
		createAPIGatewayOptions.APIGatewayConfig.Spec.Host = afterUpdateHostValue
		createAPIGatewayOptions.APIGatewayConfig.Meta.Annotations = annotations
		err = suite.Platform.UpdateAPIGateway(suite.Ctx, &platform.UpdateAPIGatewayOptions{
			APIGatewayConfig: createAPIGatewayOptions.APIGatewayConfig,
		})
		suite.Require().NoError(err)

		getAPIGatewayOptions := &platform.GetAPIGatewaysOptions{
			Name:      createAPIGatewayOptions.APIGatewayConfig.Meta.Name,
			Namespace: createAPIGatewayOptions.APIGatewayConfig.Meta.Namespace,
		}
		suite.WaitForAPIGatewayState(getAPIGatewayOptions, platform.APIGatewayStateReady, 10*time.Second)

		ingressInstance = suite.GetAPIGatewayIngress(createAPIGatewayOptions.APIGatewayConfig.Meta.Name, false)
		suite.Require().Equal(afterUpdateHostValue, ingressInstance.Spec.Rules[0].Host)

		apiGateway := suite.GetAPIGateway(getAPIGatewayOptions)
		suite.Require().Equal(annotations, apiGateway.GetConfig().Meta.Annotations)
		return true
	})
}

func (suite *DeployAPIGatewayTestSuite) TestSetSpecificPort() {
	functionName := "some-function-name"
	apiGatewayName := "api-gateway-1"
	sidecarPort := 8050
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	createFunctionOptions.FunctionConfig.Spec.Sidecars = []*v1.Container{
		{
			Name:    "sidecar",
			Image:   "busybox",
			Command: []string{"sh", "-c", "while true; do echo 'sidecar'; sleep 1; done"},
			Ports: []v1.ContainerPort{
				{
					Name:          "sidecar-port",
					ContainerPort: int32(sidecarPort),
				},
			},
		},
	}

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		// create an api gateway on top of the function with a specific port to the sidecar
		createAPIGatewayOptions1 := suite.CompileCreateAPIGatewayOptions(apiGatewayName, functionName)
		createAPIGatewayOptions1.APIGatewayConfig.Spec.AuthenticationMode = ingress.AuthenticationModeNone
		createAPIGatewayOptions1.APIGatewayConfig.Spec.Host = "host1.com"
		createAPIGatewayOptions1.APIGatewayConfig.Spec.Upstreams[0].Port = sidecarPort

		err := suite.DeployAPIGateway(createAPIGatewayOptions1, func(ingressObj *networkingv1.Ingress) {
			// check that the ingress has the correct port
			suite.Require().Equal(int32(sidecarPort), ingressObj.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number)
		})
		suite.Require().NoError(err)

		return true
	})
}

func TestAPIGatewayTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}
	suite.Run(t, new(DeployAPIGatewayTestSuite))
}
