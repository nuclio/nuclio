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
	"sync/atomic"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/processor/statistics"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type syncPoolAllocator struct {
	*abstractPoolAllocator
	objectsChan chan EventProcessor
	statistics  safeAllocatorStatistics
}

func NewSyncPoolAllocator(parentLogger logger.Logger, objects []EventProcessor) (Allocator, error) {
	abstractPoolAllocatorInstance := newAbstractPoolAllocator(parentLogger.GetChild("sync_pool_allocator"))
	newFixedPool := &syncPoolAllocator{
		abstractPoolAllocator: abstractPoolAllocatorInstance,
		statistics:            safeAllocatorStatistics{},
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

type nonBlockingPoolAllocator struct {
	*abstractPoolAllocator

	// index used for round-robin allocation
	index atomic.Uint64
}

func NewNonBlockingPoolAllocator(parentLogger logger.Logger, processors []EventProcessor) (Allocator, error) {
	nonBlockingPoolAllocatorInstance := &nonBlockingPoolAllocator{
		abstractPoolAllocator: newAbstractPoolAllocator(parentLogger.GetChild("nonblock_pool_allocator")),
		index:                 atomic.Uint64{},
	}
	if err := nonBlockingPoolAllocatorInstance.SetObjects(processors); err != nil {
		return nil, errors.Wrap(err, "Failed to set non blocking pool allocator")
	}

	return nonBlockingPoolAllocatorInstance, nil
}

// Allocate allocates an EventProcessor in a non-blocking manner
func (nba *nonBlockingPoolAllocator) Allocate(timeout time.Duration) (EventProcessor, error) {
	// Atomically increment and get the index
	// If idx exceeds math.MaxUint64, it will wrap back to 0, and the subsequent modulo will still yield nba valid slot
	// For optimal performance, this uses a combined atomic add-and-load operation.
	// As a result, the first allocated object will have index 1 instead of 0, which is only
	idx := nba.index.Add(1)

	// Select the next EventProcessor in nba round-robin manner, wrapping around if needed.
	// This ensures even distribution of allocations across all processors.
	selected := nba.objects[idx%uint64(len(nba.objects))]

	return selected, nil
}

// Release is a no-op for non-blocking allocators
func (nba *nonBlockingPoolAllocator) Release(object EventProcessor) {
}

func (nba *nonBlockingPoolAllocator) Stop() error {
	// Stop the old objects that are being cleaned up
	if err := nba.SignalTermination(); err != nil {
		nba.logger.DebugWith("Failed to stop objects in allocator",
			"error", err.Error())
	}

	// clean up objects
	clear(nba.objects)
	return nil
}

func (nba *nonBlockingPoolAllocator) SetObjects(objects []EventProcessor) error {
	// Stop() cleans up sa.objects, so if `objects` and `sa.objects` are the same reference,
	// the new objects will be cleaned up as well.
	// To avoid this, we create nba copy of the `objects` slice
	objects = append([]EventProcessor(nil), objects...)

	if err := nba.Stop(); err != nil {
		nba.logger.WarnWith("Failed to stop objects in allocator",
			"error", err.Error())
	}

	// Stop() marks the allocator as terminated and closes the channel,
	// so we can safely set new objects
	nba.isTerminated.Store(false)

	// Set new objects
	nba.objects = objects

	// Log the update of allocator objects
	nba.logger.DebugWith("Allocator objects updated", "size", len(objects))
	return nil
}
