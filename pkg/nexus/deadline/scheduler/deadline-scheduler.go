package deadline

import (
	"log"
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

// TODO: fix this please sleep -> something todo until next awakening (do it) -> sleep
func (ds *DeadlineScheduler) executeSchedule() {
	for ds.RunFlag {
		if ds.Queue.Len() == 0 {
			println("Sleeping for ", ds.SleepDuration.Milliseconds(), " milliseconds")
			time.Sleep(ds.SleepDuration)
			continue

		}

		log.Println("Checking for expired deadlines...")
		timeUntilDeadline := ds.Queue.Peek().Deadline.Sub(time.Now())
		log.Println(timeUntilDeadline)
		if timeUntilDeadline < ds.DeadlineRemovalThreshold {
			println("Removing item from queue")
			ds.Pop()
		}
	}
}
