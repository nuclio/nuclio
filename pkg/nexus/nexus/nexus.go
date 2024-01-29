package nexus

import (
	bulk "github.com/nuclio/nuclio/pkg/nexus/bulk/scheduler"
	"github.com/nuclio/nuclio/pkg/nexus/common/env"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/config"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/interfaces"
	common "github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
	queue "github.com/nuclio/nuclio/pkg/nexus/common/queue"
	"github.com/nuclio/nuclio/pkg/nexus/common/scheduler"
	deadline "github.com/nuclio/nuclio/pkg/nexus/deadline/scheduler"
	elastic_deploy "github.com/nuclio/nuclio/pkg/nexus/elastic-deploy"
	"log"
	"sync"
	"sync/atomic"
)

// Nexus is the main core of the profaastinate system
type Nexus struct {
	// The queue of the nexus
	queue *queue.NexusQueue
	// The config of the nexus
	nexusConfig *config.NexusConfig
	// The environment registry of the nexus
	envRegistry *env.EnvRegistry
	// The deployer of the nexus
	deployer *elastic_deploy.ProElasticDeploy

	// The wait group of the nexus which is used to wait for all schedulers to stop
	wg sync.WaitGroup
	// The schedulers of the nexus
	schedulers map[string]interfaces.INexusScheduler
}

// Initialize initializes the nexus with all its components and schedulers
func Initialize() (nexus *Nexus) {
	nexusQueue := *queue.Initialize()

	var maxParallelRequests atomic.Int32
	maxParallelRequests.Store(1)
	nexusConfig := config.NewNexusConfig(&maxParallelRequests)

	nexus = &Nexus{
		queue:       &nexusQueue,
		nexusConfig: &nexusConfig,
	}

	nexus.envRegistry = env.NewEnvRegistry()
	nexus.envRegistry.Initialize()

	nexus.deployer = elastic_deploy.NewProElasticDeployDefault(nexus.envRegistry)
	nexus.deployer.Initialize()

	defaultBaseScheduler := scheduler.NewDefaultBaseNexusScheduler(&nexusQueue, &nexusConfig, nexus.deployer)

	deadlineScheduler := deadline.NewDefaultScheduler(defaultBaseScheduler)
	bulkScheduler := bulk.NewDefaultScheduler(defaultBaseScheduler)

	nexus.schedulers = map[string]interfaces.INexusScheduler{
		"deadline": deadlineScheduler,
		"bulk":     bulkScheduler,
	}

	return
}

// StartScheduler starts a scheduler from the scheduler map
func (nexus *Nexus) StartScheduler(name string) {
	log.Printf("Starting %s scheduler...", name)
	go nexus.schedulers[name].Start()
}

// StopScheduler stops a scheduler from the scheduler map
func (nexus *Nexus) StopScheduler(name string) {
	log.Printf("Stopping %s scheduler...", name)
	nexus.schedulers[name].Stop()
}

// SetMaxParallelRequests sets the max parallel requests of the nexus
func (nexus *Nexus) SetMaxParallelRequests(maxParallelRequests int32) {
	nexus.nexusConfig.MaxParallelRequests.Store(maxParallelRequests)
}

// Start starts the nexus and all its schedulers
func (nexus *Nexus) Start() {
	log.Printf("Starting deployer...\n")
	go nexus.deployer.PauseUnusedFunctionContainers()

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

// Push adds an asynchronize request send to the dashboard to the queue to be processed
func (n *Nexus) Push(elem *common.NexusItem) {
	n.queue.Push(elem)
}

// GetAllSchedulers returns all schedulers of the nexus
func (n *Nexus) GetAllSchedulers() map[string]interfaces.INexusScheduler {
	return n.schedulers
}
