package scheduler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/nuclio/nuclio/pkg/nexus/common/models"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/config"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
	queue "github.com/nuclio/nuclio/pkg/nexus/common/queue"
	"github.com/nuclio/nuclio/pkg/nexus/common/utils"
)

type BaseNexusScheduler struct {
	*config.BaseNexusSchedulerConfig
	*config.NexusConfig

	Queue      *queue.NexusQueue
	requestUrl string
	client     *http.Client
}

func NewBaseNexusScheduler(queue *queue.NexusQueue, config *config.BaseNexusSchedulerConfig, nexusConfig *config.NexusConfig) *BaseNexusScheduler {
	return &BaseNexusScheduler{
		BaseNexusSchedulerConfig: config,
		Queue:                    queue,
		requestUrl:               models.NUCLIO_NEXUS_REQUEST_URL,
		client:                   &http.Client{},
		NexusConfig:              nexusConfig,
	}
}

func NewDefaultBaseNexusScheduler(queue *queue.NexusQueue, nexusConfig *config.NexusConfig) *BaseNexusScheduler {
	baseSchedulerConfig := config.NewDefaultBaseNexusSchedulerConfig()
	return NewBaseNexusScheduler(queue, &baseSchedulerConfig, nexusConfig)
}

func (bns *BaseNexusScheduler) Push(elem *structs.NexusItem) {
	bns.Queue.Push(elem)
}

func (bns *BaseNexusScheduler) Pop() (nexusItem *structs.NexusItem) {
	bns.NexusConfig.MaxParallelRequests.Add(-1)
	defer bns.NexusConfig.MaxParallelRequests.Add(1)

	nexusItem = bns.Queue.Pop()

	bns.evaluateInvocation(nexusItem)
	bns.CallSynchronized(nexusItem)
	return
}

func (bns *BaseNexusScheduler) CallSynchronized(nexusItem *structs.NexusItem) {
	newRequest := utils.TransformRequestToClientRequest(nexusItem.Request)

	_, err := bns.client.Do(newRequest)
	if err != nil {
		fmt.Println("Error sending request to Nuclio:", err)
	}
}

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

func (bns *BaseNexusScheduler) Start() {
	bns.RunFlag = true

	bns.executeSchedule()
}

func (bns *BaseNexusScheduler) Stop() {
	bns.RunFlag = false
}

func (bns *BaseNexusScheduler) executeSchedule() {
	for bns.RunFlag {
		if bns.Queue.Len() == 0 {
			time.Sleep(bns.SleepDuration)
			continue
		}
	}
}
