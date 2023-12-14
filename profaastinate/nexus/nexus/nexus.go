package nexus

import (
	common "nexus/common/models/structs"
	queue "nexus/common/queue"
)

type Nexus struct {
	Queue *queue.NexusQueue
}

func Init() *Nexus {
	return &Nexus{
		Queue: queue.Init(),
	}
}

func (n *Nexus) Push(elem *common.NexusItem) {
	n.Queue.Push(elem)
}

func (n *Nexus) Pop() interface{} {
	return n.Queue.Pop()
}
