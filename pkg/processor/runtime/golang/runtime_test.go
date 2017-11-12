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

	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/nuclio/nuclio-sdk"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

//
// Mock loader
//

type MockLoader struct {
	mock.Mock
}

func (ml *MockLoader) load(path string, handlerName string) (func(*nuclio.Context, nuclio.Event) (interface{}, error), error) {
	ml.Called(path, handlerName)
	return nil, nil
}

//
// Suite
//

type GolangTestSuite struct {
	suite.Suite
	logger nuclio.Logger
}

func (suite *GolangTestSuite) SetupSuite() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
}

func (suite *GolangTestSuite) TestHandlerNoModule() {
	mockLoader := MockLoader{}

	// expect to load some path and handler
	mockLoader.On("load", "somepath", "handler")

	runtime, err := NewRuntime(suite.logger, suite.createConfiguration("somepath", "main:handler"), &mockLoader)
	suite.Require().NoError(err)
	suite.Require().NotNil(runtime)

	mockLoader.AssertExpectations(suite.T())
}

func (suite *GolangTestSuite) TestHandlerAndModule() {
	mockLoader := MockLoader{}

	// expect to load some path and handler
	mockLoader.On("load", "somepath", "anotherHandler")

	runtime, err := NewRuntime(suite.logger,
		suite.createConfiguration("somepath", "module:anotherHandler"),
		&mockLoader)

	suite.Require().NoError(err)
	suite.Require().NotNil(runtime)

	mockLoader.AssertExpectations(suite.T())
}

func (suite *GolangTestSuite) TestInvalidHandler() {
	mockLoader := MockLoader{}

	runtime, err := NewRuntime(suite.logger,
		suite.createConfiguration("somepath", "module:wat:handler"),
		&mockLoader)

	suite.Require().Error(err)
	suite.Require().Nil(runtime)

	mockLoader.AssertExpectations(suite.T())
}

func (suite *GolangTestSuite) TestBuiltinHandler() {
	mockLoader := MockLoader{}

	runtime, err := NewRuntime(suite.logger,
		suite.createConfiguration("somepath", "nuclio:builtin"),
		&mockLoader)

	suite.Require().NoError(err)
	suite.Require().NotNil(runtime)

	// since function equality is disallowed, call the handler and expect the output
	response, err := runtime.(*golang).eventHandler(nil, nil)
	suite.Require().Equal("Built in handler called", response.(string))

	mockLoader.AssertExpectations(suite.T())
}

func (suite *GolangTestSuite) TestEmptyHandler() {
	mockLoader := MockLoader{}

	// expect to load some path and handler
	mockLoader.On("load", "somepath", "Handler")

	runtime, err := NewRuntime(suite.logger, suite.createConfiguration("somepath", ""), &mockLoader)
	suite.Require().NoError(err)
	suite.Require().NotNil(runtime)

	mockLoader.AssertExpectations(suite.T())
}

func (suite *GolangTestSuite) createConfiguration(path string, handler string) *Configuration {
	configuration := Configuration{}
	configuration.Handler = handler
	configuration.PluginPath = path

	return &configuration
}

func TestWriterTestSuite(t *testing.T) {
	suite.Run(t, new(GolangTestSuite))
}
