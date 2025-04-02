package metrics

type EventProcessingStatistics struct {
	EventsHandledSuccess uint64
	EventsHandledError   uint64
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
