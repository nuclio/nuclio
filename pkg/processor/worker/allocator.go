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
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/nuclio/nuclio/pkg/errgroup"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

var ErrNoAvailableWorkers = errors.New("No available workers")
var ErrAllWorkersAreTerminated = errors.New("All workers are terminated")

type Allocator interface {

	// Allocate allocates a worker
	Allocate(timeout time.Duration) (*Worker, error)

	// Release releases a worker
	Release(worker *Worker)

	// Shareable returns true if the several go routines can share this allocator
	Shareable() bool

	// GetWorkers gets direct access to all workers for things like management / housekeeping
	GetWorkers() []*Worker

	// GetNumWorkersAvailable gets number of workers available in the allocator
	GetNumWorkersAvailable() int

	// GetStatistics returns worker allocator statistics
	GetStatistics() *AllocatorStatistics

	// SignalDraining signals all workers to drain events
	SignalDraining() error

	// SignalContinue signals all workers to continue event processing
	SignalContinue() error

	// SignalTermination signals all workers to terminate
	SignalTermination() error

	// IsTerminated returns true if all workers are terminated
	IsTerminated() bool
}

//
// Singleton worker
// Holds a single worker
//

type singleton struct {

	// accessed atomically, keep as first field for alignment
	statistics AllocatorStatistics

	logger       logger.Logger
	worker       *Worker
	isTerminated bool
}

func NewSingletonWorkerAllocator(parentLogger logger.Logger, worker *Worker) (Allocator, error) {

	return &singleton{
		logger: parentLogger.GetChild("singelton_allocator"),
		worker: worker,
	}, nil
}

func (s *singleton) Allocate(time.Duration) (*Worker, error) {
	if s.isTerminated {
		return nil, ErrAllWorkersAreTerminated
	}
	return s.worker, nil
}

func (s *singleton) Release(*Worker) {
}

func (s *singleton) Shareable() bool {
	return false
}

func (s *singleton) GetWorkers() []*Worker {
	return []*Worker{s.worker}
}

func (s *singleton) GetNumWorkersAvailable() int {
	return 1
}

// GetStatistics returns worker allocator statistics
func (s *singleton) GetStatistics() *AllocatorStatistics {
	return &s.statistics
}

func (s *singleton) SignalDraining() error {
	return s.worker.Drain()
}

func (s *singleton) SignalContinue() error {
	return s.worker.Continue()
}

func (s *singleton) SignalTermination() error {
	s.isTerminated = true
	return s.worker.Terminate()
}

func (s *singleton) IsTerminated() bool {
	return s.isTerminated
}

//
// Fixed pool of workers
// Holds a fixed number of workers. When a worker is unavailable, caller is blocked
//

type fixedPool struct {

	// accessed atomically, keep as first field for alignment
	statistics AllocatorStatistics

	logger       logger.Logger
	workerChan   chan *Worker
	workers      []*Worker
	isTerminated bool
}

func NewFixedPoolWorkerAllocator(parentLogger logger.Logger, workers []*Worker) (Allocator, error) {

	newFixedPool := fixedPool{
		logger:     parentLogger.GetChild("fixed_pool_allocator"),
		workerChan: make(chan *Worker, len(workers)),
		workers:    workers,
		statistics: AllocatorStatistics{},
	}

	// iterate over workers, shove to pool
	for _, workerInstance := range workers {
		newFixedPool.workerChan <- workerInstance
	}

	return &newFixedPool, nil
}

func (fp *fixedPool) Allocate(timeout time.Duration) (*Worker, error) {
	if fp.isTerminated {
		return nil, ErrAllWorkersAreTerminated
	}

	// we don't want to completely lock here, but we'll use atomic to inc counters where possible
	atomic.AddUint64(&fp.statistics.WorkerAllocationCount, 1)

	// get total number of workers
	totalNumberWorkers := len(fp.workers)
	currentNumberOfAvailableWorkers := len(fp.workerChan)
	percentageOfAvailableWorkers := float64(currentNumberOfAvailableWorkers*100.0) / float64(totalNumberWorkers)

	// measure how many workers are available in the queue while we're allocating
	atomic.AddUint64(&fp.statistics.WorkerAllocationWorkersAvailablePercentage, uint64(percentageOfAvailableWorkers))

	// try to allocate a worker and fall back to default immediately if there's none available
	select {
	case workerInstance := <-fp.workerChan:
		atomic.AddUint64(&fp.statistics.WorkerAllocationSuccessImmediateTotal, 1)

		return workerInstance, nil
	default:

		// if there's no timeout, return now
		if timeout == 0 {
			atomic.AddUint64(&fp.statistics.WorkerAllocationTimeoutTotal, 1)
			return nil, ErrNoAvailableWorkers
		}

		waitStartAt := time.Now()

		// if there is a timeout, try to allocate while waiting for the time
		// to pass
		select {
		case workerInstance := <-fp.workerChan:
			atomic.AddUint64(&fp.statistics.WorkerAllocationSuccessAfterWaitTotal, 1)
			atomic.AddUint64(&fp.statistics.WorkerAllocationWaitDurationMilliSecondsSum,
				uint64(time.Since(waitStartAt).Nanoseconds()/1e6))
			return workerInstance, nil
		case <-time.After(timeout):
			atomic.AddUint64(&fp.statistics.WorkerAllocationTimeoutTotal, 1)
			return nil, ErrNoAvailableWorkers
		}
	}
}

func (fp *fixedPool) Release(worker *Worker) {
	fp.workerChan <- worker
}

func (fp *fixedPool) Shareable() bool {
	return true
}

func (fp *fixedPool) GetWorkers() []*Worker {
	return fp.workers
}

func (fp *fixedPool) GetNumWorkersAvailable() int {
	return len(fp.workerChan)
}

// GetStatistics returns worker allocator statistics
func (fp *fixedPool) GetStatistics() *AllocatorStatistics {
	return &fp.statistics
}

func (fp *fixedPool) SignalDraining() error {
	errGroup, _ := errgroup.WithContext(context.Background(), fp.logger)

	for _, workerInstance := range fp.GetWorkers() {
		workerInstance := workerInstance

		errGroup.Go(fmt.Sprintf("Drain worker %d", workerInstance.GetIndex()), func() error {
			// if worker is not already drained, signal it to drain events
			if err := workerInstance.Drain(); err != nil {
				return errors.Wrapf(err, "Failed to signal worker %d to drain events", workerInstance.GetIndex())
			}
			return nil
		})
	}

	if err := errGroup.Wait(); err != nil {
		return errors.Wrap(err, "At least one worker failed to drain")
	}

	return nil
}

func (fp *fixedPool) SignalContinue() error {
	errGroup, _ := errgroup.WithContext(context.Background(), fp.logger)

	for _, workerInstance := range fp.GetWorkers() {
		workerInstance := workerInstance

		errGroup.Go(fmt.Sprintf("Send continue signal to worker %d", workerInstance.GetIndex()), func() error {
			if err := workerInstance.Continue(); err != nil {
				return errors.Wrapf(err, "Failed to signal worker %d to continue event processing", workerInstance.GetIndex())
			}
			return nil
		})
	}

	if err := errGroup.Wait(); err != nil {
		return errors.Wrap(err, "At least one worker failed to continue")
	}

	return nil
}

func (fp *fixedPool) SignalTermination() error {
	errGroup, _ := errgroup.WithContext(context.Background(), fp.logger)
	fp.isTerminated = true
	for _, workerInstance := range fp.GetWorkers() {
		workerInstance := workerInstance

		errGroup.Go(fmt.Sprintf("Terminate worker %d", workerInstance.GetIndex()), func() error {

			if err := workerInstance.Terminate(); err != nil {
				return errors.Wrapf(err, "Failed to signal worker %d to terminate", workerInstance.GetIndex())
			}
			return nil
		})
	}

	if err := errGroup.Wait(); err != nil {
		return errors.Wrap(err, "At least one worker failed to terminate")
	}

	return nil
}

func (fp *fixedPool) IsTerminated() bool {
	return fp.isTerminated
}
