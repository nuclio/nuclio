package structs

import (
	"time"
)

type NexusItem struct {
	Value    interface{} // The value of the NexusEntry; arbitrary.
	Index    int         // The index of the NexusEntry in the heap.
	Deadline time.Time   // The priority of the NexusEntry in the queue.
	Name     string      // The name of the NexusEntry
}
