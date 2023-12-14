package common

import (
	"container/heap"
	common "nexus/common/models/structs"
	"time"
)

type NexusQueue struct {
	impl *deadlineHeap
}

func Init() *NexusQueue {
	mh := &deadlineHeap{}
	heap.Init(mh)
	return &NexusQueue{impl: mh}
}

func (p NexusQueue) Len() int { return p.impl.Len() }

func (p *NexusQueue) Push(el *common.NexusItem) {
	heap.Push(p.impl, el)
}

func (p *NexusQueue) Update(el *common.NexusItem, deadline time.Time) {
	el.Deadline = deadline
	heap.Fix(p.impl, el.Index)
}

func (p *NexusQueue) Pop() *common.NexusItem {
	el := heap.Pop(p.impl)
	return el.(*common.NexusItem)
}

func (p *NexusQueue) Peek() *common.NexusItem {
	return (*p.impl)[0]
}

func (p *NexusQueue) Remove(index int) {
	heap.Remove(p.impl, index)
}

func (p *NexusQueue) RemoveAll(nexusIndices []int) {
	for nexusIndex, _ := range nexusIndices {
		p.Remove(nexusIndex)
	}
}

func (p *NexusQueue) GetMostCommonEntryIndices() []int {
	counts := make(map[string][]int)

	for _, item := range *p.impl {
		counts[item.Name] = append(counts[item.Name], item.Index)
	}

	maxCount := 0
	var maxEntryIndices []int

	for _, itemIndices := range counts {
		if numberOfEntries := len(itemIndices); numberOfEntries > maxCount {
			maxCount = numberOfEntries
			maxEntryIndices = itemIndices
		}
	}

	counts = nil // free memory
	return maxEntryIndices
}

type deadlineHeap []*common.NexusItem

func (nxs deadlineHeap) Len() int { return len(nxs) }

func (nxs deadlineHeap) Swap(i, j int) {
	nxs[i], nxs[j] = nxs[j], nxs[i]
	nxs[i].Index = i
	nxs[j].Index = j
}

func (nxs *deadlineHeap) Push(x any) {
	n := len(*nxs)
	NexusEntry := x.(*common.NexusItem)
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
