package scheduler

import (
	"github.com/nuclio/nuclio/pkg/nexus/bulk/models"
<<<<<<< HEAD
<<<<<<< HEAD
	common "github.com/nuclio/nuclio/pkg/nexus/common/scheduler"
=======
	common "github.com/nuclio/nuclio/pkg/nexus/common/models"
>>>>>>> ed6969168 (feat(pkg-restful): nexus)
=======
	common "github.com/nuclio/nuclio/pkg/nexus/common/scheduler"
>>>>>>> bbe05e095 (feat(pkg-nexus): models, scheduler, utils)
	"log"
	"time"
)

type BulkScheduler struct {
	common.BaseNexusScheduler

	models.BulkSchedulerConfig
}

func NewScheduler(baseNexusScheduler *common.BaseNexusScheduler, bulkConfig models.BulkSchedulerConfig) *BulkScheduler {
	return &BulkScheduler{
		BaseNexusScheduler:  *baseNexusScheduler,
		BulkSchedulerConfig: bulkConfig,
	}
}

func NewDefaultScheduler(baseNexusScheduler *common.BaseNexusScheduler) *BulkScheduler {
	return NewScheduler(baseNexusScheduler, *models.NewDefaultBulkSchedulerConfig())
}

func (ds *BulkScheduler) Start() {
<<<<<<< HEAD
<<<<<<< HEAD
	log.Println("Starting BulkScheduler...")
=======
>>>>>>> ed6969168 (feat(pkg-restful): nexus)
=======
	log.Println("Starting BulkScheduler...")
>>>>>>> 51b03bcaa (refactor(pkg-nexus): logging)
	ds.RunFlag = true

	ds.executeSchedule()
}

func (ds *BulkScheduler) Stop() {
	ds.RunFlag = false
}

func (ds *BulkScheduler) executeSchedule() {
	for ds.RunFlag {
		if ds.Queue.Len() == 0 {
			// TODO: sleep take care of offset due to processing
			time.Sleep(ds.SleepDuration)
			continue
		}

<<<<<<< HEAD
<<<<<<< HEAD
		if itemsToPop := ds.Queue.GetMostCommonEntryItems(); len(itemsToPop) >= ds.MinAmountOfBulkItems {
=======
		log.Println("Checking for bulking")
		if itemsToPop := ds.Queue.GetMostCommonEntryItems(); len(itemsToPop) >= ds.MinAmountOfBulkItems {
			log.Println("items with name: " + itemsToPop[0].Name)
>>>>>>> ed6969168 (feat(pkg-restful): nexus)
=======
		if itemsToPop := ds.Queue.GetMostCommonEntryItems(); len(itemsToPop) >= ds.MinAmountOfBulkItems {
>>>>>>> 51b03bcaa (refactor(pkg-nexus): logging)
			ds.Queue.RemoveAll(itemsToPop)
		}
	}
}
