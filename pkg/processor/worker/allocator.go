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

package worker

import (
	"sync"
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/nuclio/logger"
)

// Common errors
var (
	ErrNoAvailableWorkers = errors.New("No available workers")
	ErrNoDelete           = errors.New("Delete not supported")
)

// Allocator interface
type Allocator interface {
	// Allocate a worker
	Allocate(timeout time.Duration) (*Worker, error)
	// Release a worker
	Release(worker *Worker)
	// Delete a worker from the pool
	Delete(worker *Worker) error
	// Shareable returns true if the several go routines can share this allocator
	Shareable() bool
	// GetWorkers gives direct access to all workers for management/housekeeping
	GetWorkers() []*Worker
}

//
// Singleton worker
// Holds a single worker
//

type singleton struct {
	logger logger.Logger
	worker *Worker
}

// NewSingletonWorkerAllocator returns an allocator with one worker
func NewSingletonWorkerAllocator(parentLogger logger.Logger, worker *Worker) (Allocator, error) {

	return &singleton{
		logger: parentLogger.GetChild("singelton_allocator"),
		worker: worker,
	}, nil
}

func (s *singleton) Allocate(timeout time.Duration) (*Worker, error) {
	return s.worker, nil
}

func (s *singleton) Release(worker *Worker) {
}

// true if the several go routines can share this allocator
func (s *singleton) Shareable() bool {
	return false
}

// get direct access to all workers for things like management / housekeeping
func (s *singleton) GetWorkers() []*Worker {
	return []*Worker{s.worker}
}

func (s *singleton) Delete(worker *Worker) error {
	return ErrNoDelete
}

//
// Fixed pool of workers
// Holds a fixed number of workers. When a worker is unavailable, caller is blocked
//

type fixedPool struct {
	logger     logger.Logger
	workerChan chan *Worker
	workers    []*Worker
}

// NewFixedPoolWorkerAllocator return an allocator with fixed pool of workers
func NewFixedPoolWorkerAllocator(parentLogger logger.Logger, workers []*Worker) (Allocator, error) {

	newFixedPool := fixedPool{
		logger:     parentLogger.GetChild("fixed_pool_allocator"),
		workerChan: make(chan *Worker, len(workers)),
		workers:    workers,
	}

	// iterate over workers, shove to pool
	for _, workerInstance := range workers {
		newFixedPool.workerChan <- workerInstance
	}

	return &newFixedPool, nil
}

func (fp *fixedPool) Allocate(timeout time.Duration) (*Worker, error) {
	select {
	case workerInstance := <-fp.workerChan:
		return workerInstance, nil
	default:
		return nil, ErrNoAvailableWorkers
	}
}

func (fp *fixedPool) Release(worker *Worker) {
	fp.workerChan <- worker
}

// true if the several go routines can share this allocator
func (fp *fixedPool) Shareable() bool {
	return true
}

// get direct access to all workers for things like management / housekeeping
func (fp *fixedPool) GetWorkers() []*Worker {
	return fp.workers
}

func (fp *fixedPool) Delete(worker *Worker) error {
	return ErrNoDelete
}

// flexible pool
type flexPool struct {
	runtimeConfiguration *runtime.Configuration
	logger               logger.Logger
	lock                 sync.Mutex
	freeWorkers          map[*Worker]bool
	allocatedWorkers     map[*Worker]bool
}

// NewFlexPoolWorkerAllocator return a new worker pool with flexible size
func NewFlexPoolWorkerAllocator(parentLogger logger.Logger, runtimeConfiguration *runtime.Configuration, workers []*Worker) (Allocator, error) {
	configurationCopy := *runtimeConfiguration
	flexPool := &flexPool{
		logger:               parentLogger.GetChild("flex_pool_allocator"),
		runtimeConfiguration: &configurationCopy,
		freeWorkers:          make(map[*Worker]bool),
		allocatedWorkers:     make(map[*Worker]bool),
	}

	for _, worker := range workers {
		flexPool.freeWorkers[worker] = true
	}

	return flexPool, nil
}
func (fa *flexPool) nextIndex() int {
	return len(fa.freeWorkers) + len(fa.allocatedWorkers)
}

func (fa *flexPool) Allocate(timeout time.Duration) (*Worker, error) {
	fa.lock.Lock()
	defer fa.lock.Unlock()

	if len(fa.freeWorkers) == 0 {
		worker, err := WorkerFactorySingleton.CreateWorker(fa.logger, fa.nextIndex(), fa.runtimeConfiguration)
		if err != nil {
			return nil, err
		}

		fa.freeWorkers[worker] = true
	}

	for worker := range fa.freeWorkers {
		delete(fa.freeWorkers, worker)
		fa.allocatedWorkers[worker] = true
		return worker, nil
	}

	return nil, ErrNoAvailableWorkers
}

func (fa *flexPool) Release(worker *Worker) {
	fa.lock.Lock()
	defer fa.lock.Unlock()

	if _, ok := fa.allocatedWorkers[worker]; !ok {
		fa.logger.WarnWith("Release of non allocated worker", "index", worker.GetIndex())
		return
	}

	delete(fa.allocatedWorkers, worker)
	fa.freeWorkers[worker] = true
}

func (fa *flexPool) Shareable() bool {
	return true
}

func (fa *flexPool) GetWorkers() []*Worker {
	fa.lock.Lock()
	defer fa.lock.Unlock()

	workers := make([]*Worker, 0, len(fa.freeWorkers)+len(fa.allocatedWorkers))
	for worker := range fa.freeWorkers {
		workers = append(workers, worker)
	}

	for worker := range fa.allocatedWorkers {
		workers = append(workers, worker)
	}

	return workers
}

func (fa *flexPool) SetWorkers(workers []*Worker) error {
	fa.lock.Lock()
	defer fa.lock.Unlock()

	fa.freeWorkers = make(map[*Worker]bool)
	fa.allocatedWorkers = make(map[*Worker]bool)

	for _, worker := range workers {
		fa.freeWorkers[worker] = true
	}

	return nil
}

func (fa *flexPool) Delete(worker *Worker) error {
	fa.lock.Lock()
	defer fa.lock.Unlock()

	if _, ok := fa.allocatedWorkers[worker]; ok {
		fa.logger.ErrorWith("Releasing allocated worker", "worker", worker)
		return errors.Errorf("Releasing allocated worker - %#v", worker)
	}

	if _, ok := fa.freeWorkers[worker]; !ok {
		fa.logger.ErrorWith("Releasing unknown worker", "worker", worker)
		return errors.Errorf("Releasing unknown worker - %#v", worker)
	}

	delete(fa.freeWorkers, worker)

	return nil
}
