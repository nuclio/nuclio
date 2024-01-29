package common

import (
	"container/heap"
	"time"

	"github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
)

// Len returns the length of the queue
func (p *NexusQueue) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.impl.Len()
}

// Push pushes an item to the queue
func (p *NexusQueue) Push(el *structs.NexusItem) {
	p.mu.Lock()
	defer p.mu.Unlock()

	heap.Push(p.impl, el)
}

// Update updates an item in the queue
func (p *NexusQueue) Update(el *structs.NexusItem, deadline time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()

	el.Deadline = deadline
	heap.Fix(p.impl, el.Index)
}

// Pop removes and returns the first item from the queue
func (p *NexusQueue) Pop() *structs.NexusItem {
	p.mu.RLock()
	defer p.mu.RUnlock()

	el := heap.Pop(p.impl)
	return el.(*structs.NexusItem)
}

// PopBulkUntilDeadline pops all items from the queue until the deadline
func (p *NexusQueue) PopBulkUntilDeadline(deadline time.Time) []*structs.NexusItem {
	p.mu.Lock()
	defer p.mu.Unlock()

	items := p.getAllItemsUntilDeadlineNotBlocking(deadline)

	p.removeAllNotBlocking(items)

	return items
}

// Peek returns the first item from the queue without removing it
func (p *NexusQueue) Peek() *structs.NexusItem {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return (*p.impl)[0]
}

// Remove removes an item from the queue without returning it
func (p *NexusQueue) Remove(item *structs.NexusItem) {
	p.mu.Lock()
	defer p.mu.Unlock()

	heap.Remove(p.impl, item.Index)
}

// RemoveAll removes all given items from the queue
func (p *NexusQueue) RemoveAll(nexusItems []*structs.NexusItem) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.removeAllNotBlocking(nexusItems)
}

// GetMostCommonEntryItems checks which entry has the most items by name in the queue and returns them
func (p *NexusQueue) GetMostCommonEntryItems() []*structs.NexusItem {
	p.mu.RLock()
	defer p.mu.RUnlock()

	counts := make(map[string][]*structs.NexusItem)

	for _, item := range *p.impl {
		counts[item.Name] = append(counts[item.Name], item)
	}

	maxCount := 0
	var maxEntryItems []*structs.NexusItem

	for _, items := range counts {
		if numberOfEntries := len(items); numberOfEntries > maxCount {
			maxCount = numberOfEntries
			maxEntryItems = items
		}
	}
	return maxEntryItems
}
