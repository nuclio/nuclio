//go:build test_unit

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

package gatherer

import (
	"context"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/eventprocessor"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
)

type mockTrigger struct {
	trigger.AbstractTrigger
	statistics *trigger.UnsafeStatistics
}

func (mt *mockTrigger) GetStatistics() *trigger.UnsafeStatistics {
	return mt.statistics
}

func (mt *mockTrigger) GetClass() string {
	return "test-class"
}

func (mt *mockTrigger) GetKind() string {
	return "test-kind"
}

func (mt *mockTrigger) GetID() string {
	return "test-id"
}

func (mt *mockTrigger) GetName() string {
	return "test-name"
}

func (mt *mockTrigger) GetConfig() map[string]interface{} {
	return map[string]interface{}{}
}

func (mt *mockTrigger) GetFunctionName() string {
	return "test-function"
}

func (mt *mockTrigger) GetProjectName() string {
	return "test-project"
}

func (mt *mockTrigger) GetNamespace() string {
	return "test-namespace"
}

func (mt *mockTrigger) GetWorkers() []eventprocessor.EventProcessor {
	return []eventprocessor.EventProcessor{}
}

func (mt *mockTrigger) Initialize() error {
	return nil
}

func (mt *mockTrigger) Start(checkpoint functionconfig.Checkpoint) error {
	return nil
}

func (mt *mockTrigger) Stop(force bool) (functionconfig.Checkpoint, error) {
	return nil, nil
}

func (mt *mockTrigger) Drain(ctx context.Context) error {
	return nil
}

func (mt *mockTrigger) SignalWorkersToContinue() error {
	return nil
}
