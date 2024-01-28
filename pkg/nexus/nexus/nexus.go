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

type Nexus struct {
	queue        *queue.NexusQueue
	wg           sync.WaitGroup
	schedulers   map[string]interfaces.INexusScheduler
	nexusConfig  *config.NexusConfig
	envRegistry  *env.EnvRegistry
	deployer     *elastic_deploy.ProElasticDeploy
	loadBalancer *load_balancer.LoadBalancer
}

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

func (nexus *Nexus) StartScheduler(name string) {
	log.Printf("Starting %s scheduler...", name)
	go nexus.schedulers[name].Start()
}

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

func (nexus *Nexus) SetMaxParallelRequests(maxParallelRequests int32) {
	nexus.nexusConfig.MaxParallelRequests.Store(maxParallelRequests)
	close(nexus.nexusConfig.FunctionExecutionChannel)
	nexus.nexusConfig.FunctionExecutionChannel = make(chan string, maxParallelRequests*10)
}

func (nexus *Nexus) SetTargetLoadCPU(targetLoadCPU float64) {
	nexus.loadBalancer.SetTargetLoadCPU(targetLoadCPU)
}

func (nexus *Nexus) SetTargetLoadMemory(targetLoadMemory float64) {
	nexus.loadBalancer.SetTargetLoadMemory(targetLoadMemory)
}

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

func (n *Nexus) Push(elem *common.NexusItem) {
	n.queue.Push(elem)
}

func (n *Nexus) GetAllSchedulers() map[string]interfaces.INexusScheduler {
	return n.schedulers
}
