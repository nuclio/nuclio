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
	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/processor/statistics"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

// blockingPoolAllocator is a thread-safe object allocator that uses a channel to manage its objects
// `blocking` means that if an object is allocated, it will be unavailable for allocation until the object is released
type blockingPoolAllocator struct {
	*abstractPoolAllocator
	objectsChan chan EventProcessor
	statistics  safeAllocatorStatistics
}

func NewBlockingPoolAllocator(parentLogger logger.Logger, objects []EventProcessor) (Allocator, error) {
	abstractPoolAllocatorInstance := newAbstractPoolAllocator(parentLogger.GetChild("sync-pool-allocator"))
	newFixedPool := &blockingPoolAllocator{
		abstractPoolAllocator: abstractPoolAllocatorInstance,
		statistics:            safeAllocatorStatistics{},
	}

	if err := newFixedPool.SetObjects(objects); err != nil {
		return nil, errors.Wrap(err, "Failed to create sync pool allocator")
	}
	return newFixedPool, nil
}

func (a *blockingPoolAllocator) Allocate(timeout time.Duration) (EventProcessor, error) {
	a.statistics.AllocationCount.Add(1)
	// get total number of objects
	totalNumberObjects := len(a.objects)
	currentNumberOfAvailableObjects := len(a.objectsChan)
	percentageOfAvailableObjects := float64(currentNumberOfAvailableObjects*100.0) / float64(totalNumberObjects)

	// measure how many objects are available in the queue while we're allocating
	a.statistics.AllocationObjectsAvailablePercentage.Add(uint64(percentageOfAvailableObjects))

	// try to allocate a worker and fall back to default immediately if there's none available
	select {
	case objectInstance := <-a.objectsChan:
		a.statistics.AllocationSuccessImmediateTotal.Add(1)
		return objectInstance, nil
	default:

		// if there's no timeout, return now
		if timeout == 0 {
			a.statistics.AllocationSuccessAfterWaitTotal.Add(1)
			return nil, ErrNoAvailableObjects
		}

		waitStartAt := time.Now()

		// if there is a timeout, try to allocate while waiting for the time
		// to pass
		select {
		case workerInstance := <-a.objectsChan:
			a.statistics.AllocationSuccessAfterWaitTotal.Add(1)
			a.statistics.AllocationWaitDurationMilliSecondsSum.Add(uint64(time.Since(waitStartAt).Nanoseconds() / 1e6))
			return workerInstance, nil
		case <-time.After(timeout):
			a.statistics.AllocationTimeoutTotal.Add(1)
			return nil, ErrNoAvailableObjects
		}
	}
}

func (a *blockingPoolAllocator) Stop() error {
	// Stop the old objects that are being cleaned up
	if err := a.SignalTermination(); err != nil {
		a.logger.DebugWith("Failed to stop objects in allocator",
			"error", err.Error())
	}

	// close channel
	if a.objectsChan != nil {
		close(a.objectsChan)
	}
	// clean up objects
	clear(a.objects)
	return nil
}

func (a *blockingPoolAllocator) Release(object EventProcessor) {
	if a.IsTerminated() {
		a.logger.DebugWith("Allocator is terminated, not releasing object",
			"object", object.GetIndex())
		return
	}
	defer common.CatchAndLogPanicWithOptions(context.Background(), // nolint: errcheck
		a.logger,
		"Release object (Allocator restarted ?)",
		&common.CatchAndLogPanicOptions{
			Args:          nil,
			CustomHandler: nil,
		})
	a.objectsChan <- object
}

func (a *blockingPoolAllocator) SetObjects(objects []EventProcessor) error {
	// Stop() cleans up sa.objects, so if `objects` and `sa.objects` are the same reference,
	// the new objects will be cleaned up as well.
	// To avoid this, we create a copy of the `objects` slice
	objects = append([]EventProcessor(nil), objects...)

	if err := a.Stop(); err != nil {
		a.logger.WarnWith("Failed to stop objects in allocator",
			"error", err.Error())
	}

	// Stop() marks the allocator as terminated and closes the channel,
	// so we can safely set new objects
	a.isTerminated.Store(false)

	// Set new objects and initialize channels with the length of new objects
	a.objects = objects
	a.objectsChan = make(chan EventProcessor, len(objects))

	// Populate the objects channel with the new objects
	for _, object := range objects {
		a.objectsChan <- object
	}

	// Log the update of allocator objects
	a.logger.DebugWith("Allocator objects updated", "size", len(objects))
	return nil
}

// GetStatistics returns object allocator statistics
// return unsafe copy of the statistics to avoid any unnecessary blocking of the actual statistics object
// used in gatherers which are thread-safe
func (a *blockingPoolAllocator) GetStatistics() *statistics.AllocatorStatistics {
	allocatorStatistics := &statistics.AllocatorStatistics{
		AllocationCount:                       a.statistics.AllocationCount.Load(),
		AllocationSuccessImmediateTotal:       a.statistics.AllocationSuccessImmediateTotal.Load(),
		AllocationSuccessAfterWaitTotal:       a.statistics.AllocationSuccessAfterWaitTotal.Load(),
		AllocationTimeoutTotal:                a.statistics.AllocationTimeoutTotal.Load(),
		AllocationWaitDurationMilliSecondsSum: a.statistics.AllocationWaitDurationMilliSecondsSum.Load(),
		AllocationObjectsAvailablePercentage:  a.statistics.AllocationObjectsAvailablePercentage.Load(),
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
		abstractPoolAllocator: newAbstractPoolAllocator(parentLogger.GetChild("nonblock-pool-allocator")),
		index:                 atomic.Uint64{},
	}
	if err := nonBlockingPoolAllocatorInstance.SetObjects(processors); err != nil {
		return nil, errors.Wrap(err, "Failed to set non blocking pool allocator")
	}

	return nonBlockingPoolAllocatorInstance, nil
}

// Allocate attempts to retrieve a ready EventProcessor instance from the pool in a non-blocking manner.
// If a ready instance is found, it is immediately returned.
// Otherwise, it retries allocation across available instances up to len(objects) times, respecting an optional timeout constraint.
func (nba *nonBlockingPoolAllocator) Allocate(timeout time.Duration) (EventProcessor, error) {
	var startTime time.Time
	// If a timeout is specified, record the start time
	if timeout > 0 {
		startTime = time.Now()
	}

	// Perform up to len(objects) allocation attempts
	for attempt := 0; attempt < len(nba.objects); attempt++ {
		eventProcessor := nba.allocate()
		currentStatus := eventProcessor.GetStatus()

		// If the allocated object is ready, return it immediately
		if currentStatus == status.Ready {
			nba.logger.DebugWith("Object is ready, allocated",
				"id", eventProcessor.GetIndex())

			return eventProcessor, nil
		}

		nba.logger.DebugWith("Object is not ready, cannot allocate",
			"status", currentStatus,
			"objectIndex", eventProcessor.GetIndex(),
			"attempt", attempt+1)

		// If timeout is set and exceeded, stop trying further
		if timeout > 0 && time.Since(startTime) >= timeout {
			break
		}
	}

	return nil, ErrNoAvailableObjects
}

func (nba *nonBlockingPoolAllocator) allocate() EventProcessor {
	// Atomically increment and get the index
	// If idx exceeds math.MaxUint64, it will wrap back to 0, and the subsequent modulo will still yield nba valid slot
	// For optimal performance, this uses a combined atomic add-and-load operation.
	// As a result, the first allocated object will have index 1 instead of 0, which is only the case if len(object) >=2
	// If len(object) == 1, the first object will be allocated with index 0
	idx := nba.index.Add(1)

	// Select the next EventProcessor in nba round-robin manner, wrapping around if needed.
	// This ensures even distribution of allocations across all processors.
	return nba.objects[idx%uint64(len(nba.objects))]
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
	// Stop() cleans up nba.objects, so if `objects` and `nba.objects` are the same reference,
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

// GetStatistics returns aggregated allocation statistics from all objects in the pool.
// The non-blocking allocator itself does not maintain internal statistics,
// as it performs allocations in a non-blocking manner.
// Therefore, it delegates statistics collection to the individual objects it manages.
func (nba *nonBlockingPoolAllocator) GetStatistics() *statistics.AllocatorStatistics {
	// Initialize a struct to accumulate total statistics
	totalStats := &statistics.AllocatorStatistics{}
	numProcessors := len(nba.objects)
	var percentageSum uint64

	for _, object := range nba.objects {
		stats := object.GetAllocationStatistics()

		// Sum counters that are additive by nature
		totalStats.AllocationCount += stats.AllocationCount
		totalStats.AllocationSuccessImmediateTotal += stats.AllocationSuccessImmediateTotal
		totalStats.AllocationSuccessAfterWaitTotal += stats.AllocationSuccessAfterWaitTotal
		totalStats.AllocationTimeoutTotal += stats.AllocationTimeoutTotal
		totalStats.AllocationWaitDurationMilliSecondsSum += stats.AllocationWaitDurationMilliSecondsSum

		// For percentage-based metrics, summing doesn't make sense
		// Instead, accumulate the values and calculate the average later
		// This avoids misrepresenting the overall availability percentage
		percentageSum += stats.AllocationObjectsAvailablePercentage
	}

	// Average percentage metrics to reflect a realistic combined view
	if numProcessors > 0 {
		totalStats.AllocationObjectsAvailablePercentage = percentageSum / uint64(numProcessors)
	}

	return totalStats
}
