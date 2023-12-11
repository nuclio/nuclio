package queue

import (
	"container/heap"
	"github.com/konsumgandalf/mpga-protoype-david/pkg/nexus/common/structs"
	"time"
)

type DeadlineItem struct {
	structs.BaseNexusItem
	Deadline time.Time // The priority of the NexusEntry in the queue.
}

type DeadlineQueue struct {
	impl *deadlineHeap
}

func NewDeadlineQueue() *DeadlineQueue {
	mh := &deadlineHeap{}
	heap.Init(mh)
	return &DeadlineQueue{impl: mh}
}

func (p DeadlineQueue) Len() int { return p.impl.Len() }

func (p *DeadlineQueue) Push(el *DeadlineItem) {
	heap.Push(p.impl, el)
}

func (p *DeadlineQueue) Update(el *DeadlineItem, deadline time.Time) {
	heap.Remove(p.impl, el.Index)
	el.Deadline = deadline
	heap.Push(p.impl, el)
}

func (p *DeadlineQueue) Pop() *DeadlineItem {
	el := heap.Pop(p.impl)
	return el.(*DeadlineItem)
}

func (p *DeadlineQueue) Peek() *DeadlineItem {
	return (*p.impl)[0]
}

func (p *DeadlineQueue) Remove(searchID string) {
	for _, item := range *p.impl {
		if item.ID == searchID {
			heap.Remove(p.impl, item.Index)
			return
		}
	}
}

type deadlineHeap []*DeadlineItem

func (nxs deadlineHeap) Len() int { return len(nxs) }

func (nxs deadlineHeap) Swap(i, j int) {
	nxs[i], nxs[j] = nxs[j], nxs[i]
	nxs[i].Index = i
	nxs[j].Index = j
}

func (nxs *deadlineHeap) Push(x any) {
	n := len(*nxs)
	NexusEntry := x.(*DeadlineItem)
	NexusEntry.Index = n
	*nxs = append(*nxs, NexusEntry)
}

func (nxs *deadlineHeap) Pop() any {
	old := *nxs
	n := len(old)
	NexusEntry := old[n-1]
	old[n-1] = nil        // avoid memory leak
	NexusEntry.Index = -1 // for safety
	*nxs = old[0 : n-1]
	return NexusEntry
}

func (nxs deadlineHeap) Less(i, j int) bool {
	return nxs[i].Deadline.Before(nxs[j].Deadline)
}
