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

package controller

import (
	"context"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned/fake"
	"github.com/nuclio/nuclio/pkg/platform/kube/functionres"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

type ControllerTestSuite struct {
	suite.Suite
	logger            logger.Logger
	controller        *Controller
	namespace         string
	functionClientSet *fake.Clientset
	k8sClientSet      *k8sfake.Clientset
	ctx               context.Context
}

func (suite *ControllerTestSuite) SetupTest() {
	var err error
	resyncInterval := 0 * time.Second
	functionMonitoringInterval := 10 * time.Second
	cronJobInterval := 10 * time.Second
	defaultNumWorkers := 1

	// create logger
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	suite.ctx = context.Background()

	platformConfig, err := platformconfig.NewPlatformConfig("")
	suite.Require().NoError(err)

	suite.k8sClientSet = k8sfake.NewSimpleClientset()
	suite.functionClientSet = fake.NewSimpleClientset()

	functionresClient, err := functionres.NewLazyClient(suite.logger,
		suite.k8sClientSet,
		suite.functionClientSet)
	suite.Require().NoError(err)

	// create controller
	suite.controller, err = NewController(suite.logger,
		suite.namespace,
		"",
		suite.k8sClientSet,
		suite.functionClientSet,
		functionresClient,
		nil,
		resyncInterval,
		functionMonitoringInterval,
		cronJobInterval,
		platformConfig,
		"configuration-name",
		defaultNumWorkers,
		defaultNumWorkers,
		defaultNumWorkers,
		defaultNumWorkers)
	suite.Require().NoError(err)

	suite.logger.Info("Starting test")
}

func (suite *ControllerTestSuite) TestFunctionUpdateFailureInvocationURLs() {

	// mock function
	functionInstance := &nuclioio.NuclioFunction{}
	functionInstance.Name = "func-name"
	functionInstance.Status.State = functionconfig.FunctionStateReady
	functionInstance.Status.InternalInvocationURLs = []string{"internal.url:port1"}
	functionInstance.Status.ExternalInvocationURLs = []string{"external.url:port2"}
	functionInstance.Spec.ReadinessTimeoutSeconds = 1

	// call CreateOrUpdate
	err := suite.controller.functionOperator.CreateOrUpdate(suite.ctx, functionInstance)
	suite.Require().Error(err)

	// make sure there are no invocation URLs
	internalInvocationURLs := functionInstance.Status.InternalInvocationURLs
	externalInvocationURLs := functionInstance.Status.ExternalInvocationURLs
	suite.logger.DebugWith("Function failed to update",
		"internalUrls", internalInvocationURLs,
		"externalUrls", externalInvocationURLs)
	suite.Require().Empty(internalInvocationURLs)
	suite.Require().Empty(externalInvocationURLs)
}

func TestControllerUnitTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}
