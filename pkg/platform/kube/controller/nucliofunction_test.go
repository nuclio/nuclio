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
	"github.com/nuclio/nuclio/pkg/platform/kube"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/mocks"
	"github.com/nuclio/nuclio/pkg/platform/kube/functionres"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type NuclioFunctionTestSuite struct {
	suite.Suite
	logger                       logger.Logger
	namespace                    string
	nuclioioV1beta1InterfaceMock *mocks.NuclioV1beta1Interface
	nuclioFunctionInterfaceMock  *mocks.NuclioFunctionInterface
	nuclioioInterfaceMock        *mocks.Interface
	functionresClientMock        *functionres.MockedFunctionRes
	functionOperatorInstance     *functionOperator
}

func (suite *NuclioFunctionTestSuite) SetupTest() {
	var err error
	resyncInterval := 1 * time.Hour

	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	suite.functionresClientMock = &functionres.MockedFunctionRes{}

	suite.functionOperatorInstance, err = newFunctionOperator(suite.logger,
		&Controller{
			namespace: suite.namespace,
		},
		&resyncInterval,
		"",
		suite.functionresClientMock,
		0)
	suite.Require().NoError(err)

	// mock it all the way down
	suite.nuclioioInterfaceMock = &mocks.Interface{}
	suite.nuclioioV1beta1InterfaceMock = &mocks.NuclioV1beta1Interface{}
	suite.nuclioFunctionInterfaceMock = &mocks.NuclioFunctionInterface{}

	suite.nuclioioInterfaceMock.
		On("NuclioV1beta1").
		Return(suite.nuclioioV1beta1InterfaceMock)

	suite.nuclioioV1beta1InterfaceMock.
		On("NuclioFunctions", suite.namespace).
		Return(suite.nuclioFunctionInterfaceMock)

	suite.functionOperatorInstance.controller.nuclioClientSet = suite.nuclioioInterfaceMock
}

func (suite *NuclioFunctionTestSuite) TestCreateOrUpdateWithScaleToZeroLabel() {
	var err error
	kubeFakeClient := fake.NewSimpleClientset()
	suite.functionOperatorInstance.functionresClient, err = functionres.NewLazyClient(
		suite.logger,
		kubeFakeClient,
		suite.nuclioioInterfaceMock)

	go func() {
		functionDeploymentName := "nuclio-func-name"
		for {
			select {
			case <-time.After(5 * time.Second):
				suite.Require().Fail("Took too much time to mark deployment as ready")
			default:
				suite.logger.DebugWith("Getting function deployment",
					"functionDeploymentName", functionDeploymentName)
				deployment, err := kubeFakeClient.AppsV1().Deployments(suite.namespace).Get(functionDeploymentName,
					metav1.GetOptions{})

				if err != nil || deployment == nil {
					suite.logger.DebugWith("Function deployment does not exists yet",
						"functionDeploymentName", functionDeploymentName,
						"err", err)
					time.Sleep(250 * time.Millisecond)
					continue
				}

				if _, found := deployment.Labels[kube.FunctionScaleToZeroLabelKey]; found {
					suite.Require().FailNow("Key should have been omitted from deployment resource")
				}

				suite.logger.DebugWith("Marking function deployment as available",
					"functionDeploymentName", functionDeploymentName)
				_, err = kubeFakeClient.AppsV1().Deployments(suite.namespace).Update(&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "nuclio-func-name",
						Namespace: suite.namespace,
					},
					Status: appsv1.DeploymentStatus{
						Conditions: []appsv1.DeploymentCondition{
							{
								Type:   appsv1.DeploymentAvailable,
								Status: v1.ConditionTrue,
								Reason: "manually-changed-by-nuclio-test",
							},
						},
					},
				})
				suite.Require().NoError(err)

				// done
				return
			}
		}
	}()

	mockedPlatformConfig := &functionres.MockedPlatformConfigurationProvider{}
	mockedPlatformConfig.
		On("GetPlatformConfiguration").
		Return(&platformconfig.Config{
			FunctionAugmentedConfigs: []platformconfig.LabelSelectorAndConfig{
				{
					LabelSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							kube.FunctionScaleToZeroLabelKey: "true",
						},
					},
					HTTPTrigger: functionconfig.Trigger{
						Annotations: map[string]string{
							"do": "it",
						},
					},
				},
			},
		})
	mockedPlatformConfig.
		On("GetPlatformConfigurationName").
		Return("something")

	suite.functionOperatorInstance.functionresClient.SetPlatformConfigurationProvider(mockedPlatformConfig)
	functionInstance := &nuclioio.NuclioFunction{}
	functionInstance.Name = "func-name"
	functionInstance.Namespace = suite.namespace
	functionInstance.Status.State = functionconfig.FunctionStateReady
	functionInstance.Labels = map[string]string{
		kube.FunctionScaleToZeroLabelKey: "true",
	}
	functionInstance.Spec.Triggers = map[string]functionconfig.Trigger{
		"default-http": functionconfig.GetDefaultHTTPTrigger(),
	}
	ctx := context.TODO()
	err = suite.functionOperatorInstance.CreateOrUpdate(ctx, functionInstance)
	suite.Require().NoError(err)

	// nothing override it accidentally
	suite.Require().Equal("true", functionInstance.Labels[kube.FunctionScaleToZeroLabelKey])
	suite.Require().Equal("it", functionInstance.Spec.Triggers["default-http"].Annotations["do"])
}

func (suite *NuclioFunctionTestSuite) TestRecoverFromPanic() {
	functionInstance := &nuclioio.NuclioFunction{}
	functionInstance.Name = "func-name"
	functionInstance.Status.State = functionconfig.FunctionStateReady

	suite.functionresClientMock.
		On("CreateOrUpdate", mock.Anything, functionInstance, mock.Anything).
		Panic("something bad happened")

	suite.nuclioFunctionInterfaceMock.
		On("Update", functionInstance).
		Return(nil, nil).
		Once()

	err := suite.functionOperatorInstance.CreateOrUpdate(context.TODO(), functionInstance)
	suite.Require().NoError(err)

	// function state must be change to error after panicking during its create/update
	suite.Assert().Equal(functionInstance.Status.State, functionconfig.FunctionStateError)
}

func TestTestSuite(t *testing.T) {
	suite.Run(t, new(NuclioFunctionTestSuite))
}
