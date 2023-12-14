// This example demonstrates a priority queue built using the heap interface.
package main

import (
	"container/heap"
	"fmt"
	"github.com/brianvoe/gofakeit/v6"
	"sync"
	"time"
)

// An NexusEntry is something we manage in a priority queue.
type NexusEntry struct {
	value    string // The value of the NexusEntry; arbitrary.
	Deadline int    // The priority of the NexusEntry in the queue.
	// The index is needed by update and is maintained by the heap.Interface methods.
	index int // The index of the NexusEntry in the heap.
}

// A Nexus implements heap.Interface and holds NexusEntrys.
type Nexus []*NexusEntry

func (nxs Nexus) Len() int { return len(nxs) }

func (nxs Nexus) Less(i, j int) bool {
	// We want Pop to give us the highest, not lowest, priority so we use greater than here.
	return nxs[i].Deadline > nxs[j].Deadline
}

func (nxs Nexus) Swap(i, j int) {
	nxs[i], nxs[j] = nxs[j], nxs[i]
	nxs[i].index = i
	nxs[j].index = j
}

func (nxs *Nexus) Push(x any) {
	n := len(*nxs)
	NexusEntry := x.(*NexusEntry)
	NexusEntry.index = n
	*nxs = append(*nxs, NexusEntry)
}

func (nxs *Nexus) Pop() any {
	old := *nxs
	n := len(old)
	NexusEntry := old[n-1]
	old[n-1] = nil        // avoid memory leak
	NexusEntry.index = -1 // for safety
	*nxs = old[0 : n-1]
	return NexusEntry
}

// TODO immutablity is violated here, why do not overwrite the value before?
// update modifies the priority and value of an NexusEntry in the queue.
func (nxs *Nexus) update(NexusEntry *NexusEntry, value string, priority int) {
	NexusEntry.value = value
	NexusEntry.Deadline = priority
	heap.Fix(nxs, NexusEntry.index)
}

// TODO why is this an exported function?
func (nxs *Nexus) Remove(NexusEntry *NexusEntry) {
	heap.Remove(nxs, NexusEntry.index)
}

func (nxs *Nexus) find(f func(NexusEntry) bool) *NexusEntry {
	for _, NexusEntry := range *nxs {
		if f(*NexusEntry) {
			nxs.Remove(NexusEntry)
			return NexusEntry
		}
	}

	return nil
}

func (nxs *Nexus) search(f func(NexusEntry) bool) *NexusEntry {
	for _, NexusEntry := range *nxs {
		if f(*NexusEntry) {
			return NexusEntry
		}
	}

	return nil
}

func (nxs *Nexus) filter(f func(NexusEntry) bool) []*NexusEntry {
	var NexusEntrys []*NexusEntry

	for _, NexusEntry := range *nxs {
		if f(*NexusEntry) {
			NexusEntrys = append(NexusEntrys, NexusEntry)
		}
	}

	return NexusEntrys
}

func (nxs *Nexus) reduce(f func(accumulator any, entry NexusEntry) any, accumulator any) any {
	for _, entry := range *nxs {
		accumulator = f(accumulator, *entry)
	}

	return accumulator
}

// This example creates a Nexus with some NexusEntrys, adds and manipulates an NexusEntry,
// and then removes the NexusEntrys in priority order.
func main() {
	// Some NexusEntrys and their priorities.
	NexusEntrys := map[string]int{
		"banana": 3, "apple": 2, "pear": 4, "orange": 3,
	}

	// Create a priority queue, put the NexusEntrys in it, and
	// establish the priority queue (heap) invariants.
	nxs := make(Nexus, len(NexusEntrys))
	i := 0
	for value, priority := range NexusEntrys {
		nxs[i] = &NexusEntry{
			value:    value,
			Deadline: priority,
			index:    i,
		}
		i++
	}
	heap.Init(&nxs)

	// Insert a new NexusEntry and then modify its priority.
	item := &NexusEntry{
		value:    "orange",
		Deadline: 1,
	}
	heap.Push(&nxs, item)
	nxs.update(item, item.value, 5)

	// find all NexusEntrys with value orange using reduce
	r := nxs.reduce(func(accumulator any, entry NexusEntry) any {
		if entry.value == "orange" {
			accumulator = append(accumulator.([]*NexusEntry), &entry)
		}

		return accumulator
	}, []*NexusEntry{})

	println(len(r.([]*NexusEntry)))

	for _, entry := range r.([]*NexusEntry) {
		fmt.Printf("%.2d:%s ", entry.Deadline, entry.value)
		fmt.Println()
	}

	// Take the NexusEntrys out; they arrive in decreasing priority order.
	for nxs.Len() > 0 {
		item := heap.Pop(&nxs).(*NexusEntry)
		fmt.Printf("%.2d:%s ", item.Deadline, item.value)
	}

	mainDavid()

}

var nxsMutex sync.Mutex

func mainDavid() {
	gofakeit.Seed(100)

	nxs := make(Nexus, 0)
	heap.Init(&nxs)

	var wg sync.WaitGroup
	wg.Add(2)

	startTime := time.Now()

	go addItems(&nxs, &wg)
	time.Sleep(time.Millisecond * 300)
	go scheduledRemoveItems(&nxs, &wg)

	wg.Wait()

	endTime := time.Now()
	elapsedTime := endTime.Sub(startTime)
	fmt.Printf("Elapsed Time: %v\n", elapsedTime)
}

func addItems(nxs *Nexus, wg *sync.WaitGroup) {
	defer wg.Done()

	for i := 0; i < 1000; i++ {
		value := gofakeit.Word()
		priority := gofakeit.IntRange(1, 10)

		nxsMutex.Lock()
		heap.Push(nxs, &NexusEntry{
			value:    value,
			Deadline: priority,
		})
		nxsMutex.Unlock()
	}
}

var stopChannel = make(chan struct{})

func scheduledRemoveItems(nxs *Nexus, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(1 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !removeItems(nxs) {
				// Stop the goroutine when there are no more items.
				return
			}
		case <-stopChannel:
			return
		}
	}
}

// Modified removeItems function to return a boolean indicating if there are more items.
func removeItems(nxs *Nexus) bool {
	nxsMutex.Lock()
	defer nxsMutex.Unlock()

	if nxs.Len() > 0 {
		item := heap.Pop(nxs).(*NexusEntry)
		fmt.Printf("Removed: %.2d:%s\n", item.Deadline, item.value)
		return true
	}

	return false
}
