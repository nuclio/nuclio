//go:build test_unit

/*
Copyright 2017 The Nuclio Authors.

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

package partitionworker

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type partitionWorkerAllocatorTestSuite struct {
	suite.Suite
	ctx    context.Context
	logger logger.Logger
}

func (suite *partitionWorkerAllocatorTestSuite) SetupSuite() {
	var err error

	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)
	suite.ctx = context.Background()
}

func (suite *partitionWorkerAllocatorTestSuite) TestAllocationBlocking() {
	workerAllocator, err := worker.NewFixedPoolWorkerAllocator(suite.logger, suite.createWorkers(2))
	suite.Require().NoError(err)

	partitionWorkerAllocator, err := NewStaticWorkerAllocator(suite.logger,
		workerAllocator,
		map[string][]int{
			"t1": {0, 1, 2, 3},
			"t2": {0},
		})
	suite.Require().NoError(err)

	// allocate a worker - should succeed
	workerInstance, cookie, err := partitionWorkerAllocator.AllocateWorker("t1", 0, nil)
	suite.Require().NoError(err)

	// allocate a worker for the same partition with no timeout - should fail immediately
	noTimeout := time.Duration(0)
	failedWorkerInstance, failedCookie, err := partitionWorkerAllocator.AllocateWorker("t1", 0, &noTimeout)
	suite.Require().Equal(err, worker.ErrNoAvailableWorkers)
	suite.Require().Nil(failedCookie)
	suite.Require().Nil(failedWorkerInstance)

	// allocate a worker for the same partition with a timeout - should fail after a while
	smallTimeout := 2 * time.Second
	failedWorkerInstance, failedCookie, err = partitionWorkerAllocator.AllocateWorker("t1", 0, &smallTimeout)
	suite.Require().Equal(err, worker.ErrNoAvailableWorkers)
	suite.Require().Nil(failedCookie)
	suite.Require().Nil(failedWorkerInstance)

	// release worker
	err = partitionWorkerAllocator.ReleaseWorker(cookie, workerInstance)
	suite.Require().NoError(err)

	// try to allocate again
	workerInstance, cookie, err = partitionWorkerAllocator.AllocateWorker("t1", 0, &smallTimeout)
	suite.Require().NoError(err)
	suite.Require().NotNil(cookie)
	suite.Require().NotNil(workerInstance)
}

// create W workers and add to a static pool
// create a static allocator for N topics, where each topic has a variable number of partitions
// perform repeating allocations making sure that (a) nothing gets stuck and (b) the same worker is allocated
// to a given partition/topic
func (suite *partitionWorkerAllocatorTestSuite) TestStaticAllocatorAllocations() {

	for _, testCase := range []struct {
		name string

		// number of workers to shove into pool
		numWorkers int

		// number of partitions per topic
		topicPartitionIDs map[string][]int

		// expected worker id per topic per partition
		expectedWorkerID map[string][]int
	}{
		{
			name:       "CyclicTwoWorkersAssignment",
			numWorkers: 2,
			topicPartitionIDs: map[string][]int{
				"t1": {0},
				"t2": {10, 20, 30},
			},
			expectedWorkerID: map[string][]int{
				"t1": {0},
				"t2": {0, 1, 0},
			},
		},
		{
			name:       "CyclicFourWorkersAssignment",
			numWorkers: 4,
			topicPartitionIDs: map[string][]int{
				"t1": {0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			},
			expectedWorkerID: map[string][]int{
				"t1": {0, 1, 2, 3, 0, 1, 2, 3, 0, 1, 2, 3, 0, 1, 2, 3},
			},
		},
		{
			name:       "Sanity",
			numWorkers: 4,
			topicPartitionIDs: map[string][]int{
				"t1": {0},
				"t2": {10, 20, 30},
			},
			expectedWorkerID: map[string][]int{
				"t1": {0},
				"t2": {0, 1, 2},
			},
		},
	} {
		suite.Run(testCase.name, func() {
			suite.T().Parallel()
			workerAllocator, err := worker.NewFixedPoolWorkerAllocator(suite.logger,
				suite.createWorkers(testCase.numWorkers))
			suite.Require().NoError(err)

			partitionWorkerAllocator, err := NewStaticWorkerAllocator(suite.logger,
				workerAllocator,
				testCase.topicPartitionIDs)
			suite.Require().NoError(err)

			for repetitions := 0; repetitions < 100000; repetitions++ {
				for topic, partitionIDs := range testCase.topicPartitionIDs {
					for partitionIndex, partitionID := range partitionIDs {
						workerInstance, cookie, err := partitionWorkerAllocator.AllocateWorker(topic, partitionID, nil)
						suite.Require().NoError(err)
						suite.Require().NotNil(workerInstance)
						suite.Require().NotNil(cookie)
						suite.Require().Equal(testCase.expectedWorkerID[topic][partitionIndex], workerInstance.GetIndex())

						partitionWorkerAllocator.ReleaseWorker(cookie, workerInstance) // nolint: errcheck
					}
				}
			}
		})
	}
}

func (suite *partitionWorkerAllocatorTestSuite) TestStaticAllocatorStress() {
	numPartitions := runtime.NumCPU() * 4
	numMessagesPerPartition := numPartitions * 200
	numWorkers := runtime.NumCPU()
	topic := "t1"

	var messageChannels []chan struct{}
	var partitionIDs []int

	for partitionIdx := 0; partitionIdx < numPartitions; partitionIdx++ {
		partitionIDs = append(partitionIDs, partitionIdx)
	}

	// create 16 channels of nothing. this will simulate the partitions
	for messageChannelIdx := 0; messageChannelIdx < numPartitions; messageChannelIdx++ {
		messageChannel := make(chan struct{}, numMessagesPerPartition)

		for messageIdx := 0; messageIdx < numMessagesPerPartition; messageIdx++ {
			messageChannel <- struct{}{}
		}

		messageChannels = append(messageChannels, messageChannel)
	}

	// create a worker allocator with a small number of workers
	workerAllocator, err := worker.NewFixedPoolWorkerAllocator(suite.logger, suite.createWorkers(numWorkers))
	suite.Require().NoError(err)

	// create a static worker allocator
	partitionWorkerAllocator, err := NewStaticWorkerAllocator(suite.logger,
		workerAllocator,
		map[string][]int{
			topic: partitionIDs,
		})
	suite.Require().NoError(err)

	// there's one partition reader per partition, and we need to wait til all partition readers complete
	waitGroup := sync.WaitGroup{}
	waitGroup.Add(numPartitions)

	for partitionReaderIdx := 0; partitionReaderIdx < numPartitions; partitionReaderIdx++ {

		// reads a message from the partition's message channel, allocates a worker to "handle" it,
		// waits a bit and then returns the worker
		go func(partitionReaderIdx int) {
			prevWorkerIndex := -1

			for {
				select {
				case <-messageChannels[partitionReaderIdx]:
					workerInstance, cookie, err := partitionWorkerAllocator.AllocateWorker(topic,
						partitionIDs[partitionReaderIdx],
						nil)
					suite.Require().NoError(err)
					suite.Require().NotNil(workerInstance)

					// make sure we got the same worker
					if prevWorkerIndex != -1 {
						suite.Require().Equal(prevWorkerIndex, workerInstance.GetIndex())
					}

					prevWorkerIndex = workerInstance.GetIndex()

					// wait a bit to simulate processing
					time.Sleep(50 * time.Microsecond)

					// release the worker
					err = partitionWorkerAllocator.ReleaseWorker(cookie, workerInstance)
					suite.Require().NoError(err)

				default:

					// nothing more to read - we're done
					suite.logger.DebugWithCtx(suite.ctx,
						"Done reading from message channel",
						"partitionReaderIdx", partitionReaderIdx)
					waitGroup.Done()

					// exit the loop
					return
				}
			}
		}(partitionReaderIdx)
	}

	// wait for waitgroup with timeout
	doneChan := make(chan struct{})

	go func() {
		waitGroup.Wait()
		doneChan <- struct{}{}
	}()

	select {
	case <-doneChan:
	case <-time.After(45 * time.Second):
		suite.Fail("Expected to process all messages by now")
	}
}

func (suite *partitionWorkerAllocatorTestSuite) createWorkers(numWorkers int) []*worker.Worker {
	var workers []*worker.Worker

	for workerIdx := 0; workerIdx < numWorkers; workerIdx++ {
		workerInstance, err := worker.NewWorker(suite.logger, workerIdx, nil)
		suite.Require().NoError(err)

		workers = append(workers, workerInstance)
	}

	return workers
}

func TestPartitionWorkerAllocator(t *testing.T) {
	suite.Run(t, new(partitionWorkerAllocatorTestSuite))
}
