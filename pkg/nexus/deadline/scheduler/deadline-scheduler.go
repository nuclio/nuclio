package deadline

import (
	common "github.com/nuclio/nuclio/pkg/nexus/common/scheduler"
	"github.com/nuclio/nuclio/pkg/nexus/deadline/models"
	"log"
	"time"
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

func (ds *DeadlineScheduler) executeSchedule() {
	for ds.RunFlag {
		if ds.Queue.Len() == 0 {
			time.Sleep(ds.SleepDuration)
			continue
		}

		log.Println("Checking for expired deadlines...")
		timeUntilDeadline := ds.Queue.Peek().Deadline.Sub(time.Now())
		log.Println(timeUntilDeadline)
		if timeUntilDeadline < ds.DeadlineRemovalThreshold {
			ds.Pop()
		}
	}
}
