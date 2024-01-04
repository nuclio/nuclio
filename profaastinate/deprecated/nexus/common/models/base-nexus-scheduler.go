package models

import (
	"github.com/konsumgandalf/profaastinate/nexus/common/models/configs"
	"github.com/konsumgandalf/profaastinate/nexus/common/models/structs"
	queue "github.com/konsumgandalf/profaastinate/nexus/common/queue"
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
