package nexus

import (
	bulk "github.com/nuclio/nuclio/pkg/nexus/bulk/scheduler"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/config"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/interfaces"
	common "github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
	queue "github.com/nuclio/nuclio/pkg/nexus/common/queue"
	"github.com/nuclio/nuclio/pkg/nexus/common/scheduler"
	deadline "github.com/nuclio/nuclio/pkg/nexus/deadline/scheduler"
	"log"
	"sync"
	"sync/atomic"
)

type Nexus struct {
	queue            *queue.NexusQueue
	wg               sync.WaitGroup
	schedulers       map[string]interfaces.INexusScheduler
	parallelRequests *atomic.Int32
	nexusConfig      *config.NexusConfig
}

func Initialize() (nexus *Nexus) {
	nexusQueue := *queue.Initialize()

	var maxParallelRequests atomic.Int32
	maxParallelRequests.Store(1)
	nexusConfig := config.NewNexusConfig(&maxParallelRequests)

	nexus = &Nexus{
		queue:       &nexusQueue,
		nexusConfig: &nexusConfig,
	}

	defaultBaseScheduler := scheduler.NewDefaultBaseNexusScheduler(&nexusQueue, &nexusConfig)

	deadlineScheduler := deadline.NewDefaultScheduler(defaultBaseScheduler)
	bulkScheduler := bulk.NewDefaultScheduler(defaultBaseScheduler)

	nexus.schedulers = map[string]interfaces.INexusScheduler{
		"deadline": deadlineScheduler,
		"bulk":     bulkScheduler,
	}

	return
}

func (nexus *Nexus) StartScheduler(name string) {
	log.Printf("Starting %s scheduler...", name)
	go nexus.schedulers[name].Start()
}

func (nexus *Nexus) StopScheduler(name string) {
	log.Printf("Stopping %s scheduler...", name)
	nexus.schedulers[name].Stop()
}

func (nexus *Nexus) SetMaxParallelRequests(maxParallelRequests int32) {
	nexus.nexusConfig.MaxParallelRequests.Store(maxParallelRequests)
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
	n.queue.Push(elem)
}

func (n *Nexus) GetAllSchedulers() map[string]interfaces.INexusScheduler {
	return n.schedulers
}
