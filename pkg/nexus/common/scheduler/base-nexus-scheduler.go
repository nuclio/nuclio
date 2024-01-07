package scheduler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/nuclio/nuclio/pkg/common/headers"
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

func (bs *BaseNexusScheduler) Pop() (nexusItem *structs.NexusItem) {
	bs.NexusConfig.MaxParallelRequests.Add(-1)
	defer bs.NexusConfig.MaxParallelRequests.Add(1)

	nexusItem = bs.Queue.Pop()

	bs.evaluateInvocation(nexusItem)

	nexusItem.Request.Header.Del(headers.ProcessDeadline)

	var requestUrl url.URL
	requestUrl.Scheme = models.HTTP_SCHEME
	requestUrl.Path = models.NUCLIO_PATH
	requestUrl.Host = fmt.Sprintf("%s:%s", utils.GetEnvironmentHost(), models.PORT)

	newRequest, _ := http.NewRequest(nexusItem.Request.Method, requestUrl.String(), nexusItem.Request.Body)
	newRequest.Header = nexusItem.Request.Header

	_, err := bs.client.Do(newRequest)
	if err != nil {
		fmt.Println("Error sending request to Nuclio:", err)
	}

	return
}

func (bs *BaseNexusScheduler) evaluateInvocation(nexusItem *structs.NexusItem) {
	jsonData, err := json.Marshal(nexusItem.Name)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return
	}

	var evaluationUrl url.URL
	evaluationUrl.Scheme = models.HTTP_SCHEME
	evaluationUrl.Path = models.EVALUATION_PATH
	evaluationUrl.Host = fmt.Sprintf("%s:%s", utils.GetEnvironmentHost(), models.PORT)

	_, postErr := bs.client.Post(evaluationUrl.String(), "application/json", bytes.NewBuffer(jsonData))
	if postErr != nil {
		return
	}
}

func (bs *BaseNexusScheduler) Start() {
	bs.RunFlag = true

	bs.executeSchedule()
}

func (bs *BaseNexusScheduler) Stop() {
	bs.RunFlag = false
}

func (bs *BaseNexusScheduler) executeSchedule() {
	for bs.RunFlag {
		if bs.Queue.Len() == 0 {
			time.Sleep(bs.SleepDuration)
			continue
		}
	}
}
