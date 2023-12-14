package deadline

import (
	common "nexus/common/models"
	"nexus/deadline/models"
	"time"
)

type DeadlineScheduler struct {
	common.BaseNexusScheduler

	models.DeadlineSchedulerConfig
}

func CreateNewScheduler(baseNexusScheduler *common.BaseNexusScheduler, deadlineConfig models.DeadlineSchedulerConfig) *DeadlineScheduler {
	return &DeadlineScheduler{
		BaseNexusScheduler:      *baseNexusScheduler,
		DeadlineSchedulerConfig: deadlineConfig,
	}
}

func (ds *DeadlineScheduler) Start() {
	ds.RunFlag = true

	ds.executeSchedule()
}

func (ds *DeadlineScheduler) Stop() {
	ds.RunFlag = false
}

func (ds *DeadlineScheduler) executeSchedule() {
	for ds.RunFlag {
		if ds.Queue.Len() == 0 {
			time.Sleep(ds.SleepDuration)
			continue
		}

		timeUntilDeadline := ds.Queue.Peek().Deadline.Sub(time.Now())
		if timeUntilDeadline < ds.DeadlineRemovalThreshold {
			ds.Pop()
		}
	}
}
