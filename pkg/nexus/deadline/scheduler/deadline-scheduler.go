package deadline

import (
<<<<<<< HEAD
<<<<<<< HEAD
	common "github.com/nuclio/nuclio/pkg/nexus/common/scheduler"
=======
	common "github.com/nuclio/nuclio/pkg/nexus/common/models"
>>>>>>> ed6969168 (feat(pkg-restful): nexus)
=======
	common "github.com/nuclio/nuclio/pkg/nexus/common/scheduler"
>>>>>>> bbe05e095 (feat(pkg-nexus): models, scheduler, utils)
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
<<<<<<< HEAD
	log.Println("Starting DeadlineScheduler...")
=======
>>>>>>> ed6969168 (feat(pkg-restful): nexus)
	ds.RunFlag = true

	ds.executeSchedule()
}

func (ds *DeadlineScheduler) Stop() {
	ds.RunFlag = false
}

<<<<<<< HEAD
<<<<<<< HEAD
// TODO: fix this please sleep -> something todo until next awakening (do it) -> sleep
=======
>>>>>>> ed6969168 (feat(pkg-restful): nexus)
=======
// TODO: fix this please sleep -> something todo until next awakening (do it) -> sleep
>>>>>>> bf909479b (Changes by jonas)
func (ds *DeadlineScheduler) executeSchedule() {
	for ds.RunFlag {
		if ds.Queue.Len() == 0 {
			time.Sleep(ds.SleepDuration)
			continue
		}

<<<<<<< HEAD
		timeUntilDeadline := ds.Queue.Peek().Deadline.Sub(time.Now())
=======
		log.Println("Checking for expired deadlines...")
		timeUntilDeadline := ds.Queue.Peek().Deadline.Sub(time.Now())
		log.Println(timeUntilDeadline)
>>>>>>> ed6969168 (feat(pkg-restful): nexus)
		if timeUntilDeadline < ds.DeadlineRemovalThreshold {
			ds.Pop()
		}
	}
}
