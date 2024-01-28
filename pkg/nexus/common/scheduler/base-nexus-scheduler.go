package scheduler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/nuclio/nuclio/pkg/nexus/common/models"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/config"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
	queue "github.com/nuclio/nuclio/pkg/nexus/common/queue"
	"github.com/nuclio/nuclio/pkg/nexus/common/utils"
	elastic_deploy "github.com/nuclio/nuclio/pkg/nexus/elastic-deploy"
)

// BaseNexusScheduler is the base scheduler for all schedulers
type BaseNexusScheduler struct {
	// The config of the scheduler (e.g. sleep duration)
	*config.BaseNexusSchedulerConfig
	// The config of the nexus
	*config.NexusConfig

	// The queue of the scheduler
	Queue *queue.NexusQueue
	// The URL to send async requests to
	requestUrl string
	// The client to send async requests with
	client *http.Client
	// The deployer to use for unpausing / resuming functions
	deployer *elastic_deploy.ProElasticDeploy
	executionChannel chan string
}

// NewBaseNexusScheduler creates a new base scheduler
func NewBaseNexusScheduler(queue *queue.NexusQueue, config *config.BaseNexusSchedulerConfig, nexusConfig *config.NexusConfig, client *http.Client, deployer *elastic_deploy.ProElasticDeploy, executionChannel chan string) *BaseNexusScheduler {
	return &BaseNexusScheduler{
		BaseNexusSchedulerConfig: config,
		Queue:                    queue,
		requestUrl:               models.NUCLIO_NEXUS_REQUEST_URL,
		client:                   client,
		NexusConfig:              nexusConfig,
		deployer:                 deployer,
		executionChannel:         executionChannel,
	}
}

// NewDefaultBaseNexusScheduler creates a new base scheduler with default config
func NewDefaultBaseNexusScheduler(queue *queue.NexusQueue, nexusConfig *config.NexusConfig, deployer *elastic_deploy.ProElasticDeploy, executionChannel chan string) *BaseNexusScheduler {
	baseSchedulerConfig := config.NewDefaultBaseNexusSchedulerConfig()
	return NewBaseNexusScheduler(queue, &baseSchedulerConfig, nexusConfig, &http.Client{}, deployer, executionChannel)
}

// Push adds an element to the queue
func (bns *BaseNexusScheduler) Push(elem *structs.NexusItem) {
	bns.Queue.Push(elem)
}

// Pop removes and returns the first element from the queue
func (bns *BaseNexusScheduler) Pop() (nexusItem *structs.NexusItem) {
	bns.MaxParallelRequests.Add(-1)
	defer bns.MaxParallelRequests.Add(1)

	nexusItem = bns.Queue.Pop()

	bns.Unpause(nexusItem.Name)
	bns.CallSynchronized(nexusItem)
	return
}

// Unpause ensures that the function container is running
func (bns *BaseNexusScheduler) Unpause(functionName string) {
	if bns.deployer == nil {
		return
	}

	err := bns.deployer.Unpause(functionName)
	if err != nil {
		fmt.Println("Error unpausing function:", err)
	}
}

func (bns *BaseNexusScheduler) SendToExecutionChannel(functionName string) {
	fmt.Println("Sending to execution channel:", functionName)
	bns.executionChannel <- functionName
}

// CallSynchronized calls the function synchronously on the default nuclio endpoint
func (bns *BaseNexusScheduler) CallSynchronized(nexusItem *structs.NexusItem) {
	newRequest := utils.TransformRequestToClientRequest(nexusItem.Request)

	_, err := bns.client.Do(newRequest)
	if err != nil {
		fmt.Println("Error sending request to Nuclio:", err)
	}
}

// Deprecated: evaluateInvocation evaluates the invocation of a function - It just used for testing
func (bns *BaseNexusScheduler) evaluateInvocation(nexusItem *structs.NexusItem) {
	jsonData, err := json.Marshal(nexusItem.Name)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return
	}

	var evaluationUrl url.URL
	evaluationUrl.Scheme = models.HTTP_SCHEME
	evaluationUrl.Path = models.EVALUATION_PATH
	evaluationUrl.Host = fmt.Sprintf("%s:%s", utils.GetEnvironmentHost(), models.PORT)

	_, postErr := bns.client.Post(evaluationUrl.String(), "application/json", bytes.NewBuffer(jsonData))
	if postErr != nil {
		return
	}
}
