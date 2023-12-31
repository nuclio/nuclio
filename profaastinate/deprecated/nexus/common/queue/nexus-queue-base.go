package common

import (
	"container/heap"
	common "github.com/konsumgandalf/profaastinate/nexus/common/models/structs"
	"sync"
	"time"
)

type nexusHeap []*common.NexusItem

func (nxs nexusHeap) Len() int { return len(nxs) }

func (nxs nexusHeap) Swap(i, j int) {
	nxs[i], nxs[j] = nxs[j], nxs[i]
	nxs[i].Index = i
	nxs[j].Index = j
}

func (nxs *nexusHeap) Push(x any) {
	n := len(*nxs)
	NexusEntry := x.(*common.NexusItem)
	NexusEntry.Index = n
	*nxs = append(*nxs, NexusEntry)
}

func (nxs *nexusHeap) Pop() any {
	old := *nxs
	n := len(old)
	NexusEntry := old[n-1]
	old[n-1] = nil        // avoid memory leak
	NexusEntry.Index = -1 // for safety
	*nxs = old[0 : n-1]
	return NexusEntry
}

func (nxs nexusHeap) Less(i, j int) bool {
	return nxs[i].Deadline.Before(nxs[j].Deadline)
}

type NexusQueue struct {
	impl *nexusHeap

	mu *sync.RWMutex
}

func Initialize() *NexusQueue {
	mh := &nexusHeap{}
	heap.Init(mh)

	mutex := &sync.RWMutex{}

	return &NexusQueue{impl: mh, mu: mutex}
}

func (p *NexusQueue) removeAllNotBlocking(nexusItems []*common.NexusItem) {
	for _, item := range nexusItems {
		heap.Remove(p.impl, item.Index)
	}
}

func (p *NexusQueue) getAllItemsUntilDeadlineNotBlocking(deadline time.Time) []*common.NexusItem {
	var items []*common.NexusItem

	for _, item := range *p.impl {
		if item.Deadline.Before(deadline) {
			items = append(items, item)
		} else {
			break
		}
	}

	return items
}
