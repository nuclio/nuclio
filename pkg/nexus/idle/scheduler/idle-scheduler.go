package idle

import (
	"time"

	"github.com/nuclio/nuclio/pkg/nexus/common/models/interfaces"
	structs "github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
	common "github.com/nuclio/nuclio/pkg/nexus/common/scheduler"
)

// IdleScheduler is the scheduler that schedules as soon as plenty of MaxParallelRequests are available
// Purpose: use the resources as much as possible
// More details can be found here: profaastinate/docs/diagrams/uml/activity/idle-schedule.puml
type IdleScheduler struct {
	common.BaseNexusScheduler
}

// NewScheduler creates a new idle scheduler
func NewScheduler(baseNexusScheduler *common.BaseNexusScheduler) *IdleScheduler {
	return &IdleScheduler{
		BaseNexusScheduler: *baseNexusScheduler,
	}
}

// NewDefaultScheduler creates a new idle scheduler with default values
func NewDefaultScheduler(baseNexusScheduler *common.BaseNexusScheduler) *IdleScheduler {
	return NewScheduler(baseNexusScheduler)
}

// Start starts the scheduler
func (ds *IdleScheduler) Start() {
	ds.RunFlag = true

	ds.executeSchedule()
}

// Stop stops the scheduler
func (ds *IdleScheduler) Stop() {
	ds.RunFlag = false
}

// GetStatus returns the running status of the scheduler
func (ds *IdleScheduler) GetStatus() interfaces.SchedulerStatus {
	if ds.RunFlag {
		return interfaces.Running
	} else {
		return interfaces.Stopped
	}
}

// This scheduler is a simple scheduler that sleeps for a given duration
// when awake, it checks if there currently are enough MaxParallelRequests available
// if there are, it pops NexusItems which results in them being executed
func (ds *IdleScheduler) executeSchedule() {
	for ds.RunFlag {
		nextWakeUpTime := time.Now().Add(ds.SleepDuration)

		for ds.Queue.Len() > 0 && ds.MaxParallelRequests.Load() > 0 {
			ds.MaxParallelRequests.Add(-1)
			task := ds.Queue.Pop()

			go func(taskInFunction *structs.NexusItem) {
				defer ds.MaxParallelRequests.Add(1)

				ds.Unpause(taskInFunction.Name)
				ds.CallSynchronized(taskInFunction)
				ds.SendToExecutionChannel(taskInFunction.Name)
			}(task)
		}

		time.Sleep(time.Until(nextWakeUpTime))
	}
}
