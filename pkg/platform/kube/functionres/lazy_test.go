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
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	ext_v1beta1 "k8s.io/api/extensions/v1beta1"
)

type lazyTestSuite struct {
	suite.Suite
	logger logger.Logger
	client lazyClient
}

func (suite *lazyTestSuite) SetupTest() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	suite.client.logger = suite.logger
}

func (suite *lazyTestSuite) TestDefaultIngressPatternNoTriggers() {
	ingressSpec := ext_v1beta1.IngressSpec{}

	// function instance has no triggers
	functionInstance := nuclioio.Function{}
	functionInstance.Name = "func-name"
	functionInstance.Spec.Triggers = map[string]functionconfig.Trigger{}

	// get labels
	labels := map[string]string{
		"version": "latest",
	}

	err := suite.client.populateIngressSpec(labels,
		&functionInstance,
		&ingressSpec)

	suite.Require().NoError(err)

	suite.Require().Equal("", ingressSpec.Rules[0].Host)
	suite.Require().Equal("/func-name/latest", ingressSpec.Rules[0].HTTP.Paths[0].Path)
}

func (suite *lazyTestSuite) TestDefaultIngressPatternNoneSpecified() {
	ingressSpec := ext_v1beta1.IngressSpec{}

	// function instance has no triggers
	functionInstance := nuclioio.Function{}
	functionInstance.Name = "func-name"
	functionInstance.Spec.Triggers = map[string]functionconfig.Trigger{
		"mh": {
			Kind: "http",
		},
	}

	// get labels
	labels := map[string]string{
		"version": "latest",
	}

	err := suite.client.populateIngressSpec(labels,
		&functionInstance,
		&ingressSpec)

	suite.Require().NoError(err)

	suite.Require().Equal("", ingressSpec.Rules[0].Host)
	suite.Require().Equal("/func-name/latest", ingressSpec.Rules[0].HTTP.Paths[0].Path)
}

func (suite *lazyTestSuite) TestDefaultIngressPatternEmptySpecified() {
	ingressSpec := ext_v1beta1.IngressSpec{}

	// function instance has no triggers
	functionInstance := nuclioio.Function{}
	functionInstance.Name = "func-name"
	functionInstance.Spec.Triggers = map[string]functionconfig.Trigger{
		"mh": {
			Kind: "http",
			Attributes: map[string]interface{}{
				"defaultIngressPattern": "",
			},
		},
	}

	// get labels
	labels := map[string]string{
		"version": "latest",
	}

	err := suite.client.populateIngressSpec(labels,
		&functionInstance,
		&ingressSpec)

	suite.Require().NoError(err)

	suite.Require().Len(ingressSpec.Rules, 0)
}

func (suite *lazyTestSuite) TestDefaultIngressPatternSpecified() {
	ingressSpec := ext_v1beta1.IngressSpec{}

	// function instance has no triggers
	functionInstance := nuclioio.Function{}
	functionInstance.Name = "func-name"
	functionInstance.Namespace = "func-namespace"
	functionInstance.Spec.Triggers = map[string]functionconfig.Trigger{
		"mh": {
			Kind: "http",
			Attributes: map[string]interface{}{
				"defaultIngressPattern": "/{{.Namespace}}/{{.Name}}/{{.Version}}/wat",
			},
		},
	}

	// get labels
	labels := map[string]string{
		"version": "latest",
	}

	err := suite.client.populateIngressSpec(labels,
		&functionInstance,
		&ingressSpec)

	suite.Require().NoError(err)

	suite.Require().Equal("/func-namespace/func-name/latest/wat", ingressSpec.Rules[0].HTTP.Paths[0].Path)
}

func TestRegistryTestSuite(t *testing.T) {
	suite.Run(t, new(lazyTestSuite))
}
