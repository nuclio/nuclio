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
	"github.com/nuclio/nuclio/pkg/processor/eventprocessor"

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
	worker1 := &Worker{}

	allocator := eventprocessor.NewNonBlockingSingletonAllocator(suite.logger, worker1)
	suite.Require().NotNil(allocator)

	// allocate once, time should be ignored
	allocatedWorker, err := allocator.Allocate(time.Hour)
	suite.Require().NoError(err)
	suite.Require().Equal(worker1, allocatedWorker)

	// allocate again, release doesn't need to happen
	allocatedWorker, err = allocator.Allocate(time.Hour)
	suite.Require().NoError(err)
	suite.Require().Equal(worker1, allocatedWorker)

	// release shouldn't do anything
	suite.Require().NotPanics(func() { allocator.Release(worker1) })
}

func (suite *AllocatorTestSuite) TestNonPoolAllocator() {
	eventProcessors := suite.createEventProcessors(2)

	worker1 := eventProcessors[1]
	worker2 := eventProcessors[0]

	allocator, err := eventprocessor.NewNonBlockingPoolAllocator(suite.logger, eventProcessors)
	suite.Require().NoError(err)
	suite.Require().NotNil(allocator)

	// allocate and not release
	firstAllocatedWorker, err := allocator.Allocate(time.Second)
	suite.Require().NoError(err)
	suite.Require().Equal(worker1, firstAllocatedWorker)

	// ensure round robin allocation
	nextAllocatedWorker, err := allocator.Allocate(time.Second)
	suite.Require().NoError(err)
	suite.Require().Equal(worker2, nextAllocatedWorker)

	// allocate 1st again (check round robin + allocation of already allocated worker)
	nextAllocatedWorker, err = allocator.Allocate(time.Second)
	suite.Require().NoError(err)
	suite.Require().Equal(firstAllocatedWorker, nextAllocatedWorker)

	// release the first worker
	allocator.Release(worker1)

	// ensure that allocator allocates the seocnd worker anyway
	nextAllocatedWorker, err = allocator.Allocate(time.Second)
	suite.Require().NoError(err)
	suite.Require().Equal(worker2, nextAllocatedWorker)

	allocator.Release(worker2)
}

func (suite *AllocatorTestSuite) TestFixedBlockingPoolAllocator() {
	eventProcessors := suite.createEventProcessors(2)
	worker2 := eventProcessors[1]

	allocator, err := eventprocessor.NewBlockingPoolAllocator(suite.logger, eventProcessors)
	suite.Require().NoError(err)
	suite.Require().NotNil(allocator)

	// allocate once - should allocate
	firstAllocatedWorker, err := allocator.Allocate(time.Hour)
	suite.Require().NoError(err)
	suite.Require().Contains(eventProcessors, firstAllocatedWorker)

	// allocate again - should allocate other worker
	secondAllocatedWorker, err := allocator.Allocate(time.Hour)
	suite.Require().NoError(err)
	suite.Require().Contains(eventProcessors, secondAllocatedWorker)
	suite.NotEqual(firstAllocatedWorker, secondAllocatedWorker)

	// allocate yet again - should time out
	failedAllocationWorker, err := allocator.Allocate(50 * time.Millisecond)
	suite.Require().Error(err)
	suite.Require().Nil(failedAllocationWorker)

	// release the second worker
	suite.Require().NotPanics(func() { allocator.Release(worker2) })

	// allocate again - should allocate second worker
	thirdAllocatedWorker, err := allocator.Allocate(time.Hour)
	suite.Require().NoError(err)
	suite.Require().Equal(worker2, thirdAllocatedWorker)

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
	workerInstance, err := allocator.Allocate(time.Hour)
	suite.Require().NoError(err)
	suite.Require().Contains(eventProcessors, workerInstance)

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

func (suite *AllocatorTestSuite) createEventProcessors(numEventProcessors int) []eventprocessor.EventProcessor {
	workers := suite.createWorkers(numEventProcessors)
	eventProcessors := make([]eventprocessor.EventProcessor, numEventProcessors)
	for i, worker := range workers {
		eventProcessors[i] = worker
	}
	return eventProcessors
}

func (suite *AllocatorTestSuite) createWorkers(numWorkers int) []*Worker {
	workers := make([]*Worker, numWorkers)
	for i := 0; i < numWorkers; i++ {
		workers[i] = suite.createWorker(i)
	}
	return workers
}

func (suite *AllocatorTestSuite) createWorker(index int) *Worker {
	worker := &Worker{
		index:   index,
		runtime: &MockRuntime{},
		logger:  suite.logger,
	}

	return worker
}

func TestAllocatorTestSuite(t *testing.T) {
	suite.Run(t, new(AllocatorTestSuite))
}

func BenchmarkParallelAllocation(b *testing.B) {
	workerCounts := []int{10, 100, 1000}
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

	for _, count := range workerCounts {
		for _, allocator := range allocatorTypes {
			b.Run(fmt.Sprintf("%s_%dWorkers", allocator.name, count), func(b *testing.B) {
				benchmarkParallelAllocation(b, count, allocator.constructor)
			})
		}
	}
}

func benchmarkParallelAllocation(b *testing.B, numberOfWorkers int, allocatorConstructor func(logger.Logger, []eventprocessor.EventProcessor) (eventprocessor.Allocator, error)) {
	// Initialize logger
	logger, _ := nucliozap.NewNuclioZapTest("benchmark")

	// Create numberOfWorkers workers
	workers := make([]*Worker, numberOfWorkers)
	for i := 0; i < numberOfWorkers; i++ {
		workers[i] = &Worker{index: i, runtime: &MockRuntime{}}
	}

	// Convert workers to EventProcessors
	eventProcessors := make([]eventprocessor.EventProcessor, numberOfWorkers)
	for i, worker := range workers {
		eventProcessors[i] = worker
	}

	// Create an allocator
	allocator, _ := allocatorConstructor(logger, eventProcessors)

	// Reset the timer to exclude setup time
	b.ResetTimer()

	// Run the benchmark in parallel
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Allocate a worker
			processor, err := allocator.Allocate(time.Hour)
			if err != nil {
				b.Error(err)
			}
			allocator.Release(processor)
		}
	})
}
