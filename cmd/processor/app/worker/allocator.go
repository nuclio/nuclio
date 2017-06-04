package worker

import (
	"errors"
	"time"

	"github.com/nuclio/nuclio/pkg/logger"
)

type WorkerAllocator interface {

	// allocate a worker
	Allocate(timeout time.Duration) (*Worker, error)

	// release a worker
	Release(worker *Worker)

	// true if the several go routines can share this allocator
	Shareable() bool
}

//
// Singleton worker
// Holds a single worker
//

type singleton struct {
	logger logger.Logger
	worker *Worker
}

func NewSingletonWorkerAllocator(logger logger.Logger, worker *Worker) (WorkerAllocator, error) {

	return &singleton{
		logger: logger.GetChild("singelton_allocator"),
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

//
// Fixed pool of workers
// Holds a fixed number of workers. When a worker is unavailable, caller is blocked
//

type fixedPool struct {
	logger     logger.Logger
	workerChan chan *Worker
}

func NewFixedPoolWorkerAllocator(logger logger.Logger, workers []*Worker) (WorkerAllocator, error) {

	newFixedPool := fixedPool{
		logger:     logger.GetChild("fixed_pool_allocator"),
		workerChan: make(chan *Worker, len(workers)),
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

	// TODO: might be a slight performance drain as this creates an ad hoc channel
	case <-time.After(timeout):
		return nil, errors.New("Timed out waiting for available worker")
	}
}

func (fp *fixedPool) Release(worker *Worker) {
	fp.workerChan <- worker
}

// true if the several go routines can share this allocator
func (fp *fixedPool) Shareable() bool {
	return true
}
