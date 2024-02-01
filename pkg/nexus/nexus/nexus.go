package nexus

import (
	"log"
	"sync"
	"sync/atomic"
	"time"

	bulk "github.com/nuclio/nuclio/pkg/nexus/bulk/scheduler"
	"github.com/nuclio/nuclio/pkg/nexus/common/env"
	"github.com/nuclio/nuclio/pkg/nexus/common/load-balancer"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/config"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/interfaces"
	common "github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
	queue "github.com/nuclio/nuclio/pkg/nexus/common/queue"
	"github.com/nuclio/nuclio/pkg/nexus/common/scheduler"
	deadline "github.com/nuclio/nuclio/pkg/nexus/deadline/scheduler"
	"github.com/nuclio/nuclio/pkg/nexus/elastic-deploy"
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
	// The load balancer of the nexus
	loadBalancer *load_balancer.LoadBalancer

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

	channel := make(chan string, maxParallelRequests.Load()*10)

	nexusConfig := config.NewNexusConfig(&maxParallelRequests, channel)

	nexus = &Nexus{
		queue:       &nexusQueue,
		nexusConfig: &nexusConfig,
	}

	nexus.envRegistry = env.NewEnvRegistry()
	nexus.envRegistry.Initialize()

	nexus.deployer = elastic_deploy.NewProElasticDeployDefault(nexus.envRegistry)
	nexus.deployer.Initialize()

	nexus.loadBalancer = load_balancer.NewLoadBalancer(&maxParallelRequests, channel, 1*time.Second, 40.0, 40.0)
	nexus.loadBalancer.Initialize()

	defaultBaseScheduler := scheduler.NewDefaultBaseNexusScheduler(&nexusQueue, &nexusConfig, nexus.deployer, channel)

	deadlineScheduler := deadline.NewDefaultScheduler(defaultBaseScheduler)
	bulkScheduler := bulk.NewDefaultScheduler(defaultBaseScheduler)

	nexus.schedulers = map[string]interfaces.INexusScheduler{
		"deadline": deadlineScheduler,
		"bulk":     bulkScheduler,
		// "idle": idleScheduler
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

func (nexus *Nexus) StartLoadBalancer() {
	log.Println("Starting LoadBalancer...")
	go nexus.loadBalancer.Start()
}

func (nexus *Nexus) StartDeployer() {
	log.Printf("Starting deployer...\n")
	go nexus.deployer.PauseUnusedFunctionContainers()
}

// SetMaxParallelRequests sets the max parallel requests of the nexus
func (nexus *Nexus) SetMaxParallelRequests(maxParallelRequests int32) {
	nexus.nexusConfig.MaxParallelRequests.Store(maxParallelRequests)
	close(nexus.nexusConfig.FunctionExecutionChannel)
	nexus.nexusConfig.FunctionExecutionChannel = make(chan string, maxParallelRequests*10)
}

// SetTargetLoadCPU sets the target load cpu of the nexus
func (nexus *Nexus) SetTargetLoadCPU(targetLoadCPU float64) {
	nexus.loadBalancer.SetTargetLoadCPU(targetLoadCPU)
}

// SetTargetLoadMemory sets the target load memory of the nexus
func (nexus *Nexus) SetTargetLoadMemory(targetLoadMemory float64) {
	nexus.loadBalancer.SetTargetLoadMemory(targetLoadMemory)
}

// Start starts the nexus and all its schedulers
func (nexus *Nexus) Start() {
	nexus.StartDeployer()
	nexus.StartLoadBalancer()

	log.Println("Starting Scheduler...")

	nexus.wg.Add(len(nexus.schedulers))
	for _, nexusScheduler := range nexus.schedulers {
		go func(scheduler interfaces.INexusScheduler) {
			defer nexus.wg.Done()
			go scheduler.Start()
		}(nexusScheduler)
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
