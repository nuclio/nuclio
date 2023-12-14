package models

import (
	"nexus/common/models/configs"
	"nexus/common/models/structs"
	queue "nexus/common/queue"
	"sync"
)

type BaseNexusScheduler struct {
	Queue *queue.NexusQueue
	Mu    *sync.RWMutex
	configs.BaseNexusSchedulerConfig
}

func CreateBaseNexusScheduler(queue *queue.NexusQueue, mu *sync.RWMutex, config configs.BaseNexusSchedulerConfig) *BaseNexusScheduler {
	return &BaseNexusScheduler{
		Queue:                    queue,
		Mu:                       mu,
		BaseNexusSchedulerConfig: config,
	}
}

func (bns *BaseNexusScheduler) Push(elem *structs.NexusItem) {
	bns.Mu.Lock()
	defer bns.Mu.Unlock()

	bns.Queue.Push(elem)
}

func (bns *BaseNexusScheduler) Pop() interface{} {
	bns.Mu.Lock()
	defer bns.Mu.Unlock()

	return bns.Queue.Pop()
}
