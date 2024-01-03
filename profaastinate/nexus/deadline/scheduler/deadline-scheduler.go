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

func CreateNewScheduler(baseNexusScheduler *common.BaseNexusScheduler, deadlineConfig *models.DeadlineSchedulerConfig) *DeadlineScheduler {

	if deadlineConfig == nil {
		deadlineConfig = &models.DeadlineSchedulerConfig{
			DeadlineRemovalThreshold: baseNexusScheduler.SleepDuration,
		}
	}

	println("Deadline removal threshold: ", deadlineConfig.DeadlineRemovalThreshold)
	return &DeadlineScheduler{
		BaseNexusScheduler:      *baseNexusScheduler,
		DeadlineSchedulerConfig: *deadlineConfig,
	}
}

func (ds *DeadlineScheduler) Start() {
	ds.RunFlag = true

	ds.executeSchedule()
}

func (ds *DeadlineScheduler) Stop() {
	ds.RunFlag = false
}

func (ds *DeadlineScheduler) ConcurrencySwitch(in <-chan int) {
	for {

		// if in is closed, then we should stop
		if _, ok := <-in; !ok {
			break
		}
		

	}
}

func (ds *DeadlineScheduler) executeSchedule() {
	for ds.RunFlag {
		sleepUntil := time.Now().Add(ds.SleepDuration)

		if ds.Queue.Len() == 0 {
			time.Sleep(ds.SleepDuration)
			continue
		}

		// get all items with deadline before now + threshold
		deadline := time.Now().Add(ds.DeadlineRemovalThreshold)

		items := ds.Queue.PopBulkUntilDeadline(deadline)

		for _, item := range items {
			println("Removing item: ", item.Value)
		}

		time.Sleep(sleepUntil.Sub(time.Now()))
	}
}
