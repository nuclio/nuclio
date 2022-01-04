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

package worker

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type MockRuntime struct {
	mock.Mock
}

func (mr *MockRuntime) ProcessEvent(event nuclio.Event, functionLogger logger.Logger) (interface{}, error) {
	args := mr.Called(event, functionLogger)
	return args.Get(0), args.Error(1)
}

func (mr *MockRuntime) GetFunctionLogger() logger.Logger {
	return nil
}

func (mr *MockRuntime) GetStatistics() *runtime.Statistics {
	return nil
}

func (mr *MockRuntime) GetConfiguration() *runtime.Configuration {
	return nil
}

func (mr *MockRuntime) SetStatus(newStatus status.Status) {
}

func (mr *MockRuntime) GetStatus() status.Status {
	return status.Ready
}

func (mr *MockRuntime) Start() error {
	return nil
}

func (mr *MockRuntime) Stop() error {
	return nil
}

func (mr *MockRuntime) Restart() error {
	return nil
}

func (mr *MockRuntime) SupportsRestart() bool {
	return true
}

type WorkerTestSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *WorkerTestSuite) SetupSuite() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
}

func (suite *WorkerTestSuite) TestProcessEvent() {
	mockRuntime := MockRuntime{}
	worker, _ := NewWorker(suite.logger, 100, &mockRuntime)
	event := &nuclio.AbstractEvent{}

	// expect the mock process event to be called with the event
	mockRuntime.On("ProcessEvent", event, suite.logger).Return(nil, nil).Once()

	// process the event
	_, err := worker.ProcessEvent(event, suite.logger)
	suite.Require().NoError(err)

	// make sure all expectations are met
	mockRuntime.AssertExpectations(suite.T())

	// make sure id was set
	suite.Require().NotNil(event.GetID())
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestWorkerTestSuite(t *testing.T) {
	suite.Run(t, new(WorkerTestSuite))
}
