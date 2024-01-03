package scheduler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/nexus/common/models"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/configs"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
	queue "github.com/nuclio/nuclio/pkg/nexus/common/queue"
	"github.com/nuclio/nuclio/pkg/nexus/common/utils"
)

type BaseNexusScheduler struct {
	Queue *queue.NexusQueue
	configs.BaseNexusSchedulerConfig
	requestUrl string
	client     *http.Client
}

func NewBaseNexusScheduler(queue *queue.NexusQueue, config configs.BaseNexusSchedulerConfig) *BaseNexusScheduler {
	return &BaseNexusScheduler{
		Queue:                    queue,
		BaseNexusSchedulerConfig: config,
		requestUrl:               models.NUCLIO_NEXUS_REQUEST_URL,
		client:                   &http.Client{},
	}
}

func NewDefaultBaseNexusScheduler(queue *queue.NexusQueue) *BaseNexusScheduler {
	return NewBaseNexusScheduler(queue, configs.NewDefaultBaseNexusSchedulerConfig())
}

func (bns *BaseNexusScheduler) Push(elem *structs.NexusItem) {
	bns.Queue.Push(elem)
}

func (bs *BaseNexusScheduler) Pop() (nexusItem *structs.NexusItem) {
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

	bs.client.Post(evaluationUrl.String(), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
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
