package common

import (
	"container/heap"
	common "nexus/common/models/structs"
	"sync"
	"time"
)

type NexusQueue struct {
	impl *nexusHeap

	mu *sync.RWMutex
}

func Init() *NexusQueue {
	mh := &nexusHeap{}
	heap.Init(mh)

	mutex := &sync.RWMutex{}

	return &NexusQueue{impl: mh, mu: mutex}
}

func (p NexusQueue) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.impl.Len()
}

func (p *NexusQueue) Push(el *common.NexusItem) {
	p.mu.Lock()
	defer p.mu.Unlock()

	heap.Push(p.impl, el)
}

func (p *NexusQueue) Update(el *common.NexusItem, deadline time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()

	el.Deadline = deadline
	heap.Fix(p.impl, el.Index)
}

func (p *NexusQueue) Pop() *common.NexusItem {
	p.mu.RLock()
	defer p.mu.RUnlock()

	el := heap.Pop(p.impl)
	return el.(*common.NexusItem)
}

func (p *NexusQueue) Peek() *common.NexusItem {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return (*p.impl)[0]
}

func (p *NexusQueue) Remove(item *common.NexusItem) {
	p.mu.Lock()
	defer p.mu.Unlock()

	heap.Remove(p.impl, item.Index)
}

// RemoveAll removes all given items from the queue
func (p *NexusQueue) RemoveAll(nexusItems []*common.NexusItem) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, item := range nexusItems {
		heap.Remove(p.impl, item.Index)
	}
}

func (p *NexusQueue) GetMostCommonEntryItems() []*common.NexusItem {
	p.mu.RLock()
	defer p.mu.RUnlock()

	counts := make(map[string][]*common.NexusItem)

	for _, item := range *p.impl {
		counts[item.Name] = append(counts[item.Name], item)
	}

	maxCount := 0
	var maxEntryItems []*common.NexusItem

	for _, items := range counts {
		if numberOfEntries := len(items); numberOfEntries > maxCount {
			maxCount = numberOfEntries
			maxEntryItems = items
		}
	}
	counts = nil // free memory
	return maxEntryItems
}

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
