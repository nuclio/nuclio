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

	sa := eventprocessor.NewSingletonAllocator(suite.logger, worker1)
	suite.Require().NotNil(sa)

	// allocate once, time should be ignored
	allocatedWorker, err := sa.Allocate(time.Hour)
	suite.Require().NoError(err)
	suite.Require().Equal(worker1, allocatedWorker)

	// allocate again, release doesn't need to happen
	allocatedWorker, err = sa.Allocate(time.Hour)
	suite.Require().NoError(err)
	suite.Require().Equal(worker1, allocatedWorker)

	// release shouldn't do anything
	suite.Require().NotPanics(func() { sa.Release(worker1) })

	suite.Require().False(sa.Shareable())
}

func (suite *AllocatorTestSuite) TestFixedPoolAllocator() {
	worker1 := &Worker{index: 0, runtime: &MockRuntime{}}
	worker2 := &Worker{index: 1, runtime: &MockRuntime{}}
	workers := []*Worker{worker1, worker2}

	eventProcessors := make([]eventprocessor.EventProcessor, 2)
	for i, worker := range workers {
		eventProcessors[i] = worker
	}

	fpa, err := eventprocessor.NewSyncPoolAllocator(suite.logger, eventProcessors)
	suite.Require().NoError(err)
	suite.Require().NotNil(fpa)

	// allocate once - should allocate
	firstAllocatedWorker, err := fpa.Allocate(time.Hour)
	suite.Require().NoError(err)
	suite.Require().Contains(workers, firstAllocatedWorker)

	// allocate again - should allocate other worker
	secondAllocatedWorker, err := fpa.Allocate(time.Hour)
	suite.Require().NoError(err)
	suite.Require().Contains(workers, secondAllocatedWorker)
	suite.NotEqual(firstAllocatedWorker, secondAllocatedWorker)

	// allocate yet again - should time out
	failedAllocationWorker, err := fpa.Allocate(50 * time.Millisecond)
	suite.Require().Error(err)
	suite.Require().Nil(failedAllocationWorker)

	// release the second worker
	suite.Require().NotPanics(func() { fpa.Release(worker2) })

	// allocate again - should allocate second worker
	thirdAllocatedWorker, err := fpa.Allocate(time.Hour)
	suite.Require().NoError(err)
	suite.Require().Equal(worker2, thirdAllocatedWorker)

	suite.Require().True(fpa.Shareable())

	err = common.RetryUntilSuccessful(3*time.Second,
		1*time.Second,
		func() bool {
			statistics := fpa.GetStatistics()
			return statistics.AllocationCount == uint64(4) &&
				statistics.AllocationSuccessImmediateTotal == uint64(3) &&
				statistics.AllocationTimeoutTotal == uint64(1)
		})

	suite.Require().NoError(err)

	// reset objects in allocator (both should become available)
	err = fpa.SetObjects(eventProcessors)
	suite.Require().NoError(err)

	// check allocation
	workerInstance, err := fpa.Allocate(time.Hour)
	suite.Require().NoError(err)
	suite.Require().Contains(workers, workerInstance)

	// check that statistics wasn't reset
	err = common.RetryUntilSuccessful(3*time.Second,
		1*time.Second,
		func() bool {
			statistics := fpa.GetStatistics()
			return statistics.AllocationCount == uint64(5) &&
				statistics.AllocationSuccessImmediateTotal == uint64(4) &&
				statistics.AllocationTimeoutTotal == uint64(1)
		})
	suite.Require().NoError(err)
}

func TestAllocatorTestSuite(t *testing.T) {
	suite.Run(t, new(AllocatorTestSuite))
}

func BenchmarkParallelAllocation100(b *testing.B) {
	benchmarkParallelAllocation(b, 100)
}

func benchmarkParallelAllocation(b *testing.B, numberOfWorkers int) {
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

	// Create a new SyncPoolAllocator
	fpa, _ := eventprocessor.NewSyncPoolAllocator(logger, eventProcessors)

	// Reset the timer to exclude setup time
	b.ResetTimer()

	// Run the benchmark in parallel
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Allocate a worker
			processor, err := fpa.Allocate(time.Hour)
			if err != nil {
				b.Error(err)
			}
			fpa.Release(processor)
		}
	})
}
