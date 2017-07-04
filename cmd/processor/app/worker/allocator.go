package worker

import (
	"errors"
	"sync"
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

func NewSingletonWorkerAllocator(parentLogger logger.Logger, worker *Worker) (WorkerAllocator, error) {

	return &singleton{
		logger: parentLogger.GetChild("singelton_allocator").(logger.Logger),
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
	timerPool  sync.Pool
}

func NewFixedPoolWorkerAllocator(parentLogger logger.Logger, workers []*Worker) (WorkerAllocator, error) {

	newFixedPool := fixedPool{
		logger:     parentLogger.GetChild("fixed_pool_allocator").(logger.Logger),
		workerChan: make(chan *Worker, len(workers)),
		timerPool: sync.Pool{
			New: func() interface{} {
				return time.NewTimer(0)
			},
		},
	}

	// iterate over workers, shove to pool
	for _, workerInstance := range workers {
		newFixedPool.workerChan <- workerInstance
	}

	return &newFixedPool, nil
}

func (fp *fixedPool) Allocate(timeout time.Duration) (*Worker, error) {
	timer := fp.timerPool.Get().(*time.Timer)
	defer fp.timerPool.Put(timer)

	timer.Reset(timeout)
	select {
	case workerInstance := <-fp.workerChan:
		return workerInstance, nil
	case <-timer.C:
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
