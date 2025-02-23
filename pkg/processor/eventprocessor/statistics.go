/*
Copyright 2025 The Nuclio Authors.

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

package eventprocessor

import "sync/atomic"

type Statistics struct {
	EventsHandledSuccess uint64
	EventsHandledError   uint64
}

type AllocatorStatistics struct {
	AllocationCount                       uint64
	AllocationSuccessImmediateTotal       uint64
	AllocationSuccessAfterWaitTotal       uint64
	AllocationTimeoutTotal                uint64
	AllocationWaitDurationMilliSecondsSum uint64
	AllocationObjectsAvailablePercentage  uint64
}

func (s *AllocatorStatistics) DiffFrom(prev *AllocatorStatistics) AllocatorStatistics {

	// atomically load the counters
	currAllocationCount := atomic.LoadUint64(&s.AllocationCount)
	currAllocationSuccessImmediateTotal := atomic.LoadUint64(&s.AllocationSuccessImmediateTotal)
	currAllocationSuccessAfterWaitTotal := atomic.LoadUint64(&s.AllocationSuccessAfterWaitTotal)
	currAllocationTimeoutTotal := atomic.LoadUint64(&s.AllocationTimeoutTotal)
	currAllocationWaitDurationMilliSecondsSum := atomic.LoadUint64(&s.AllocationWaitDurationMilliSecondsSum)
	currAllocationObjectsAvailablePercentage := atomic.LoadUint64(&s.AllocationObjectsAvailablePercentage)

	prevAllocationCount := atomic.LoadUint64(&prev.AllocationCount)
	prevAllocationSuccessImmediateTotal := atomic.LoadUint64(&prev.AllocationSuccessImmediateTotal)
	prevAllocationSuccessAfterWaitTotal := atomic.LoadUint64(&prev.AllocationSuccessAfterWaitTotal)
	prevAllocationTimeoutTotal := atomic.LoadUint64(&prev.AllocationTimeoutTotal)
	prevAllocationWaitDurationMilliSecondsSum := atomic.LoadUint64(&prev.AllocationWaitDurationMilliSecondsSum)
	prevAllocationObjectsAvailablePercentage := atomic.LoadUint64(&prev.AllocationObjectsAvailablePercentage)

	return AllocatorStatistics{
		AllocationCount:                       currAllocationCount - prevAllocationCount,
		AllocationSuccessImmediateTotal:       currAllocationSuccessImmediateTotal - prevAllocationSuccessImmediateTotal,
		AllocationSuccessAfterWaitTotal:       currAllocationSuccessAfterWaitTotal - prevAllocationSuccessAfterWaitTotal,
		AllocationTimeoutTotal:                currAllocationTimeoutTotal - prevAllocationTimeoutTotal,
		AllocationWaitDurationMilliSecondsSum: currAllocationWaitDurationMilliSecondsSum - prevAllocationWaitDurationMilliSecondsSum,
		AllocationObjectsAvailablePercentage:  currAllocationObjectsAvailablePercentage - prevAllocationObjectsAvailablePercentage,
	}
}
