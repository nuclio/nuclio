package deadline

import (
	"fmt"
	"time"

	"github.com/nuclio/nuclio/pkg/nexus/common/models/interfaces"
	common "github.com/nuclio/nuclio/pkg/nexus/common/scheduler"
	"github.com/nuclio/nuclio/pkg/nexus/deadline/models"
)

type DeadlineScheduler struct {
	common.BaseNexusScheduler

	models.DeadlineSchedulerConfig
}

func NewScheduler(baseNexusScheduler *common.BaseNexusScheduler, deadlineConfig models.DeadlineSchedulerConfig) *DeadlineScheduler {

	return &DeadlineScheduler{
		BaseNexusScheduler:      *baseNexusScheduler,
		DeadlineSchedulerConfig: deadlineConfig,
	}
}

func NewDefaultScheduler(baseNexusScheduler *common.BaseNexusScheduler) *DeadlineScheduler {

	return NewScheduler(baseNexusScheduler, *models.NewDefaultDeadlineSchedulerConfig())
}

func (ds *DeadlineScheduler) Start() {
	ds.RunFlag = true

	ds.executeSchedule()
}

func (ds *DeadlineScheduler) Stop() {
	ds.RunFlag = false
}

func (ds *DeadlineScheduler) GetStatus() interfaces.SchedulerStatus {
	if ds.RunFlag {
		return interfaces.Running
	} else {
		return interfaces.Stopped
	}
}

// This scheduler is a simple scheduler that sleeps for a given duration
// when awake, it checks if there are any tasks that are due until the given threshold
// if there are, it pops which results in the task being executed
func (ds *DeadlineScheduler) executeSchedule() {
	for ds.RunFlag {
		nextWakeUpTime := time.Now().Add(ds.SleepDuration)

		removeUntil := time.Now().Add(ds.DeadlineRemovalThreshold)

		for ds.Queue.Len() > 0 &&
			ds.Queue.Peek().Deadline.Before(removeUntil) {

			ds.MaxParallelRequests.Add(-1)
			task := ds.Queue.Pop()

			go func() {
				defer ds.MaxParallelRequests.Add(1)
				ds.CallSynchronized(task)
			}()
		}

		fmt.Println("Sleeping:", time.Until(nextWakeUpTime).Seconds(), "seconds")
		time.Sleep(time.Until(nextWakeUpTime))
	}
}
