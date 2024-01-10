package idle

import (
	"time"

	"github.com/nuclio/nuclio/pkg/nexus/common/models/interfaces"
	common "github.com/nuclio/nuclio/pkg/nexus/common/scheduler"
)

type IdleScheduler struct {
	common.BaseNexusScheduler
}

func NewScheduler(baseNexusScheduler *common.BaseNexusScheduler) *IdleScheduler {
	return &IdleScheduler{
		BaseNexusScheduler: *baseNexusScheduler,
	}
}

func NewDefaultScheduler(baseNexusScheduler *common.BaseNexusScheduler) *IdleScheduler {
	return NewScheduler(baseNexusScheduler)
}

func (ds *IdleScheduler) Start() {
	ds.RunFlag = true

	ds.executeSchedule()
}

func (ds *IdleScheduler) Stop() {
	ds.RunFlag = false
}

func (ds *IdleScheduler) GetStatus() interfaces.SchedulerStatus {
	if ds.RunFlag {
		return interfaces.Running
	} else {
		return interfaces.Stopped
	}
}

func (ds *IdleScheduler) executeSchedule() {
	for ds.RunFlag {
		nextWakeUpTime := time.Now().Add(ds.SleepDuration)

		for ds.Queue.Len() > 0 && ds.MaxParallelRequests.Load() > 0 {
			ds.MaxParallelRequests.Add(-1)
			task := ds.Queue.Pop()
			go func() {
				defer ds.MaxParallelRequests.Add(1)
				ds.CallSynchronized(task)
			}()
		}

		// sleep until the next wake up time
		time.Sleep(time.Until(nextWakeUpTime))
	}
}
