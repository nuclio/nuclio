//go:build test_unit

/*
Copyright 2026 The Nuclio Authors.

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


package mocks

import (
	"context"

	"github.com/nuclio/logger"
	"github.com/stretchr/testify/mock"
)

// MockLogger is a mock implementation of the logger.Logger interface for testing purposes.
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Error(format interface{}, vars ...interface{}) {
	m.Called(format, vars)
}

func (m *MockLogger) Warn(format interface{}, vars ...interface{}) {
	m.Called(format, vars)
}

func (m *MockLogger) Info(format interface{}, vars ...interface{}) {
	m.Called(format, vars)
}

func (m *MockLogger) Debug(format interface{}, vars ...interface{}) {
	m.Called(format, vars)
}

func (m *MockLogger) ErrorCtx(ctx context.Context, format interface{}, vars ...interface{}) {
	m.Called(ctx, format, vars)
}

func (m *MockLogger) WarnCtx(ctx context.Context, format interface{}, vars ...interface{}) {
	m.Called(ctx, format, vars)
}

func (m *MockLogger) InfoCtx(ctx context.Context, format interface{}, vars ...interface{}) {
	m.Called(ctx, format, vars)
}

func (m *MockLogger) DebugCtx(ctx context.Context, format interface{}, vars ...interface{}) {
	m.Called(ctx, format, vars)
}

func (m *MockLogger) ErrorWith(format interface{}, vars ...interface{}) {
	m.Called(format, vars)
}

func (m *MockLogger) WarnWith(format interface{}, vars ...interface{}) {
	m.Called(format, vars)
}

func (m *MockLogger) InfoWith(format interface{}, vars ...interface{}) {
	m.Called(format, vars)
}

func (m *MockLogger) DebugWith(format interface{}, vars ...interface{}) {
	m.Called(format, vars)
}

func (m *MockLogger) ErrorWithCtx(ctx context.Context, format interface{}, vars ...interface{}) {
	m.Called(ctx, format, vars)
}

func (m *MockLogger) WarnWithCtx(ctx context.Context, format interface{}, vars ...interface{}) {
	m.Called(ctx, format, vars)
}

func (m *MockLogger) InfoWithCtx(ctx context.Context, format interface{}, vars ...interface{}) {
	m.Called(ctx, format, vars)
}

func (m *MockLogger) DebugWithCtx(ctx context.Context, format interface{}, vars ...interface{}) {
	m.Called(ctx, format, vars)
}

func (m *MockLogger) Flush() {
	m.Called()
}

func (m *MockLogger) GetChild(name string) logger.Logger {
	args := m.Called(name)
	return args.Get(0).(logger.Logger)
}
