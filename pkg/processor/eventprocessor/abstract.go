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
	"context"
	"fmt"
	"sync/atomic"

	"github.com/nuclio/nuclio/pkg/errgroup"
	"github.com/nuclio/nuclio/pkg/processor/statistics"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type abstractPoolAllocator struct {
	logger       logger.Logger
	objects      []EventProcessor
	isTerminated atomic.Bool
}

func newAbstractPoolAllocator(loggerInstance logger.Logger) *abstractPoolAllocator {
	return &abstractPoolAllocator{
		logger:       loggerInstance,
		isTerminated: atomic.Bool{},
	}
}

func (a *abstractPoolAllocator) SignalDraining() error {
	errGroup, _ := errgroup.WithContext(context.Background(), a.logger)

	for _, objectInstance := range a.GetObjects() {

		errGroup.Go(fmt.Sprintf("Drain object %d", objectInstance.GetIndex()), func() error {
			// if object is not already drained, signal it to drain events
			if err := objectInstance.Drain(); err != nil {
				return errors.Wrapf(err, "Failed to signal object %d to drain events", objectInstance.GetIndex())
			}
			return nil
		})
	}

	if err := errGroup.Wait(); err != nil {
		return errors.Wrap(err, "At least one object failed to drain")
	}

	return nil
}

func (a *abstractPoolAllocator) SignalContinue() error {
	errGroup, _ := errgroup.WithContext(context.Background(), a.logger)

	for _, objectInstance := range a.GetObjects() {

		errGroup.Go(fmt.Sprintf("Send continue signal to object %d", objectInstance.GetIndex()), func() error {
			if err := objectInstance.Continue(); err != nil {
				return errors.Wrapf(err, "Failed to signal object %d to continue event processing", objectInstance.GetIndex())
			}
			return nil
		})
	}

	if err := errGroup.Wait(); err != nil {
		return errors.Wrap(err, "At least one object failed to continue")
	}

	return nil
}

func (a *abstractPoolAllocator) SignalTermination() error {
	errGroup, _ := errgroup.WithContext(context.Background(), a.logger)
	a.isTerminated.Store(true)
	for _, objectInstance := range a.GetObjects() {

		errGroup.Go(fmt.Sprintf("Terminate object %d", objectInstance.GetIndex()), func() error {

			if err := objectInstance.Terminate(); err != nil {
				return errors.Wrapf(err, "Failed to signal object %d to terminate", objectInstance.GetIndex())
			}
			return nil
		})
	}

	if err := errGroup.Wait(); err != nil {
		return errors.Wrap(err, "At least one object failed to terminate")
	}

	return nil
}

func (a *abstractPoolAllocator) IsTerminated() bool {
	return a.isTerminated.Load()
}

func (a *abstractPoolAllocator) GetObjects() []EventProcessor {
	return a.objects
}

func (a *abstractPoolAllocator) GetNumObjectsAvailable() int {
	return len(a.objects)
}

func (a *abstractPoolAllocator) GetStatistics() *statistics.AllocatorStatistics {
	return nil
}
