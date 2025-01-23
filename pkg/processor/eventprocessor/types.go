package eventprocessor

import (
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"sync/atomic"
	"time"
)

var ErrNoAvailableWorkers = errors.New("No available workers")

type Allocator interface {
	Allocate(timeout time.Duration) (*EventProcessor, error)

	Release() error
}

type EventProcessor interface {
	ProcessEvent(event nuclio.Event, functionLogger logger.Logger) (interface{}, error)
}

type NonBlockingPoolAllocator struct {
	logger      logger.Logger
	objectsChan chan *EventProcessor
	objects     []*EventProcessor
	statistics  AllocatorStatistics
}

func (nba *NonBlockingPoolAllocator) Allocate(timeout time.Duration) (*EventProcessor, error) {
	select {
	case obj := <-nba.objectsChan:
		defer func() {
			nba.objectsChan <- obj
		}()
		return obj, nil
	default:
		return nil, errors.New("No event processors found")
	}
}

// Release does nothing in non-blocking allocator
func (nba *NonBlockingPoolAllocator) Release() error {
	return nil
}

type BlockingPoolAllocator struct {
	logger      logger.Logger
	objectsChan chan *EventProcessor
	objects     []*EventProcessor
	statistics  AllocatorStatistics
}

func (ba *BlockingPoolAllocator) Allocate(timeout time.Duration) (*EventProcessor, error) {

	// we don't want to completely lock here, but we'll use atomic to inc counters where possible
	atomic.AddUint64(&ba.statistics.WorkerAllocationCount, 1)

	// get total number of workers
	totalNumberWorkers := len(ba.objects)
	currentNumberOfAvailableWorkers := len(ba.objectsChan)
	percentageOfAvailableWorkers := float64(currentNumberOfAvailableWorkers*100.0) / float64(totalNumberWorkers)

	// measure how many workers are available in the queue while we're allocating
	atomic.AddUint64(&ba.statistics.WorkerAllocationWorkersAvailablePercentage, uint64(percentageOfAvailableWorkers))

	// try to allocate a worker and fall back to default immediately if there's none available
	select {
	case workerInstance := <-ba.objectsChan:
		atomic.AddUint64(&ba.statistics.WorkerAllocationSuccessImmediateTotal, 1)

		return workerInstance, nil
	default:

		// if there's no timeout, return now
		if timeout == 0 {
			atomic.AddUint64(&ba.statistics.WorkerAllocationTimeoutTotal, 1)
			return nil, ErrNoAvailableWorkers
		}

		waitStartAt := time.Now()

		// if there is a timeout, try to allocate while waiting for the time
		// to pass
		select {
		case workerInstance := <-ba.objectsChan:
			atomic.AddUint64(&ba.statistics.WorkerAllocationSuccessAfterWaitTotal, 1)
			atomic.AddUint64(&ba.statistics.WorkerAllocationWaitDurationMilliSecondsSum,
				uint64(time.Since(waitStartAt).Nanoseconds()/1e6))
			return workerInstance, nil
		case <-time.After(timeout):
			atomic.AddUint64(&ba.statistics.WorkerAllocationTimeoutTotal, 1)
			return nil, ErrNoAvailableWorkers
		}
	}
}
