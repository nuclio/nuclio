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
	"errors"
	"time"

	"github.com/nuclio/nuclio/pkg/processor/statistics"

	"github.com/nuclio/logger"
)

// nonBlockingSingletonAllocator is a singleton object allocator
// `nonBlocking` means that if an object is allocated, it is available for allocation again
type nonBlockingSingletonAllocator struct {
	logger       logger.Logger
	object       EventProcessor
	isTerminated bool
}

func NewNonBlockingSingletonAllocator(parentLogger logger.Logger, eventProcessor EventProcessor) Allocator {

	return &nonBlockingSingletonAllocator{
		logger: parentLogger.GetChild("singelton_allocator"),
		object: eventProcessor,
	}
}

func (a *nonBlockingSingletonAllocator) Allocate(time.Duration) (EventProcessor, error) {
	if a.isTerminated {
		return nil, ErrAllObjectsAreTerminated
	}
	return a.object, nil
}

func (a *nonBlockingSingletonAllocator) SetObjects(objects []EventProcessor) error {
	if len(objects) == 0 {
		return errors.New("Length of setting objects is zero")
	}
	a.object = objects[0]
	return nil
}

func (a *nonBlockingSingletonAllocator) Release(processor EventProcessor) {
}

func (a *nonBlockingSingletonAllocator) GetObjects() []EventProcessor {
	return []EventProcessor{a.object}
}

func (a *nonBlockingSingletonAllocator) GetNumObjectsAvailable() int {
	return 1
}

// GetStatistics returns allocator statistics
func (a *nonBlockingSingletonAllocator) GetStatistics() *statistics.AllocatorStatistics {
	return a.object.GetAllocationStatistics()
}

func (a *nonBlockingSingletonAllocator) SignalDraining() error {
	return a.object.Drain()
}

func (a *nonBlockingSingletonAllocator) SignalContinue() error {
	return a.object.Continue()
}

func (a *nonBlockingSingletonAllocator) SignalTermination() error {
	a.isTerminated = true
	return a.object.Terminate()
}

func (a *nonBlockingSingletonAllocator) IsTerminated() bool {
	return a.isTerminated
}

func (a *nonBlockingSingletonAllocator) Stop() error {
	return a.SignalTermination()
}
