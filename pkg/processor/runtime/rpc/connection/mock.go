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

package connection

import (
	"time"

	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/processor/eventprocessor"
	"github.com/nuclio/nuclio/pkg/processor/statistics"

	"github.com/stretchr/testify/mock"
)

type MockConnectionManager struct {
	mock.Mock
}

func (m *MockConnectionManager) Prepare() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockConnectionManager) Start(pid int) error {
	args := m.Called(pid)
	return args.Error(0)
}

func (m *MockConnectionManager) Stop() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockConnectionManager) Allocate(duration time.Duration) (eventprocessor.EventProcessor, error) {
	args := m.Called(duration)
	return args.Get(0).(eventprocessor.EventProcessor), args.Error(1)
}

func (m *MockConnectionManager) Release(ep eventprocessor.EventProcessor) {
	m.Called(ep)
}

func (m *MockConnectionManager) GetAddressesForWrapperStart() ([]string, string) {
	args := m.Called()
	return args.Get(0).([]string), args.String(1)
}

func (m *MockConnectionManager) UpdateStatistics(durationSec float64) {
	m.Called(durationSec)
}

func (m *MockConnectionManager) GetAllocationStatistics() *statistics.AllocatorStatistics {
	args := m.Called()
	return args.Get(0).(*statistics.AllocatorStatistics)
}

func (m *MockConnectionManager) SetStatus(s status.Status) {
	m.Called(s)
}

func (m *MockConnectionManager) GetStatus() status.Status {
	args := m.Called()
	return args.Get(0).(status.Status)
}

func (m *MockConnectionManager) GetConfig() ManagerConfigration {
	args := m.Called()
	return args.Get(0).(ManagerConfigration)
}

func (m *MockConnectionManager) IsAsync() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockConnectionManager) IsBusy() bool {
	args := m.Called()
	return args.Bool(0)
}
