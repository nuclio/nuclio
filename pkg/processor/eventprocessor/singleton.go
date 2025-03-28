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

	"github.com/nuclio/logger"
)

type asyncSingletonAllocator struct {
	logger       logger.Logger
	object       EventProcessor
	isTerminated bool
}

func NewAsyncSingletonAllocator(parentLogger logger.Logger, eventProcessor EventProcessor) Allocator {

	return &asyncSingletonAllocator{
		logger: parentLogger.GetChild("singelton_allocator"),
		object: eventProcessor,
	}
}

func (s *asyncSingletonAllocator) Allocate(time.Duration) (EventProcessor, error) {
	if s.isTerminated {
		return nil, ErrAllObjectsAreTerminated
	}
	return s.object, nil
}

func (s *asyncSingletonAllocator) SetObjects(objects []EventProcessor) error {
	if len(objects) == 0 {
		return errors.New("Length of setting objects is zero")
	}
	s.object = objects[0]
	return nil
}

func (s *asyncSingletonAllocator) Release(processor EventProcessor) {
}

func (s *asyncSingletonAllocator) GetObjects() []EventProcessor {
	return []EventProcessor{s.object}
}

func (s *asyncSingletonAllocator) GetNumObjectsAvailable() int {
	return 1
}

// GetStatistics returns allocator statistics
func (s *asyncSingletonAllocator) GetStatistics() *AllocatorStatistics {
	return nil
}

func (s *asyncSingletonAllocator) SignalDraining() error {
	return s.object.Drain()
}

func (s *asyncSingletonAllocator) SignalContinue() error {
	return s.object.Continue()
}

func (s *asyncSingletonAllocator) SignalTermination() error {
	s.isTerminated = true
	return s.object.Terminate()
}

func (s *asyncSingletonAllocator) IsTerminated() bool {
	return s.isTerminated
}

func (s *asyncSingletonAllocator) Stop() error {
	return s.SignalTermination()
}
