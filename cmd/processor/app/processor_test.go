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

package app

import (
	"fmt"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/eventprocessor"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	// load cron trigger for tests purposes
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/cron"
)

type TriggerTestSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *TriggerTestSuite) SetupSuite() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)
}

func (suite *TriggerTestSuite) TestCreateManyTriggersWithSameWorkerAllocatorName() {
	processorInstance := Processor{
		logger:                suite.logger,
		functionLogger:        suite.logger.GetChild("some-function-logger"),
		namedWorkerAllocators: eventprocessor.NewAllocatorSyncMap(),
	}
	totalTriggers := 1000
	triggerSpecs := map[string]functionconfig.Trigger{}
	for i := 0; i < totalTriggers; i++ {
		triggerName := fmt.Sprintf("Cron%d", i)
		triggerSpecs[triggerName] = functionconfig.Trigger{
			Name:                triggerName,
			Kind:                "cron",
			WorkerAllocatorName: "sameAllocator",
			Attributes: map[string]interface{}{
				"interval": "24h",
			},
		}
	}
	triggers, err := processorInstance.createTriggers(&processor.Configuration{
		Config: functionconfig.Config{
			Spec: functionconfig.Spec{
				Runtime:  "golang",
				Handler:  "nuclio:builtin",
				Triggers: triggerSpecs,
			},
		},
		PlatformConfig: &platformconfig.Config{
			Kind: common.LocalPlatformName,
		},
	})

	suite.Require().NoError(err)
	suite.Require().Len(triggers, totalTriggers)
	suite.Require().Len(processorInstance.namedWorkerAllocators.Keys(),
		1,
		"Expected only one named allocator to be created")
}

func (suite *TriggerTestSuite) TestRestartTriggers() {
	restartChannel := make(chan trigger.Trigger, 1)
	stopRestart := make(chan bool, 1)
	processorInstance := Processor{
		logger:                    suite.logger,
		functionLogger:            suite.logger.GetChild("some-function-logger"),
		namedWorkerAllocators:     eventprocessor.NewAllocatorSyncMap(),
		restartTriggerChan:        restartChannel,
		stopRestartTriggerRoutine: stopRestart,
	}
	go processorInstance.listenOnRestartTriggerChannel()

	// stop listening when test is finished
	defer func() {
		stopRestart <- true
	}()

	triggerSpecs := map[string]functionconfig.Trigger{}
	triggerName := "v3iostream-test"
	triggerSpecs[triggerName] = functionconfig.Trigger{
		Name:     triggerName,
		Kind:     "v3ioStream",
		Password: "some-password",
		URL:      "some-path/container@consumer-group",
		Paths: []string{
			"some-path/container@consumer-group",
		},
	}

	triggers, err := processorInstance.createTriggers(&processor.Configuration{
		Config: functionconfig.Config{
			Spec: functionconfig.Spec{
				Runtime:  "golang",
				Handler:  "nuclio:builtin",
				Triggers: triggerSpecs,
			},
		},
		PlatformConfig: &platformconfig.Config{
			Kind: common.LocalPlatformName,
		},
	})
	suite.Require().NoError(err)

	suite.Require().NoError(processorInstance.setWorkersStatus(triggers[0], status.Error))

	for _, workerInstance := range triggers[0].GetWorkers() {
		suite.Require().Equal(workerInstance.GetStatus(), status.Error)
	}

	// mock a trigger
	testTriggerInstance := &testTrigger{}
	testTriggerInstance.On("Stop", mock.Anything).Return(nil)
	testTriggerInstance.On("Start", mock.Anything).Return(nil)
	testTriggerInstance.On("GetKind").Return("testTriggerKind")
	testTriggerInstance.On("GetName").Return("testTriggerName")
	testTriggerInstance.On("GetID").Return("testTriggerID")

	// signal the processor to stop the trigger
	restartChannel <- testTriggerInstance

	time.Sleep(time.Second)

	testTriggerInstance.AssertCalled(suite.T(), "Stop", mock.Anything)
	testTriggerInstance.AssertCalled(suite.T(), "Start", mock.Anything)
}

// mock trigger

type testTrigger struct {
	mock.Mock
}

func (t *testTrigger) Initialize() error {
	t.Called()
	return nil
}

func (t *testTrigger) Start(checkpoint functionconfig.Checkpoint) error {
	t.Called(checkpoint)
	return nil
}

func (t *testTrigger) Stop(force bool) (functionconfig.Checkpoint, error) {
	t.Called(force)
	return nil, nil
}

func (t *testTrigger) GetID() string {
	t.Called()
	return "testTriggerID"
}

func (t *testTrigger) GetClass() string {
	t.Called()
	return ""
}

func (t *testTrigger) GetKind() string {
	t.Called()
	return "testTriggerKind"
}

func (t *testTrigger) GetName() string {
	t.Called()
	return "testTriggerName"
}

func (t *testTrigger) GetConfig() map[string]interface{} {
	t.Called()
	return nil
}

func (t *testTrigger) GetStatistics() *trigger.Statistics {
	t.Called()
	return nil
}

func (t *testTrigger) IsBusy() bool {
	args := t.Called()
	return args.Bool(0)
}

func (t *testTrigger) GetWorkers() []eventprocessor.EventProcessor {
	t.Called()
	return nil
}

func (t *testTrigger) GetNamespace() string {
	t.Called()
	return ""
}

func (t *testTrigger) GetFunctionName() string {
	t.Called()
	return ""
}

func (t *testTrigger) GetProjectName() string {
	t.Called()
	return ""
}

func (t *testTrigger) SignalWorkersToDrain() error {
	t.Called()
	return nil
}

func (t *testTrigger) SignalWorkersToTerminate() error {
	t.Called()
	return nil
}

func (t *testTrigger) SignalWorkersToContinue() error {
	t.Called()
	return nil
}

type ProcessorTestSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *ProcessorTestSuite) SetupSuite() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)
}

func (suite *ProcessorTestSuite) TestResolveWatcherTimeout() {
	tests := []struct {
		name                  string
		streamChunkTimeoutStr string
		eventTimeoutStr       string
		expectedTimeout       time.Duration
		expectErr             bool
	}{
		{
			name:                  "valid stream chunk timeout",
			streamChunkTimeoutStr: "3s",
			eventTimeoutStr:       "5s",
			expectedTimeout:       3 * time.Second,
		},
		{
			name:                  "no stream chunk, valid event timeout",
			streamChunkTimeoutStr: "",
			eventTimeoutStr:       "5s",
			expectedTimeout:       5 * time.Second,
		},
		{
			name:                  "neither timeout set, use default",
			streamChunkTimeoutStr: "",
			eventTimeoutStr:       "",
			expectedTimeout:       defaultWatcherTimeout,
		},
		{
			name:                  "invalid stream chunk timeout",
			streamChunkTimeoutStr: "invalid",
			eventTimeoutStr:       "5s",
			expectErr:             true,
		},
		{
			name:                  "invalid stream chunk timeout",
			streamChunkTimeoutStr: "5s",
			eventTimeoutStr:       "invalid",
			expectErr:             true,
		},
		{
			name:                  "invalid event timeout after empty stream chunk",
			streamChunkTimeoutStr: "",
			eventTimeoutStr:       "notaduration",
			expectErr:             true,
		},
		{
			name:                  "zero timeout for stream chunk",
			streamChunkTimeoutStr: "",
			eventTimeoutStr:       "0",
			expectErr:             true,
		},
		{
			name:                  "zero timeout for stream chunk with empty event timeout",
			streamChunkTimeoutStr: "0",
			eventTimeoutStr:       "",
			expectErr:             true,
		},
	}

	for _, testCase := range tests {
		suite.Run(testCase.name, func() {
			spec := functionconfig.Spec{
				EventTimeout:       testCase.eventTimeoutStr,
				StreamChunkTimeout: testCase.streamChunkTimeoutStr,
			}

			p := &Processor{logger: suite.logger}
			timeout, err := p.resolveWatcherTimeout(spec)

			if testCase.expectErr {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
				suite.Require().Equal(testCase.expectedTimeout, timeout)
			}
		})
	}
}

func TestTriggerTestSuite(t *testing.T) {
	suite.Run(t, new(TriggerTestSuite))
}

func TestProcessorTestSuite(t *testing.T) {
	suite.Run(t, new(ProcessorTestSuite))
}
