package nexus

import (
	bulk "github.com/nuclio/nuclio/pkg/nexus/bulk/scheduler"
<<<<<<< HEAD
<<<<<<< HEAD
	"github.com/nuclio/nuclio/pkg/nexus/common/models/interfaces"
	common "github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
	queue "github.com/nuclio/nuclio/pkg/nexus/common/queue"
	"github.com/nuclio/nuclio/pkg/nexus/common/scheduler"
=======
	"github.com/nuclio/nuclio/pkg/nexus/common/models"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/interfaces"
	common "github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
	queue "github.com/nuclio/nuclio/pkg/nexus/common/queue"
>>>>>>> ed6969168 (feat(pkg-restful): nexus)
=======
	"github.com/nuclio/nuclio/pkg/nexus/common/models/interfaces"
	common "github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
	queue "github.com/nuclio/nuclio/pkg/nexus/common/queue"
	"github.com/nuclio/nuclio/pkg/nexus/common/scheduler"
>>>>>>> bbe05e095 (feat(pkg-nexus): models, scheduler, utils)
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

<<<<<<< HEAD
<<<<<<< HEAD
	baseScheduler := scheduler.NewDefaultBaseNexusScheduler(&nexusQueue)
=======
	baseScheduler := models.NewDefaultBaseNexusScheduler(&nexusQueue)
>>>>>>> ed6969168 (feat(pkg-restful): nexus)
=======
	baseScheduler := scheduler.NewDefaultBaseNexusScheduler(&nexusQueue)
>>>>>>> bbe05e095 (feat(pkg-nexus): models, scheduler, utils)

	deadlineScheduler := deadline.NewDefaultScheduler(baseScheduler)
	nexus.schedulers = append(nexus.schedulers, deadlineScheduler)

	bulkScheduler := bulk.NewDefaultScheduler(baseScheduler)
	nexus.schedulers = append(nexus.schedulers, bulkScheduler)
	return
}

func (nexus *Nexus) Start() {
<<<<<<< HEAD
<<<<<<< HEAD
	log.Println("Starting Nexus...")
=======
	log.Println("Starting Scheduler...")
>>>>>>> ed6969168 (feat(pkg-restful): nexus)
=======
	log.Println("Starting Nexus...")
>>>>>>> 51b03bcaa (refactor(pkg-nexus): logging)

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
