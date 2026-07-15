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

// ZeroAllocator is a no-op Allocator that owns zero workers. It is intended
// for use in unit tests where Drain must return immediately: with no workers,
// MergeSubscriptions returns an already-closed channel, so Drain returns as
// soon as the loop reads the first (zero-value) message.
type ZeroAllocator struct{}

func (z *ZeroAllocator) Allocate(_ time.Duration) (EventProcessor, error) { return nil, nil }
func (z *ZeroAllocator) Release(_ EventProcessor)                         {}
func (z *ZeroAllocator) GetObjects() []EventProcessor                     { return nil }
func (z *ZeroAllocator) SetObjects(_ []EventProcessor) error              { return nil }
func (z *ZeroAllocator) GetNumObjectsAvailable() int                      { return 0 }
func (z *ZeroAllocator) GetStatistics() *statistics.AllocatorStatistics   { return nil }
func (z *ZeroAllocator) SignalDraining() error                            { return nil }
func (z *ZeroAllocator) SignalContinue() error                            { return nil }
func (z *ZeroAllocator) SignalTermination() error                         { return nil }
func (z *ZeroAllocator) Stop() error                                      { return nil }
func (z *ZeroAllocator) IsTerminated() bool                               { return false }

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

func (m *MockEventProcessor) StreamProcessedSuccessfully() {
	m.Called()
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

func (m *MockEventProcessor) Subscribe(kind controlcommunication.ControlMessageKind) (controlcommunication.Subscription, error) {
	args := m.Called(kind)
	sub, _ := args.Get(0).(controlcommunication.Subscription)
	return sub, args.Error(1)
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
