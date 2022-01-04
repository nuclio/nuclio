//go:build test_unit

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

package apigatewayres

import (
	"context"
	"strings"
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned/fake"
	"github.com/nuclio/nuclio/pkg/platform/kube/ingress"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

type lazyTestSuite struct {
	suite.Suite
	logger         logger.Logger
	client         Client
	ingressManager *ingress.Manager
}

func (suite *lazyTestSuite) SetupTest() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")

	platformConfig, err := platformconfig.NewPlatformConfig("")
	suite.Require().NoError(err)

	kubeClientset := k8sfake.NewSimpleClientset()
	suite.ingressManager, err = ingress.NewManager(suite.logger, kubeClientset, platformConfig)
	suite.Require().NoError(err)

	suite.client, err = NewLazyClient(suite.logger,
		kubeClientset,
		fake.NewSimpleClientset(),
		suite.ingressManager)
	suite.Require().NoError(err)
}

func (suite *lazyTestSuite) TestEnsurePrimaryIngressHasXNuclioTargetHeader() {
	primaryFunctionConfig := *functionconfig.NewConfig()
	primaryFunctionConfig.Meta.Name = "primary-function-name"
	canaryFunctionConfig := *functionconfig.NewConfig()
	canaryFunctionConfig.Meta.Name = "canary-function-name"
	resources, err := suite.client.CreateOrUpdate(context.Background(), &nuclioio.NuclioAPIGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "test-namespace",
		},

		Spec: platform.APIGatewaySpec{
			Host:               "some-host.com",
			Name:               "test-name",
			AuthenticationMode: ingress.AuthenticationModeBasicAuth,
			Authentication: &platform.APIGatewayAuthenticationSpec{
				BasicAuth: &platform.BasicAuth{
					Username: "moshe",
					Password: "ehsom",
				},
			},
			Upstreams: []platform.APIGatewayUpstreamSpec{
				{
					Kind: platform.APIGatewayUpstreamKindNuclioFunction,
					NuclioFunction: &platform.NuclioFunctionAPIGatewaySpec{
						Name: primaryFunctionConfig.Meta.Name,
					},
				},
				{
					Kind: platform.APIGatewayUpstreamKindNuclioFunction,
					NuclioFunction: &platform.NuclioFunctionAPIGatewaySpec{
						Name: canaryFunctionConfig.Meta.Name,
					},
					Percentage: 20,
				},
			},
		},
	})
	suite.Require().NoError(err)
	suite.Require().NotNil(resources.IngressResourcesMap())

	var primaryIngressResources ingress.Resources

	for resourceName, resources := range resources.IngressResourcesMap() {
		if !strings.HasSuffix(resourceName, "-canary") {
			primaryIngressResources = *resources
		}
	}

	// expect primary function ingress to have `X-Nuclio-Target`
	// so that if has STZ option, it would wake up upon a request
	suite.Require().Equal(`proxy_set_header X-Nuclio-Target "primary-function-name,canary-function-name";`,
		primaryIngressResources.Ingress.Annotations["nginx.ingress.kubernetes.io/configuration-snippet"])
}

func TestLazyTestSuite(t *testing.T) {
	suite.Run(t, new(lazyTestSuite))
}
