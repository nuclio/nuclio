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

package statistics

type EventProcessingStatistics struct {
	EventsHandledSuccess                uint64
	EventsHandledError                  uint64
	EventsStreamingStartedSuccessfully  uint64
	EventsStreamingStartedError         uint64
	EventsStreamingFinishedSuccessfully uint64
}

// AllocatorStatistics is not a safe statistics object and should be used only to copy safe object to it and return to outside
// so it can later be taken by metric gatherers
type AllocatorStatistics struct {
	AllocationCount                       uint64
	AllocationSuccessImmediateTotal       uint64
	AllocationSuccessAfterWaitTotal       uint64
	AllocationTimeoutTotal                uint64
	AllocationWaitDurationMilliSecondsSum uint64
	AllocationObjectsAvailablePercentage  uint64
}

func (s *AllocatorStatistics) DiffFrom(prev *AllocatorStatistics) AllocatorStatistics {

	return AllocatorStatistics{
		AllocationCount:                       s.AllocationCount - prev.AllocationCount,
		AllocationSuccessImmediateTotal:       s.AllocationSuccessImmediateTotal - prev.AllocationSuccessImmediateTotal,
		AllocationSuccessAfterWaitTotal:       s.AllocationSuccessAfterWaitTotal - prev.AllocationSuccessAfterWaitTotal,
		AllocationTimeoutTotal:                s.AllocationTimeoutTotal - prev.AllocationTimeoutTotal,
		AllocationWaitDurationMilliSecondsSum: s.AllocationWaitDurationMilliSecondsSum - prev.AllocationWaitDurationMilliSecondsSum,
		AllocationObjectsAvailablePercentage:  s.AllocationObjectsAvailablePercentage - prev.AllocationObjectsAvailablePercentage,
	}
}
