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

package partitionworker

import (
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type Allocator interface {
	AllocateWorker(string, int, *time.Duration) (*worker.Worker, interface{}, error)
	ReleaseWorker(interface{}, *worker.Worker) error
	Stop() error
}

// holds a shared pool of workers for all partitions to use. cannot guarantee that single worker will always
// be used to handle the same partition but offers the best throughput since all partitions of a given topic
// share the same pool of workers
type PooledWorkerAllocator struct {
	logger          logger.Logger
	workerAllocator worker.Allocator
}

func NewPooledWorkerAllocator(parentLogger logger.Logger,
	workerAllocator worker.Allocator) (*PooledWorkerAllocator, error) {

	newPooledWorkerAllocator := PooledWorkerAllocator{
		logger:          parentLogger.GetChild("pooled"),
		workerAllocator: workerAllocator,
	}

	newPooledWorkerAllocator.logger.DebugWith("Created")

	return &newPooledWorkerAllocator, nil
}

func (wa *PooledWorkerAllocator) AllocateWorker(topic string,
	partitionID int,
	timeout *time.Duration) (*worker.Worker, interface{}, error) {

	// regardless of partition, just use the pool
	workerInstance, err := wa.workerAllocator.Allocate(common.GetDurationOrInfinite(timeout))

	return workerInstance, nil, err
}

func (wa *PooledWorkerAllocator) ReleaseWorker(cookie interface{}, workerInstance *worker.Worker) error {
	wa.workerAllocator.Release(workerInstance)

	return nil
}

func (wa *PooledWorkerAllocator) Stop() error {
	return nil
}

// statically maps a given partition to a given. this guarantees that a given partition in a given topic will
// *always* be handled by the same worker of this replica. for functions that benefit from holding in-order state
// this will be useful. however, the cost is throughput - it segments the worker pool such that it's possible
// that a partition mapped to a busy worker will wait processing an event even though there are free workers (which
// are mapped to other partitions)
type StaticWorkerAllocator struct {
	logger          logger.Logger
	workerAllocator worker.Allocator
	workerChans     []chan *worker.Worker

	// for a given topic and partition ID, holds the channel from which the worker that is assigned to this
	// specific topic/partition can be taken. TODO: the partition map *may* be an array for O(1) goodness...
	// but that assumes that partitionID cannot be very high - something I don't know if is true in the real world
	topicPartitionWorkers map[string]map[int]chan *worker.Worker
}

func NewStaticWorkerAllocator(parentLogger logger.Logger,
	workerAllocator worker.Allocator,
	topicPartitionIDs map[string][]int) (*StaticWorkerAllocator, error) {
	var err error

	newStaticWorkerAllocator := StaticWorkerAllocator{
		logger:          parentLogger.GetChild("static"),
		workerAllocator: workerAllocator,
	}

	// given a worker allocator and a topic/partition map - divide the workers we have between all partitions across
	// all topics. there will be most likely many partitions across different topics sharing the same worker
	newStaticWorkerAllocator.workerChans, newStaticWorkerAllocator.topicPartitionWorkers, err = newStaticWorkerAllocator.assignTopicPartitionWorkers(newStaticWorkerAllocator.workerAllocator,
		topicPartitionIDs)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create static worker allocator")
	}

	newStaticWorkerAllocator.logger.DebugWith("Created", "topicPartitionIDs", topicPartitionIDs)

	return &newStaticWorkerAllocator, nil
}

func (wa *StaticWorkerAllocator) AllocateWorker(topic string,
	partitionID int,
	timeout *time.Duration) (*worker.Worker, interface{}, error) {

	// get the channel from which we need to allocate
	workerChan, workerChanFound := wa.topicPartitionWorkers[topic][partitionID]
	if !workerChanFound {
		return nil, nil, errors.Errorf("No worker assigned to this topic/partition (%s/%d)", topic, partitionID)
	}

	var workerInstance *worker.Worker

	if timeout == nil {

		// simply block forever
		workerInstance = <-workerChan
	} else {

		// try to allocate a worker and fall back to default immediately if there's none available
		select {
		case workerInstance = <-workerChan:
		default:

			// if there's no timeout, return now
			if *timeout == 0 {
				return nil, nil, worker.ErrNoAvailableWorkers
			}

			// if there is a timeout, try to allocate while waiting for the time
			// to pass
			select {
			case workerInstance = <-workerChan:
			case <-time.After(*timeout):
				return nil, nil, worker.ErrNoAvailableWorkers
			}
		}
	}

	// return the worker and the channel we got it from as a cookie so that the caller can return it
	return workerInstance, workerChan, nil
}

func (wa *StaticWorkerAllocator) ReleaseWorker(cookie interface{}, workerInstance *worker.Worker) error {

	// try to get the worker chan
	workerChan, cookieIsWorkerChan := cookie.(chan *worker.Worker)
	if !cookieIsWorkerChan {
		return errors.New("Expected cookie to be a worker channel")
	}

	// shove it back to the channel
	workerChan <- workerInstance

	return nil
}

func (wa *StaticWorkerAllocator) Stop() error {
	wa.logger.Debug("Releasing workers back to worker allocator")

	// iterate over worker channels, allocate the worker (it *must* be returned) and release it back to the allocator
	// pool
	for _, workerChan := range wa.workerChans {
		workerInstance := <-workerChan
		wa.workerAllocator.Release(workerInstance)
	}

	wa.logger.Debug("Workers released back to worker allocator", "num", len(wa.workerChans))

	return nil
}

func (wa *StaticWorkerAllocator) assignTopicPartitionWorkers(workerAllocator worker.Allocator,
	topicPartitionIDs map[string][]int) ([]chan *worker.Worker, map[string]map[int]chan *worker.Worker, error) {

	var workerChans []chan *worker.Worker
	topicPartitionWorkers := map[string]map[int]chan *worker.Worker{}

	wa.logger.DebugWith("Assigning topic partition workers", "topicPartitionIDs", topicPartitionIDs)

	// allocate as many workers as we can from the worker pool, shove each one into a buffered channel that
	// can only contain one item and add that to a slice
	for {
		workerInstance, err := workerAllocator.Allocate(0)
		if err == worker.ErrNoAvailableWorkers {
			break
		}

		workerChan := make(chan *worker.Worker, 1)
		workerChan <- workerInstance

		workerChans = append(workerChans, workerChan)
	}

	// shouldn't ever happen, but make sure
	if len(workerChans) == 0 {
		return nil, nil, errors.New("No workers available in worker pool")
	}

	wa.logger.DebugWith("Assigning workers to partition topics", "numWorkers", len(workerChans), "topicPartitionIDs", topicPartitionIDs)

	for topic, topicPartitionIDs := range topicPartitionIDs {
		topicPartitionWorkers[topic] = map[int]chan *worker.Worker{}

		for partitionIdx, topicPartitionID := range topicPartitionIDs {

			// the partition gets assigned a worker according to its index. the partition ID may be a completely
			// different number
			workerChanIdx := partitionIdx % len(workerChans)

			// assign the applicable worker channel for this topic/partition. worker channels can be shared
			// across partitions and topics
			topicPartitionWorkers[topic][topicPartitionID] = workerChans[workerChanIdx]
		}
	}

	return workerChans, topicPartitionWorkers, nil
}
