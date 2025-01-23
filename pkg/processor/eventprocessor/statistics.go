package eventprocessor

import "sync/atomic"

type Statistics struct {
	EventsHandledSuccess uint64
	EventsHandledError   uint64
}

type AllocatorStatistics struct {
	WorkerAllocationCount                       uint64
	WorkerAllocationSuccessImmediateTotal       uint64
	WorkerAllocationSuccessAfterWaitTotal       uint64
	WorkerAllocationTimeoutTotal                uint64
	WorkerAllocationWaitDurationMilliSecondsSum uint64
	WorkerAllocationWorkersAvailablePercentage  uint64
}

func (s *AllocatorStatistics) DiffFrom(prev *AllocatorStatistics) AllocatorStatistics {

	// atomically load the counters
	currWorkerAllocationCount := atomic.LoadUint64(&s.WorkerAllocationCount)
	currWorkerAllocationSuccessImmediateTotal := atomic.LoadUint64(&s.WorkerAllocationSuccessImmediateTotal)
	currWorkerAllocationSuccessAfterWaitTotal := atomic.LoadUint64(&s.WorkerAllocationSuccessAfterWaitTotal)
	currWorkerAllocationTimeoutTotal := atomic.LoadUint64(&s.WorkerAllocationTimeoutTotal)
	currWorkerAllocationWaitDurationMilliSecondsSum := atomic.LoadUint64(&s.WorkerAllocationWaitDurationMilliSecondsSum)
	currWorkerAllocationWorkersAvailablePercentage := atomic.LoadUint64(&s.WorkerAllocationWorkersAvailablePercentage)

	prevWorkerAllocationCount := atomic.LoadUint64(&prev.WorkerAllocationCount)
	prevWorkerAllocationSuccessImmediateTotal := atomic.LoadUint64(&prev.WorkerAllocationSuccessImmediateTotal)
	prevWorkerAllocationSuccessAfterWaitTotal := atomic.LoadUint64(&prev.WorkerAllocationSuccessAfterWaitTotal)
	prevWorkerAllocationTimeoutTotal := atomic.LoadUint64(&prev.WorkerAllocationTimeoutTotal)
	prevWorkerAllocationWaitDurationMilliSecondsSum := atomic.LoadUint64(&prev.WorkerAllocationWaitDurationMilliSecondsSum)
	prevWorkerAllocationWorkersAvailablePercentage := atomic.LoadUint64(&prev.WorkerAllocationWorkersAvailablePercentage)

	return AllocatorStatistics{
		WorkerAllocationCount:                       currWorkerAllocationCount - prevWorkerAllocationCount,
		WorkerAllocationSuccessImmediateTotal:       currWorkerAllocationSuccessImmediateTotal - prevWorkerAllocationSuccessImmediateTotal,
		WorkerAllocationSuccessAfterWaitTotal:       currWorkerAllocationSuccessAfterWaitTotal - prevWorkerAllocationSuccessAfterWaitTotal,
		WorkerAllocationTimeoutTotal:                currWorkerAllocationTimeoutTotal - prevWorkerAllocationTimeoutTotal,
		WorkerAllocationWaitDurationMilliSecondsSum: currWorkerAllocationWaitDurationMilliSecondsSum - prevWorkerAllocationWaitDurationMilliSecondsSum,
		WorkerAllocationWorkersAvailablePercentage:  currWorkerAllocationWorkersAvailablePercentage - prevWorkerAllocationWorkersAvailablePercentage,
	}
}
