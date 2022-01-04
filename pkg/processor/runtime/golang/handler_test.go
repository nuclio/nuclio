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

package golang

import (
	"testing"

	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type handlerTestSuite struct {
	suite.Suite
	handler abstractHandler
}

func (suite *handlerTestSuite) SetupTest() {
	loggerInstance, _ := nucliozap.NewNuclioZapTest("test")

	suite.handler = abstractHandler{
		logger: loggerInstance,
	}
}

func (suite *handlerTestSuite) TestHandlerNoModule() {
	pkg, entrypoint, err := suite.handler.parseName("handler")
	suite.Require().NoError(err)
	suite.Require().Equal("", pkg)
	suite.Require().Equal("handler", entrypoint)
}

func (suite *handlerTestSuite) TestHandlerAndModule() {
	pkg, entrypoint, err := suite.handler.parseName("main:handler")
	suite.Require().NoError(err)
	suite.Require().Equal("main", pkg)
	suite.Require().Equal("handler", entrypoint)
}

func (suite *handlerTestSuite) TestEmptyHandler() {
	pkg, entrypoint, err := suite.handler.parseName("")
	suite.Require().NoError(err)
	suite.Require().Equal("main", pkg)
	suite.Require().Equal("Handler", entrypoint)
}

func (suite *handlerTestSuite) TestInvalidHandler() {
	_, _, err := suite.handler.parseName("module:wat:handler")
	suite.Require().Error(err)
}

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(handlerTestSuite))
}
