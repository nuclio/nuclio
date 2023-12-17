package common

import (
	"container/heap"
	common "nexus/common/models/structs"
	"time"
)

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

// PopBulkUntilDeadline pops all items from the queue until the deadline
func (p *NexusQueue) PopBulkUntilDeadline(deadline time.Time) []*common.NexusItem {
	p.mu.Lock()
	defer p.mu.Unlock()

	items := p.getAllItemsUntilDeadlineNotBlocking(deadline)

	p.removeAllNotBlocking(items)

	return items
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

	p.removeAllNotBlocking(nexusItems)
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

func (p *NexusQueue) GetAllItemsUntilDeadline(deadline time.Time) []*common.NexusItem {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.getAllItemsUntilDeadlineNotBlocking(deadline)
}
