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
