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
	"errors"
	"time"

	"github.com/nuclio/logger"
)

// errors
var ErrNoAvailableWorkers = errors.New("No available workers")

type Allocator interface {

	// allocate a worker
	Allocate(timeout time.Duration) (*Worker, error)

	// release a worker
	Release(worker *Worker)

	// true if the several go routines can share this allocator
	Shareable() bool

	// get direct access to all workers for things like management / housekeeping
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

//
// Fixed pool of workers
// Holds a fixed number of workers. When a worker is unavailable, caller is blocked
//

type fixedPool struct {
	logger     logger.Logger
	workerChan chan *Worker
	workers    []*Worker
}

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
