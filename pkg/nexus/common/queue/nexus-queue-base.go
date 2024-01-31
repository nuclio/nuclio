package common

import (
	"container/heap"
	"sync"
	"time"

	"github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
)

// nexusHeap is the heap that holds the NexusEntries
type nexusHeap []*structs.NexusItem

// Len returns the length of the queue
func (nxs nexusHeap) Len() int { return len(nxs) }

// Swap changes the position of two items in the queue with each other
func (nxs nexusHeap) Swap(i, j int) {
	nxs[i], nxs[j] = nxs[j], nxs[i]
	nxs[i].Index = i
	nxs[j].Index = j
}

// Push adds an item to the queue
func (nxs *nexusHeap) Push(x any) {
	n := len(*nxs)
	NexusEntry := x.(*structs.NexusItem)
	NexusEntry.Index = n
	*nxs = append(*nxs, NexusEntry)
}

// Pop removes and returns the first item from the queue
func (nxs *nexusHeap) Pop() any {
	old := *nxs
	n := len(old)
	NexusEntry := old[n-1]
	old[n-1] = nil        // avoid memory leak
	NexusEntry.Index = -1 // for safety
	*nxs = old[0 : n-1]
	return NexusEntry
}

// Less returns true if the deadline of the first item in the queue is before the one of the second item
func (nxs nexusHeap) Less(i, j int) bool {
	return nxs[i].Deadline.Before(nxs[j].Deadline)
}

// NexusQueue is the queue that holds the NexusEntries and allows to control them in a thread-safe manner
type NexusQueue struct {
	// impl is the underlying implementation of the queue
	impl *nexusHeap

	// mu is the mutex that protects the queue from concurrent access
	mu *sync.RWMutex
}

// Initialize initializes a new NexusQueue
func Initialize() *NexusQueue {
	mh := &nexusHeap{}
	heap.Init(mh)

	mutex := &sync.RWMutex{}

	return &NexusQueue{impl: mh, mu: mutex}
}

// RemoveAll removes all items from the queue without returning them and ignoring the lock
func (p *NexusQueue) removeAllNotBlocking(nexusItems []*structs.NexusItem) {
	for _, item := range nexusItems {
		heap.Remove(p.impl, item.Index)
	}
}

// getAllItemsUntilDeadlineNotBlocking returns all items from the queue until the deadline without returning them and ignoring the lock
func (p *NexusQueue) getAllItemsUntilDeadlineNotBlocking(deadline time.Time) []*structs.NexusItem {
	var items []*structs.NexusItem

	for _, item := range *p.impl {
		if item.Deadline.Before(deadline) {
			items = append(items, item)
		} else {
			break
		}
	}

	return items
}
