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

package monitoring

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	nuclioiofake "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned/fake"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type FunctionMonitoringTestSuite struct {
	suite.Suite
	nuclioioClientSet *nuclioiofake.Clientset
	kubeClientSet     *fake.Clientset
	Namespace         string
	Logger            logger.Logger
	functionMonitor   *FunctionMonitor
	ctx               context.Context
}

func (suite *FunctionMonitoringTestSuite) SetupSuite() {
	var err error
	suite.Namespace = "default-namespace"
	suite.ctx = context.Background()
	suite.Logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err, "Failed to create logger")
}

func (suite *FunctionMonitoringTestSuite) SetupTest() {
	var err error
	suite.kubeClientSet = fake.NewSimpleClientset()
	suite.nuclioioClientSet = nuclioiofake.NewSimpleClientset()
	suite.functionMonitor, err = NewFunctionMonitor(suite.ctx,
		suite.Logger,
		suite.Namespace,
		suite.kubeClientSet,
		suite.nuclioioClientSet,
		time.Second)
	suite.Require().NoError(err)
}

func (suite *FunctionMonitoringTestSuite) TestBulkCheckFunctionStatuses() {

	// create some dummy functions in provisioning mode
	for i := 0; i < 100; i++ {
		function, err := suite.nuclioioClientSet.NuclioV1beta1().
			NuclioFunctions(suite.Namespace).
			Create(suite.ctx,
				&nuclioio.NuclioFunction{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("func-%d", i),
						Namespace: suite.Namespace,
					},
					Spec: functionconfig.Spec{},
					Status: functionconfig.Status{
						State: functionconfig.FunctionStateWaitingForResourceConfiguration,
					},
				}, metav1.CreateOptions{})
		suite.Require().NoError(err)
		suite.Require().NotNil(function)
	}

	err := suite.functionMonitor.checkFunctionStatuses(suite.ctx)
	suite.Require().NoError(err)
}

func TestFunctionMonitoringTestSuite(t *testing.T) {
	suite.Run(t, new(FunctionMonitoringTestSuite))
}
