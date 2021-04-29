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

package cmdrunner

import (
	"context"
	"io"

	"github.com/stretchr/testify/mock"
)

type MockRunner struct {
	mock.Mock
}

func (m *MockRunner) Run(runOptions *RunOptions, format string, vars ...interface{}) (RunResult, error) {
	args := m.Called(runOptions, format, vars)
	runResults, ok := args.Get(0).(RunResult)
	if ok && runOptions != nil {
		runResults.Stderr = Redact(runOptions.LogRedactions, runResults.Stderr)
		runResults.Output = Redact(runOptions.LogRedactions, runResults.Output)
	}
	return runResults, args.Error(1)
}

func (m *MockRunner) Stream(ctx context.Context,
	runOptions *RunOptions,
	format string,
	vars ...interface{}) (io.ReadCloser, error) {
	args := m.Called(ctx, runOptions, format, vars)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func NewMockRunner() *MockRunner {
	return &MockRunner{}
}
