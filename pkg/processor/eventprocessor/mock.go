/*
Copyright 2025 The Nuclio Authors.

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

package eventprocessor

import (
	"time"

	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/processor/cloudevent"
	"github.com/nuclio/nuclio/pkg/processor/controlcommunication"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc/result"
	"github.com/nuclio/nuclio/pkg/processor/statistics"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/stretchr/testify/mock"
)

type MockEventProcessor struct {
	mock.Mock
}

func (m *MockEventProcessor) ProcessEvent(event nuclio.Event, functionLogger logger.Logger) (result.ResultWithProcessingResult, error) {
	args := m.Called(event, functionLogger)
	return args.Get(0).(result.ResultWithProcessingResult), args.Error(1)
}

func (m *MockEventProcessor) ProcessEventBatch(batch []nuclio.Event, functionLogger logger.Logger) (*result.BatchedResults, error) {
	args := m.Called(batch, functionLogger)
	return args.Get(0).(*result.BatchedResults), args.Error(1)
}
func (m *MockEventProcessor) ProcessStream(stream *result.StreamStart) error {
	args := m.Called(stream)
	return args.Error(0)
}

func (m *MockEventProcessor) Terminate() error {
	return nil
}

func (m *MockEventProcessor) Drain() error {
	return m.Called().Error(0)
}

func (m *MockEventProcessor) Continue() error {
	return m.Called().Error(0)
}

func (m *MockEventProcessor) GetIndex() int {
	return 0
}

func (m *MockEventProcessor) GetRuntime() runtime.Runtime {
	return m.Called().Get(0).(runtime.Runtime)
}

func (m *MockEventProcessor) GetStatus() status.Status {
	return m.Called().Get(0).(status.Status)
}

func (m *MockEventProcessor) SetStatus(s status.Status) {
	m.Called(s)
}

func (m *MockEventProcessor) Stop() error {
	return m.Called().Error(0)
}

func (m *MockEventProcessor) GetStatistics() *statistics.EventProcessingStatistics {
	return m.Called().Get(0).(*statistics.EventProcessingStatistics)
}

func (m *MockEventProcessor) GetAllocationStatistics() *statistics.AllocatorStatistics {
	return m.Called().Get(0).(*statistics.AllocatorStatistics)
}

func (m *MockEventProcessor) GetStructuredCloudEvent() *cloudevent.Structured {
	return m.Called().Get(0).(*cloudevent.Structured)
}

func (m *MockEventProcessor) GetBinaryCloudEvent() *cloudevent.Binary {
	return m.Called().Get(0).(*cloudevent.Binary)
}

func (m *MockEventProcessor) RestartRequired() bool {
	return m.Called().Bool(0)
}

func (m *MockEventProcessor) Restart() error {
	return m.Called().Error(0)
}

func (m *MockEventProcessor) SupportsRestart() bool {
	return m.Called().Bool(0)
}

func (m *MockEventProcessor) Subscribe(kind controlcommunication.ControlMessageKind, channel chan *controlcommunication.ControlMessage) error {
	return m.Called(kind, channel).Error(0)
}

func (m *MockEventProcessor) Unsubscribe(kind controlcommunication.ControlMessageKind, channel chan *controlcommunication.ControlMessage) error {
	return m.Called(kind, channel).Error(0)
}

func (m *MockEventProcessor) WaitForStart(timeout time.Duration) error {
	return m.Called(timeout).Error(0)
}

func (m *MockEventProcessor) RunHandler() {
	m.Called()
}

func (m *MockEventProcessor) IsAsync() bool {
	return m.Called().Bool(0)
}

func (m *MockEventProcessor) IsBusy() bool {
	return m.Called().Bool(0)
}
