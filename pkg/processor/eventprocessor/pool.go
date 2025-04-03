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

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errgroup"
	"github.com/nuclio/nuclio/pkg/processor/statistics"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type syncPoolAllocator struct {
	logger      logger.Logger
	objectsChan chan EventProcessor
	objects     []EventProcessor
	statistics  safeAllocatorStatistics

	isTerminated atomic.Bool
}

func NewSyncPoolAllocator(parentLogger logger.Logger, objects []EventProcessor) (Allocator, error) {
	newFixedPool := &syncPoolAllocator{
		logger:       parentLogger.GetChild("sync_pool_allocator"),
		statistics:   safeAllocatorStatistics{},
		isTerminated: atomic.Bool{},
	}

	if err := newFixedPool.SetObjects(objects); err != nil {
		return nil, errors.Wrap(err, "Failed to create sync pool allocator")
	}
	return newFixedPool, nil
}

func (sa *syncPoolAllocator) Allocate(timeout time.Duration) (EventProcessor, error) {
	sa.statistics.AllocationCount.Add(1)
	// get total number of objects
	totalNumberObjects := len(sa.objects)
	currentNumberOfAvailableObjects := len(sa.objectsChan)
	percentageOfAvailableObjects := float64(currentNumberOfAvailableObjects*100.0) / float64(totalNumberObjects)

	// measure how many objects are available in the queue while we're allocating
	sa.statistics.AllocationObjectsAvailablePercentage.Add(uint64(percentageOfAvailableObjects))

	// try to allocate a worker and fall back to default immediately if there's none available
	select {
	case objectInstance := <-sa.objectsChan:
		sa.statistics.AllocationSuccessImmediateTotal.Add(1)
		return objectInstance, nil
	default:

		// if there's no timeout, return now
		if timeout == 0 {
			sa.statistics.AllocationSuccessAfterWaitTotal.Add(1)
			return nil, ErrNoAvailableObjects
		}

		waitStartAt := time.Now()

		// if there is a timeout, try to allocate while waiting for the time
		// to pass
		select {
		case workerInstance := <-sa.objectsChan:
			sa.statistics.AllocationSuccessAfterWaitTotal.Add(1)
			sa.statistics.AllocationWaitDurationMilliSecondsSum.Add(uint64(time.Since(waitStartAt).Nanoseconds() / 1e6))
			return workerInstance, nil
		case <-time.After(timeout):
			sa.statistics.AllocationTimeoutTotal.Add(1)
			return nil, ErrNoAvailableObjects
		}
	}
}

func (sa *syncPoolAllocator) Stop() error {
	// Stop the old objects that are being cleaned up
	if err := sa.SignalTermination(); err != nil {
		sa.logger.DebugWith("Failed to stop objects in allocator",
			"error", err.Error())
	}

	// close channel
	if sa.objectsChan != nil {
		close(sa.objectsChan)
	}
	// clean up objects
	clear(sa.objects)
	return nil
}

func (sa *syncPoolAllocator) Release(object EventProcessor) {
	if sa.IsTerminated() {
		sa.logger.DebugWith("Allocator is terminated, not releasing object",
			"object", object.GetIndex())
		return
	}
	defer common.CatchAndLogPanicWithOptions(context.Background(), // nolint: errcheck
		sa.logger,
		"Release object (Allocator restarted ?)",
		&common.CatchAndLogPanicOptions{
			Args:          nil,
			CustomHandler: nil,
		})
	sa.objectsChan <- object
}

func (sa *syncPoolAllocator) GetObjects() []EventProcessor {
	return sa.objects
}

func (sa *syncPoolAllocator) GetNumObjectsAvailable() int {
	return len(sa.objectsChan)
}

func (sa *syncPoolAllocator) SetObjects(objects []EventProcessor) error {
	// Stop() cleans up sa.objects, so if `objects` and `sa.objects` are the same reference,
	// the new objects will be cleaned up as well.
	// To avoid this, we create a copy of the `objects` slice
	objects = append([]EventProcessor(nil), objects...)

	if err := sa.Stop(); err != nil {
		sa.logger.WarnWith("Failed to stop objects in allocator",
			"error", err.Error())
	}

	// Stop() marks the allocator as terminated and closes the channel,
	// so we can safely set new objects
	sa.isTerminated.Store(false)

	// Set new objects and initialize channels with the length of new objects
	sa.objects = objects
	sa.objectsChan = make(chan EventProcessor, len(objects))

	// Populate the objects channel with the new objects
	for _, object := range objects {
		sa.objectsChan <- object
	}

	// Log the update of allocator objects
	sa.logger.DebugWith("Allocator objects updated", "size", len(objects))
	return nil
}

// GetStatistics returns object allocator statistics
// return unsafe copy of the statistics to avoid any unnecessary blocking of the actual statistics object
// used in gatherers which are thread-safe
func (sa *syncPoolAllocator) GetStatistics() *statistics.AllocatorStatistics {
	allocatorStatistics := &statistics.AllocatorStatistics{
		AllocationCount:                       sa.statistics.AllocationCount.Load(),
		AllocationSuccessImmediateTotal:       sa.statistics.AllocationSuccessImmediateTotal.Load(),
		AllocationSuccessAfterWaitTotal:       sa.statistics.AllocationSuccessAfterWaitTotal.Load(),
		AllocationTimeoutTotal:                sa.statistics.AllocationTimeoutTotal.Load(),
		AllocationWaitDurationMilliSecondsSum: sa.statistics.AllocationWaitDurationMilliSecondsSum.Load(),
		AllocationObjectsAvailablePercentage:  sa.statistics.AllocationObjectsAvailablePercentage.Load(),
	}
	return allocatorStatistics
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
	sa.isTerminated.Store(true)
	for _, objectInstance := range sa.GetObjects() {
		objectInstance := objectInstance

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

func (sa *syncPoolAllocator) IsTerminated() bool {
	return sa.isTerminated.Load()
}
