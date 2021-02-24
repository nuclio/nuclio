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

package controller

import (
	"context"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/loggerus"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/mocks"
	"github.com/nuclio/nuclio/pkg/platform/kube/functionres"

	"github.com/nuclio/logger"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
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

	suite.logger, err = loggerus.CreateTestLogger("test")
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
