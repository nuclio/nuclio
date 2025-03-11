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
	"time"

	"github.com/nuclio/nuclio/pkg/errgroup"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type syncPoolAllocator struct {
	logger      logger.Logger
	objectsChan chan EventProcessor
	objects     []EventProcessor
	statistics  AllocatorStatistics

	isTerminated bool
}

func NewSyncPoolAllocator(parentLogger logger.Logger, objects []EventProcessor) Allocator {
	newFixedPool := &syncPoolAllocator{
		logger:      parentLogger.GetChild("sync_pool_allocator"),
		objectsChan: make(chan EventProcessor, len(objects)),
		objects:     objects,
		statistics:  AllocatorStatistics{},
	}

	// iterate over objects, shove to pool
	for _, object := range objects {
		newFixedPool.objectsChan <- object
	}

	return newFixedPool
}

func (sa *syncPoolAllocator) Allocate(timeout time.Duration) (EventProcessor, error) {
	// TODO: think about reworking of atomic operations logic as it might affect performance
	// we don't want to completely lock here, but we'll use atomic to inc counters where possible
	atomic.AddUint64(&sa.statistics.AllocationCount, 1)

	// get total number of objects
	totalNumberObjects := len(sa.objects)
	currentNumberOfAvailableObjects := len(sa.objectsChan)
	percentageOfAvailableObjects := float64(currentNumberOfAvailableObjects*100.0) / float64(totalNumberObjects)

	// measure how many objects are available in the queue while we're allocating
	atomic.AddUint64(&sa.statistics.AllocationObjectsAvailablePercentage, uint64(percentageOfAvailableObjects))

	// try to allocate a worker and fall back to default immediately if there's none available
	select {
	case objectInstance := <-sa.objectsChan:
		atomic.AddUint64(&sa.statistics.AllocationSuccessImmediateTotal, 1)

		return objectInstance, nil
	default:

		// if there's no timeout, return now
		if timeout == 0 {
			atomic.AddUint64(&sa.statistics.AllocationTimeoutTotal, 1)
			return nil, ErrNoAvailableObjects
		}

		waitStartAt := time.Now()

		// if there is a timeout, try to allocate while waiting for the time
		// to pass
		select {
		case workerInstance := <-sa.objectsChan:
			atomic.AddUint64(&sa.statistics.AllocationSuccessAfterWaitTotal, 1)
			atomic.AddUint64(&sa.statistics.AllocationWaitDurationMilliSecondsSum,
				uint64(time.Since(waitStartAt).Nanoseconds()/1e6))
			return workerInstance, nil
		case <-time.After(timeout):
			atomic.AddUint64(&sa.statistics.AllocationTimeoutTotal, 1)
			return nil, ErrNoAvailableObjects
		}
	}
}

func (sa *syncPoolAllocator) Release(object EventProcessor) {
	sa.objectsChan <- object
}

func (sa *syncPoolAllocator) Shareable() bool {
	return true
}

func (sa *syncPoolAllocator) GetObjects() []EventProcessor {
	return sa.objects
}

func (sa *syncPoolAllocator) GetNumObjectsAvailable() int {
	return len(sa.objectsChan)
}

func (sa *syncPoolAllocator) SetObjects(objects []EventProcessor) error {
	sa.objects = objects
	sa.objectsChan = make(chan EventProcessor, len(objects))
	for _, object := range objects {
		sa.objectsChan <- object
	}
	sa.logger.DebugWith("Allocator objects updated", "size", len(objects))
	return nil
}

// GetStatistics returns object allocator statistics
func (sa *syncPoolAllocator) GetStatistics() *AllocatorStatistics {
	return &sa.statistics
}

func (sa *syncPoolAllocator) SignalDraining() error {
	errGroup, _ := errgroup.WithContext(context.Background(), sa.logger)

	for _, objectInstance := range sa.GetObjects() {
		objectInstance := objectInstance

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

func (sa *syncPoolAllocator) SignalContinue() error {
	errGroup, _ := errgroup.WithContext(context.Background(), sa.logger)

	for _, objectInstance := range sa.GetObjects() {
		objectInstance := objectInstance

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

func (sa *syncPoolAllocator) SignalTermination() error {
	errGroup, _ := errgroup.WithContext(context.Background(), sa.logger)
	sa.isTerminated = true
	for _, objectInstance := range sa.GetObjects() {
		objectInstance := objectInstance

		errGroup.Go(fmt.Sprintf("Terminate worker %d", objectInstance.GetIndex()), func() error {

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

func (sa *syncPoolAllocator) IsTerminated() bool {
	return sa.isTerminated
}
