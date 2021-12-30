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

package timeout

import (
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/zap"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type mockTestTrigger struct {
	trigger.AbstractTrigger
	workers []*worker.Worker
	mock.Mock
}

func (t *mockTestTrigger) GetWorkers() []*worker.Worker {
	t.Called()
	return t.workers
}

func (t *mockTestTrigger) GetConfig() map[string]interface{} {
	return nil
}

func (t *mockTestTrigger) Start(checkpoint functionconfig.Checkpoint) error {
	return nil
}

func (t *mockTestTrigger) Stop(force bool) (functionconfig.Checkpoint, error) {
	return nil, nil
}

type mockTestProcessor struct {
	triggers []trigger.Trigger
	mock.Mock
}

func (tp *mockTestProcessor) GetTriggers() []trigger.Trigger {
	tp.Called()
	return tp.triggers
}

func (tp *mockTestProcessor) Stop() {
}

type eventTimeoutSuite struct {
	suite.Suite
}

func (suite *eventTimeoutSuite) TestWatcher() {
	logger, err := nucliozap.NewNuclioZapTest("EventTimeout")
	suite.Require().NoError(err, "Can't create logger")

	mockTrigger := &mockTestTrigger{
		workers: []*worker.Worker{{}},
	}

	mockTrigger.On("GetWorkers").Return(mockTrigger.GetWorkers())

	mockProcessor := &mockTestProcessor{
		triggers: []trigger.Trigger{
			mockTrigger,
		},
	}

	mockProcessor.On("GetTriggers").Return(nil)

	timeout := time.Millisecond
	_, err = NewEventTimeoutWatcher(logger, timeout, mockProcessor)
	suite.Require().NoError(err)
	time.Sleep(10 * timeout) // Give watcher time to work

	mockTrigger.AssertExpectations(suite.T())
	mockProcessor.AssertExpectations(suite.T())
}

func TestEventTimeoutWatcher(t *testing.T) {
	suite.Run(t, &eventTimeoutSuite{})
}
