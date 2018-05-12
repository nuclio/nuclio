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
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (suite *lazyTestSuite) TestNoTriggers() {
	ingressMeta := meta_v1.ObjectMeta{}
	ingressSpec := ext_v1beta1.IngressSpec{}

	// function instance has no triggers
	functionInstance := nuclioio.Function{}
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
	ingressMeta := meta_v1.ObjectMeta{}
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

func (suite *lazyTestSuite) TestTriggerDefinedMultipleIngresses() {
	ingressMeta := meta_v1.ObjectMeta{}
	ingressSpec := ext_v1beta1.IngressSpec{}

	annotations := map[string]string{
		"a1": "v1",
		"a2": "v2",
	}

	// function instance has no triggers
	functionInstance := nuclioio.Function{}
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

func (suite *lazyTestSuite) getIngressRuleByHost(rules []ext_v1beta1.IngressRule, host string) *ext_v1beta1.IngressRule {
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
