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

import "sync/atomic"

type Statistics struct {
	EventsHandleSuccess uint64
	EventsHandleError   uint64
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
	return AllocatorStatistics{
		WorkerAllocationCount:                       atomic.AddUint64(&s.WorkerAllocationCount, -prev.WorkerAllocationCount),
		WorkerAllocationSuccessImmediateTotal:       atomic.AddUint64(&s.WorkerAllocationSuccessImmediateTotal, -prev.WorkerAllocationSuccessImmediateTotal),
		WorkerAllocationSuccessAfterWaitTotal:       atomic.AddUint64(&s.WorkerAllocationSuccessAfterWaitTotal, -prev.WorkerAllocationSuccessAfterWaitTotal),
		WorkerAllocationTimeoutTotal:                atomic.AddUint64(&s.WorkerAllocationTimeoutTotal, -prev.WorkerAllocationTimeoutTotal),
		WorkerAllocationWaitDurationMilliSecondsSum: atomic.AddUint64(&s.WorkerAllocationWaitDurationMilliSecondsSum, -prev.WorkerAllocationWaitDurationMilliSecondsSum),
		WorkerAllocationWorkersAvailablePercentage:  atomic.AddUint64(&s.WorkerAllocationWorkersAvailablePercentage, -prev.WorkerAllocationWorkersAvailablePercentage),
	}
}
