//go:build test_unit

/*
Copyright 2023 The Nuclio Authors.

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

package worker

import (
	"fmt"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/processor/eventprocessor"
	"github.com/nuclio/nuclio/pkg/processor/statistics"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type AllocatorTestSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *AllocatorTestSuite) SetupSuite() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
}

func (suite *AllocatorTestSuite) TestSingletonAllocator() {
	eventProcessor1 := &eventprocessor.MockEventProcessor{}

	allocator := eventprocessor.NewNonBlockingSingletonAllocator(suite.logger, eventProcessor1)
	suite.Require().NotNil(allocator)

	// allocate once, time should be ignored
	allocatedEventProcessor1, err := allocator.Allocate(time.Hour)
	suite.Require().NoError(err)
	suite.Require().Same(eventProcessor1, allocatedEventProcessor1)

	// allocate again, release doesn't need to happen
	allocatedEventProcessor1, err = allocator.Allocate(time.Hour)
	suite.Require().NoError(err)
	suite.Require().Same(eventProcessor1, allocatedEventProcessor1)

	// release shouldn't do anything
	suite.Require().NotPanics(func() { allocator.Release(eventProcessor1, false) })
}

func (suite *AllocatorTestSuite) TestNonBlockingPoolAllocator() {
	eventProcessors := createEventProcessors(2)

	eventProcessor1 := eventProcessors[1]
	eventProcessor2 := eventProcessors[0]

	// Cast to mock versions for method mocking
	mockEventProcessor1 := eventProcessors[1].(*eventprocessor.MockEventProcessor)
	mockEventProcessor2 := eventProcessors[0].(*eventprocessor.MockEventProcessor)

	// Mock GetStatus to return Ready for both event processors initially
	mockEventProcessor1.On("GetStatus").Return(status.Ready).Times(4)
	mockEventProcessor2.On("GetStatus").Return(status.Ready).Twice()

	allocator, err := eventprocessor.NewNonBlockingPoolAllocator(suite.logger, eventProcessors)
	suite.Require().NoError(err)
	suite.Require().NotNil(allocator)

	// allocate and not release
	firstAllocatedEventProcessor1, err := allocator.Allocate(time.Second)
	suite.Require().NoError(err)
	suite.Require().Same(eventProcessor1, firstAllocatedEventProcessor1)

	// ensure round robin allocation
	nextAllocatedEventProcessor1, err := allocator.Allocate(time.Second)
	suite.Require().NoError(err)
	suite.Require().Same(eventProcessor2, nextAllocatedEventProcessor1)

	// allocate 1st again (check round robin + allocation of already allocated event processor)
	nextAllocatedEventProcessor1, err = allocator.Allocate(time.Second)
	suite.Require().NoError(err)
	suite.Require().Same(firstAllocatedEventProcessor1, nextAllocatedEventProcessor1)

	// release the first event processor
	allocator.Release(eventProcessor1, false)

	// ensure that allocator allocates the second event processor anyway
	nextAllocatedEventProcessor1, err = allocator.Allocate(time.Second)
	suite.Require().NoError(err)
	suite.Require().Same(eventProcessor2, nextAllocatedEventProcessor1)

	allocator.Release(eventProcessor2, false)

	mockEventProcessor2.On("GetStatus").Return(status.Stopped).Twice()

	// should allocate eventProcessor1 (only one that's ready)
	// it's turn for the 1st one that's ready anyway
	allocatedEventProcessor, err := allocator.Allocate(time.Second)
	suite.Require().NoError(err)
	suite.Require().Same(eventProcessor1, allocatedEventProcessor)

	// it's turn for the 2nd event processor, however it's not ready
	// so it should allocate the 1st one again
	allocatedEventProcessor, err = allocator.Allocate(time.Second)
	suite.Require().NoError(err)
	suite.Require().Same(eventProcessor1, allocatedEventProcessor)

	// simulate eventProcessor1 becoming not ready too
	mockEventProcessor1.On("GetStatus").Return(status.Stopped).Once()

	// both processors are not ready, should fail
	allocatedEventProcessor, err = allocator.Allocate(100 * time.Millisecond)
	suite.Require().ErrorIs(err, eventprocessor.ErrNoAvailableObjects)
	suite.Require().Nil(allocatedEventProcessor)

	allocator.Release(eventProcessor1, false)
	allocator.Release(eventProcessor2, false)

	mockEventProcessor1.AssertExpectations(suite.T())
	mockEventProcessor2.AssertExpectations(suite.T())
}

func (suite *AllocatorTestSuite) TestNonBlockingPoolAllocatorStatistics() {
	eventProcessors := createEventProcessors(2)

	// Create a non-blocking pool allocator using the event processors
	allocator, err := eventprocessor.NewNonBlockingPoolAllocator(suite.logger, eventProcessors)
	suite.Require().NoError(err)

	// Cast the generic event processor interfaces to mock versions for method mocking
	mockEventProcessor1 := eventProcessors[1].(*eventprocessor.MockEventProcessor)
	mockEventProcessor2 := eventProcessors[0].(*eventprocessor.MockEventProcessor)

	// Define the mock allocation statistics to be returned by each processor
	mockStatistics := &statistics.AllocatorStatistics{
		AllocationCount:                       20,
		AllocationSuccessImmediateTotal:       10,
		AllocationSuccessAfterWaitTotal:       5,
		AllocationTimeoutTotal:                5,
		AllocationWaitDurationMilliSecondsSum: 100,
		AllocationObjectsAvailablePercentage:  50,
	}

	// Configure the mock processors to return the mock statistics on GetAllocationStatistics call
	mockEventProcessor1.On("GetAllocationStatistics").Return(mockStatistics)
	mockEventProcessor2.On("GetAllocationStatistics").Return(mockStatistics)

	// Call GetStatistics on the allocator, which aggregates statistics from all its objects
	calculatedStatistics := allocator.GetStatistics()

	// Assert that all counters are summed across processors
	suite.Equal(uint64(40), calculatedStatistics.AllocationCount)
	suite.Equal(uint64(20), calculatedStatistics.AllocationSuccessImmediateTotal)
	suite.Equal(uint64(10), calculatedStatistics.AllocationSuccessAfterWaitTotal)
	suite.Equal(uint64(10), calculatedStatistics.AllocationTimeoutTotal)
	suite.Equal(uint64(200), calculatedStatistics.AllocationWaitDurationMilliSecondsSum)

	// Assert that the percentage is averaged, not summed
	// Each processor returned 50%, so average of two is still 50%
	suite.Equal(uint64(50), calculatedStatistics.AllocationObjectsAvailablePercentage)

	// Ensure that all mocked expectations were met
	mockEventProcessor1.AssertExpectations(suite.T())
	mockEventProcessor2.AssertExpectations(suite.T())
}

func (suite *AllocatorTestSuite) TestFixedBlockingPoolAllocator() {
	eventProcessors := createEventProcessors(2)
	eventProcessor2 := eventProcessors[1]
	eventProcessor1 := eventProcessors[0]

	allocator, err := eventprocessor.NewBlockingPoolAllocator(suite.logger, eventProcessors)
	suite.Require().NoError(err)
	suite.Require().NotNil(allocator)

	// allocate once - should allocate
	firstAllocatedEventProcessor, err := allocator.Allocate(time.Hour)
	suite.Require().NoError(err)
	suite.Require().Same(firstAllocatedEventProcessor, eventProcessor1)
	suite.Require().Contains(eventProcessors, firstAllocatedEventProcessor)

	// allocate again - should allocate other event processor
	secondAllocatedEventProcessor, err := allocator.Allocate(time.Hour)
	suite.Require().NoError(err)
	suite.Require().Contains(eventProcessors, secondAllocatedEventProcessor)
	suite.NotSame(firstAllocatedEventProcessor, secondAllocatedEventProcessor)

	// allocate yet again - should time out
	failedAllocationEventProcessor, err := allocator.Allocate(50 * time.Millisecond)
	suite.Require().Error(err)
	suite.Require().Nil(failedAllocationEventProcessor)

	// release the second event processor
	suite.Require().NotPanics(func() { allocator.Release(eventProcessor2, false) })

	// allocate again - should allocate second event processor
	thirdAllocatedEventProcessor, err := allocator.Allocate(time.Hour)
	suite.Require().NoError(err)
	suite.Require().Same(eventProcessor2, thirdAllocatedEventProcessor)

	err = common.RetryUntilSuccessful(3*time.Second,
		1*time.Second,
		func() bool {
			statistics := allocator.GetStatistics()
			return statistics.AllocationCount == uint64(4) &&
				statistics.AllocationSuccessImmediateTotal == uint64(3) &&
				statistics.AllocationTimeoutTotal == uint64(1)
		})

	suite.Require().NoError(err)

	// reset objects in allocator (both should become available)
	err = allocator.SetObjects(eventProcessors)
	suite.Require().NoError(err)

	// check allocation
	eventProcessor, err := allocator.Allocate(time.Hour)
	suite.Require().NoError(err)
	suite.Require().Contains(eventProcessors, eventProcessor)

	// check that statistics wasn't reset
	err = common.RetryUntilSuccessful(3*time.Second,
		1*time.Second,
		func() bool {
			statistics := allocator.GetStatistics()
			return statistics.AllocationCount == uint64(5) &&
				statistics.AllocationSuccessImmediateTotal == uint64(4) &&
				statistics.AllocationTimeoutTotal == uint64(1)
		})
	suite.Require().NoError(err)
}

func TestAllocatorTestSuite(t *testing.T) {
	suite.Run(t, new(AllocatorTestSuite))
}

func BenchmarkParallelAllocation(b *testing.B) {
	eventProcessorsCount := []int{10, 100, 1000}
	allocatorTypes := []struct {
		name        string
		constructor func(logger logger.Logger, eps []eventprocessor.EventProcessor) (eventprocessor.Allocator, error)
	}{
		{
			name:        "Sync",
			constructor: eventprocessor.NewBlockingPoolAllocator,
		},
		{
			name:        "NonBlocking",
			constructor: eventprocessor.NewNonBlockingPoolAllocator,
		},
	}

	for _, count := range eventProcessorsCount {
		for _, allocator := range allocatorTypes {
			b.Run(fmt.Sprintf("%s_%dEventProcessorCount", allocator.name, count), func(b *testing.B) {
				benchmarkParallelAllocation(b, count, allocator.constructor)
			})
		}
	}
}

func benchmarkParallelAllocation(b *testing.B, numberOfEventProcessors int, allocatorConstructor func(logger.Logger, []eventprocessor.EventProcessor) (eventprocessor.Allocator, error)) {
	// Initialize logger
	logger, _ := nucliozap.NewNuclioZapTest("benchmark")

	// Convert eventProcessors to EventProcessors
	eventProcessors := createEventProcessors(numberOfEventProcessors)

	// Create an allocator
	allocator, _ := allocatorConstructor(logger, eventProcessors)

	// Reset the timer to exclude setup time
	b.ResetTimer()

	// Run the benchmark in parallel
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Allocate an event processor
			processor, err := allocator.Allocate(time.Hour)
			if err != nil {
				b.Error(err)
			}
			allocator.Release(processor, false)
		}
	})
}

func createEventProcessors(numEventProcessors int) []eventprocessor.EventProcessor {
	eventProcessors := make([]eventprocessor.EventProcessor, numEventProcessors)
	for i := 0; i < numEventProcessors; i++ {
		eventProcessors[i] = &eventprocessor.MockEventProcessor{}
	}
	return eventProcessors
}
