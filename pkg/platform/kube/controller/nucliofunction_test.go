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

package controller

import (
	"context"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/clients/kube"
	"github.com/nuclio/nuclio/pkg/platform/kube/clients/nuclio/clientset/versioned/fake"
	"github.com/nuclio/nuclio/pkg/platform/kube/functionres"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

type NuclioFunctionTestSuite struct {
	suite.Suite
	logger            logger.Logger
	namespace         string
	functionClientSet *fake.Clientset
	k8sClientSet      *k8sfake.Clientset
	controller        *Controller
	ctx               context.Context
	projectName       string
}

func (suite *NuclioFunctionTestSuite) SetupTest() {
	var err error
	resyncInterval := 0 * time.Second
	functionMonitoringInterval := 10 * time.Second
	scalingGracePeriod := 2 * time.Minute
	evictedPodsCleanupInterval := 30 * time.Minute
	cronJobInterval := 10 * time.Second
	defaultNumWorkers := 1

	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	suite.ctx = context.Background()

	platformConfig, err := platformconfig.NewPlatformConfig("")
	suite.Require().NoError(err)

	suite.k8sClientSet = k8sfake.NewSimpleClientset()
	suite.functionClientSet = fake.NewSimpleClientset()

	functionresClient, err := functionres.NewLazyClient(suite.logger,
		kube.NewClientWithRetryFromClient(suite.k8sClientSet),
		suite.functionClientSet)
	suite.Require().NoError(err)
	suite.projectName = "default"
	project := &nuclioio.NuclioProject{}
	project.Name = suite.projectName
	project.Namespace = suite.namespace

	_, err = suite.functionClientSet.NuclioV1beta1().NuclioProjects(suite.namespace).Create(suite.ctx, project, metav1.CreateOptions{})
	suite.Require().NoError(err)

	suite.controller, err = NewController(suite.logger,
		suite.namespace,
		"",
		kube.NewClientWithRetryFromClient(suite.k8sClientSet),
		suite.functionClientSet,
		functionresClient,
		nil,
		resyncInterval,
		functionMonitoringInterval,
		scalingGracePeriod,
		evictedPodsCleanupInterval,
		cronJobInterval,
		platformConfig,
		"configuration-name",
		defaultNumWorkers,
		defaultNumWorkers,
		defaultNumWorkers,
		defaultNumWorkers,
		10*time.Second)
	suite.Require().NoError(err)
}

func (suite *NuclioFunctionTestSuite) TestPreserveBuildLogs() {
	functionInstance := &nuclioio.NuclioFunction{}
	functionInstance.Name = "func-name"
	functionInstance.Status.State = functionconfig.FunctionStateReady
	functionInstance.Status.Logs = []map[string]interface{}{
		{
			"A": "B",
		},
	}
	functionInstance.Labels = map[string]string{
		common.NuclioResourceLabelKeyProjectName: suite.projectName,
	}

	suite.k8sClientSet.PrependReactor("create",
		"configmaps",
		func(action k8stesting.Action) (bool, runtime.Object, error) {

			// simulating a panic being thrown during function creation
			panic("Oh nooo")
		})

	err := suite.controller.functionOperator.CreateOrUpdate(suite.ctx, functionInstance)
	suite.Require().NoError(err)

	// function state must be change to error after panicking during its create/update
	suite.Assert().Equal("B", functionInstance.Status.Logs[0]["A"])
}

func (suite *NuclioFunctionTestSuite) TestRecoverFromPanic() {
	functionInstance := &nuclioio.NuclioFunction{}
	functionInstance.Name = "func-name"
	functionInstance.Status.State = functionconfig.FunctionStateReady
	functionInstance.Labels = map[string]string{
		common.NuclioResourceLabelKeyProjectName: suite.projectName,
	}

	suite.k8sClientSet.PrependReactor("create",
		"configmaps",
		func(action k8stesting.Action) (bool, runtime.Object, error) {

			// simulating a panic being thrown during function creation
			panic("Oh nooo")
		})

	err := suite.controller.functionOperator.CreateOrUpdate(suite.ctx, functionInstance)
	suite.Require().NoError(err)

	// function state must be change to error after panicking during its create/update
	suite.Assert().Equal(functionconfig.FunctionStateError, functionInstance.Status.State)
}

// TestTransientApiErrorMarksFunctionUnhealthy reproduces NUC-797: a transient
// Kubernetes API server timeout on resource create/update used to send the
// function to terminal FunctionStateError. The controller's statesToRespond
// excludes Error, so the function would never re-reconcile. The fix marks the
// function as FunctionStateUnhealthy instead, which the function_monitor can
// flip back to Ready once the API recovers and the deployment is available.
func (suite *NuclioFunctionTestSuite) TestTransientApiErrorMarksFunctionUnhealthy() {
	functionInstance := &nuclioio.NuclioFunction{}
	functionInstance.Name = "transient-error-func"
	functionInstance.Namespace = suite.namespace
	functionInstance.Status.State = functionconfig.FunctionStateWaitingForResourceConfiguration
	functionInstance.Status.InternalInvocationURLs = []string{"transient-error-func.svc.cluster.local:8080"}
	functionInstance.Status.ExternalInvocationURLs = []string{"transient-error-func.example.com"}
	functionInstance.Labels = map[string]string{
		common.NuclioResourceLabelKeyProjectName: suite.projectName,
	}

	_, err := suite.functionClientSet.NuclioV1beta1().
		NuclioFunctions(suite.namespace).
		Create(suite.ctx, functionInstance, metav1.CreateOptions{})
	suite.Require().NoError(err)

	suite.k8sClientSet.PrependReactor("create",
		"services",
		func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewServerTimeout(schema.GroupResource{Resource: "services"}, "create", 0)
		})

	err = suite.controller.functionOperator.CreateOrUpdate(suite.ctx, functionInstance)
	suite.Require().Error(err, "transient error should surface so the work queue retries")

	suite.Assert().Equal(functionconfig.FunctionStateUnhealthy, functionInstance.Status.State,
		"transient K8s API error must not strand the function in terminal FunctionStateError")
	suite.Assert().Equal([]string{"transient-error-func.svc.cluster.local:8080"}, functionInstance.Status.InternalInvocationURLs,
		"invocation URLs must be preserved on transient-error transition")
	suite.Assert().Equal([]string{"transient-error-func.example.com"}, functionInstance.Status.ExternalInvocationURLs,
		"invocation URLs must be preserved on transient-error transition")
}

// TestNonTransientApiErrorMarksFunctionError guards against regressions where
// the new transient-error path accidentally swallows real failures.
func (suite *NuclioFunctionTestSuite) TestNonTransientApiErrorMarksFunctionError() {
	functionInstance := &nuclioio.NuclioFunction{}
	functionInstance.Name = "real-error-func"
	functionInstance.Namespace = suite.namespace
	functionInstance.Status.State = functionconfig.FunctionStateWaitingForResourceConfiguration
	functionInstance.Labels = map[string]string{
		common.NuclioResourceLabelKeyProjectName: suite.projectName,
	}

	_, err := suite.functionClientSet.NuclioV1beta1().
		NuclioFunctions(suite.namespace).
		Create(suite.ctx, functionInstance, metav1.CreateOptions{})
	suite.Require().NoError(err)

	suite.k8sClientSet.PrependReactor("create",
		"services",
		func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewBadRequest("service spec is invalid")
		})

	err = suite.controller.functionOperator.CreateOrUpdate(suite.ctx, functionInstance)
	suite.Require().Error(err)

	suite.Assert().Equal(functionconfig.FunctionStateError, functionInstance.Status.State,
		"non-transient errors must still go to terminal FunctionStateError")
}

func TestTestSuite(t *testing.T) {
	suite.Run(t, new(NuclioFunctionTestSuite))
}
