package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/configs"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
	queue "github.com/nuclio/nuclio/pkg/nexus/common/queue"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	NUCLIO_NEXUS_REQUEST_URL = "http://host.docker.internal:8070/api/function_invocations"
	EVALUATION_URL           = "http://host.docker.internal:8888/evaluation/invocation"
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
		requestUrl:               NUCLIO_NEXUS_REQUEST_URL,
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

	if strings.ToLower(nexusItem.Request.URL.Host) == "localhost" {
		// TODO add darwin support
		nexusItem.Request.URL.Host = "docker.host.internal"
	}

	log.Println(nexusItem.Request.URL)

	_, err := bs.client.Do(nexusItem.Request)
	if err != nil {
		log.Println("Error sending request to Nuclio:", err)
	} else {
		log.Println("Successfully sent request to Nuclio")
	}

	return
}

func (bs *BaseNexusScheduler) evaluateInvocation(nexusItem *structs.NexusItem) {
	jsonData, err := json.Marshal(nexusItem.Name)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return
	}

	bs.client.Post(EVALUATION_URL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println("Error sending POST request:", err)
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
