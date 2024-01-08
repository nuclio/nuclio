package scheduler

import (
	"github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
	"log"
	"time"

	"github.com/nuclio/nuclio/pkg/nexus/bulk/models"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/interfaces"
	common "github.com/nuclio/nuclio/pkg/nexus/common/scheduler"
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
	ds.RunFlag = true

	ds.executeSchedule()
}

func (ds *BulkScheduler) Stop() {
	ds.RunFlag = false
}

func (ds *BulkScheduler) GetStatus() interfaces.SchedulerStatus {
	if ds.RunFlag {
		return interfaces.Running
	} else {
		return interfaces.Stopped
	}
}

func (ds *BulkScheduler) executeSchedule() {
	for ds.RunFlag {
		if ds.Queue.Len() == 0 || ds.BaseNexusScheduler.MaxParallelRequests.Load() < int32(ds.MinAmountOfBulkItems) {
			// TODO: sleep take care of offset due to processing
			time.Sleep(ds.SleepDuration)
			continue
		}

		log.Println("Checking for bulking")
		if itemsToPop := ds.Queue.GetMostCommonEntryItems(); len(itemsToPop) >= ds.MinAmountOfBulkItems && ds.BaseNexusScheduler.MaxParallelRequests.Load() >= int32(len(itemsToPop)) {
			ds.callAndRemoveItems(itemsToPop)
		} else if ds.BaseNexusScheduler.MaxParallelRequests.Load() >= int32(len(itemsToPop)) {
			time.Sleep(ds.SleepDuration)
		}
	}
}

func (ds *BulkScheduler) callAndRemoveItems(items []*structs.NexusItem) {
	copiedItems := make([]*structs.NexusItem, len(items))
	copy(copiedItems, items)

	for _, item := range copiedItems {

		go func(item *structs.NexusItem) {
			ds.MaxParallelRequests.Add(1)
			defer ds.MaxParallelRequests.Add(-1)

			ds.CallSynchronized(item)
		}(item)

	}
	ds.Queue.RemoveAll(items)
}
