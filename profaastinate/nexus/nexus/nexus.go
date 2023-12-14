package nexus

import (
	common "nexus/common/models/structs"
	queue "nexus/common/queue"
	"sync"
)

type Nexus struct {
	Queue *queue.NexusQueue
	Mu    *sync.RWMutex
}

func Init() *Nexus {
	return &Nexus{
		Queue: queue.Init(),
		Mu:    &sync.RWMutex{},
	}
}

func (n *Nexus) Push(elem *common.NexusItem) {
	n.Mu.Lock()
	defer n.Mu.Unlock()

	n.Queue.Push(elem)
}

func (n *Nexus) Pop() interface{} {
	n.Mu.Lock()
	defer n.Mu.Unlock()

	return n.Queue.Pop()
}
