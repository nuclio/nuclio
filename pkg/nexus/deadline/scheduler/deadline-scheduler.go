package deadline

import (
	"fmt"
	"time"

	"github.com/nuclio/nuclio/pkg/nexus/common/models/interfaces"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
	"github.com/nuclio/nuclio/pkg/nexus/common/scheduler"
	"github.com/nuclio/nuclio/pkg/nexus/deadline/models"
)

// DeadlineScheduler is the scheduler that pops tasks that are due until a given threshold
// Purpose: ensure that tasks are executed before their deadline
// More details can be found here: profaastinate/docs/diagrams/uml/activity/deadline-schedule.puml
type DeadlineScheduler struct {
	// BaseNexusScheduler is the base scheduler
	scheduler.BaseNexusScheduler

	// DeadlineSchedulerConfig is the config of the scheduler
	models.DeadlineSchedulerConfig
}

// NewScheduler creates a new deadline scheduler
func NewScheduler(baseNexusScheduler *scheduler.BaseNexusScheduler, deadlineConfig models.DeadlineSchedulerConfig) *DeadlineScheduler {
	return &DeadlineScheduler{
		BaseNexusScheduler:      *baseNexusScheduler,
		DeadlineSchedulerConfig: deadlineConfig,
	}
}

// NewDefaultScheduler creates a new deadline scheduler with default values
// DeadlineRemovalThreshold is set to 10 seconds.
func NewDefaultScheduler(baseNexusScheduler *scheduler.BaseNexusScheduler) *DeadlineScheduler {
	return NewScheduler(baseNexusScheduler, *models.NewDefaultDeadlineSchedulerConfig())
}

// Start starts the scheduler
func (ds *DeadlineScheduler) Start() {
	ds.RunFlag = true

	ds.executeSchedule()
}

// Stop stops the scheduler
func (ds *DeadlineScheduler) Stop() {
	ds.RunFlag = false
}

// GetStatus returns the running status of the scheduler
func (ds *DeadlineScheduler) GetStatus() interfaces.SchedulerStatus {
	if ds.RunFlag {
		return interfaces.Running
	} else {
		return interfaces.Stopped
	}
}

// This scheduler is a simple scheduler that sleeps for a given duration
// when awake, it checks if there are any tasks that are due until the given threshold
// if there are, it pops NexusItems which results in them being executed
func (ds *DeadlineScheduler) executeSchedule() {
	for ds.RunFlag {
		nextWakeUpTime := time.Now().Add(ds.SleepDuration)

		removeUntil := time.Now().Add(ds.DeadlineRemovalThreshold)

		for ds.Queue.Len() > 0 &&
			ds.Queue.Peek().Deadline.Before(removeUntil) {

			ds.CurrentParallelRequests.Add(1)
			task := ds.Queue.Pop()

			go func(taskInFunction *structs.NexusItem) {
				defer ds.CurrentParallelRequests.Add(-1)
				ds.Unpause(taskInFunction.Name)
				ds.SendToExecutionChannel(taskInFunction.Name)
				ds.CallSynchronized(taskInFunction)
			}(task)
		}

		fmt.Println("Sleeping:", time.Until(nextWakeUpTime).Seconds(), "seconds")
		time.Sleep(time.Until(nextWakeUpTime))
	}
}
