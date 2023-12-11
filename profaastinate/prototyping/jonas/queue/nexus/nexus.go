// acting as a nexus for all scheduling algorithms
// core princeple: all tasks are stored in a central place
// share memory by communicating
package nexus

import (
	"github.com/Persists/profaastinate-queue/task"
)

// central storing place for all future tasks
type NexusRegistry struct {
	Data []NexusEntry

	// every time a task is executed from a scheduling algorithm
	ExecutedEntries chan []*NexusEntry

	// everytime a task is removed from the nexus
	RemovedEntries map[string]chan []*NexusEntry

	// everytime a task is added to the nexus
	NewEntries map[string]chan []*NexusEntry
}

// entry in the galactic timeline that contains a task
// and some metadata
type NexusEntry struct {
	Index int

	// task
	Task *task.Task
}

func NewNexusRegistry() *NexusRegistry {
	return &NexusRegistry{
		Data:            make([]NexusEntry, 0),
		ExecutedEntries: make(chan []*NexusEntry),
		RemovedEntries:  make(map[string]chan []*NexusEntry),
		NewEntries:      make(map[string]chan []*NexusEntry),
	}
}

func (n *NexusRegistry) RegisterNewScheduler(name string) {
	n.ExecutedEntries[name] = make(chan []*NexusEntry)
}

// insert a task into the nexus
// the task will be inserted at the end of the nexus
// so that the other indexes are not affected
func (n *NexusRegistry) Insert(task *task.Task) {
	n.Data = append(n.Data, NexusEntry{
		Index: len(n.Data),
		Task:  task,
	})
}

// remove a task from the nexus
func (n *NexusRegistry) Remove(indexes []int) {

	toDelete := 0

	var removedentries []int

	for _, index := range indexes {
		if index >= len(n.Data) {

			continue
		}

		removedentries = append(removedentries, index)

		n.Data[index] = n.Data[len(n.Data)-1-index]
		toDelete++
	}

	n.Data = n.Data[:len(n.Data)-toDelete]

	// pass update into channel
}

// get all entries as pointers
func (n *NexusRegistry) GetEntries() []*NexusEntry {
	entries := make([]*NexusEntry, len(n.Data))

	for i, entry := range n.Data {
		entries[i] = &entry
	}

	return entries
}

func (n *NexusRegistry) UpdateOnRemove(removedEntries []*NexusEntry) {
	n.RemovedEntries <- removedEntries

}
