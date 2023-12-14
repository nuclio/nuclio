package models

import (
	"nexus/common/models/configs"
	"nexus/common/models/structs"
	queue "nexus/common/queue"
	"sync"
)

type BaseNexusScheduler struct {
	Queue *queue.NexusQueue
	configs.BaseNexusSchedulerConfig
}

func CreateBaseNexusScheduler(queue *queue.NexusQueue, mu *sync.RWMutex, config configs.BaseNexusSchedulerConfig) *BaseNexusScheduler {
	return &BaseNexusScheduler{
		Queue:                    queue,
		BaseNexusSchedulerConfig: config,
	}
}

func (bns *BaseNexusScheduler) Push(elem *structs.NexusItem) {

	bns.Queue.Push(elem)
}

func (bns *BaseNexusScheduler) Pop() interface{} {
	return bns.Queue.Pop()
}
