package nexus

import (
	bulk "github.com/nuclio/nuclio/pkg/nexus/bulk/scheduler"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/interfaces"
	common "github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
	queue "github.com/nuclio/nuclio/pkg/nexus/common/queue"
	"github.com/nuclio/nuclio/pkg/nexus/common/scheduler"
	deadline "github.com/nuclio/nuclio/pkg/nexus/deadline/scheduler"
	"log"
	"sync"
)

type Nexus struct {
	Queue      *queue.NexusQueue
	wg         sync.WaitGroup
	schedulers []interfaces.INexusScheduler
}

func Initialize() (nexus Nexus) {
	nexusQueue := *queue.Initialize()

	nexus = Nexus{
		Queue: &nexusQueue,
	}

	baseScheduler := scheduler.NewDefaultBaseNexusScheduler(&nexusQueue)

	deadlineScheduler := deadline.NewDefaultScheduler(baseScheduler)
	nexus.schedulers = append(nexus.schedulers, deadlineScheduler)

	bulkScheduler := bulk.NewDefaultScheduler(baseScheduler)
	nexus.schedulers = append(nexus.schedulers, bulkScheduler)
	return
}

func (nexus *Nexus) Start() {
	log.Println("Starting Scheduler...")

	nexus.wg.Add(len(nexus.schedulers))
	for _, scheduler := range nexus.schedulers {
		go func(scheduler interfaces.INexusScheduler) {
			defer nexus.wg.Done()
			go scheduler.Start()
		}(scheduler)
	}
	// TODO nexus.wg.Wait()
}

func (n *Nexus) Push(elem *common.NexusItem) {
	n.Queue.Push(elem)
}
